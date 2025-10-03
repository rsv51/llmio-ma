package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/atopos31/llmio/balancer"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 全局配置缓存实例，默认TTL为5分钟
var configCache = NewConfigCache(5 * time.Minute)

func BalanceChat(c *gin.Context, style string, Beforer Beforer, processer Processer) error {
	return BalanceChatWithExclusions(c, style, Beforer, processer, nil)
}

// BalanceChatWithExclusions 支持排除特定提供商的负载均衡
func BalanceChatWithExclusions(c *gin.Context, style string, Beforer Beforer, processer Processer, excludedProviderIDs []uint) error {
	proxyStart := time.Now()
	rawData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	ctx := c.Request.Context()
	before, err := Beforer(rawData)
	if err != nil {
		return err
	}

	llmProvidersWithLimit, err := ProvidersBymodelsName(ctx, before.model)
	if err != nil {
		return err
	}
	// 所有模型提供商关联
	llmproviders := llmProvidersWithLimit.Providers

	slog.Info("request", "model", before.model, "stream", before.stream, "tool_call", before.toolCall, "structured_output", before.structuredOutput, "image", before.image)

	if len(llmproviders) == 0 {
		return fmt.Errorf("no provider found for models %s", before.model)
	}

	// 预分配切片容量
	providerIds := make([]uint, 0, len(llmproviders))
	for _, modelWithProvider := range llmproviders {
		providerIds = append(providerIds, modelWithProvider.ProviderID)
	}

	// 过滤排除的提供商和不健康的提供商
	healthyProviderIds := make([]uint, 0, len(providerIds))
	for _, id := range providerIds {
		// 检查是否在排除列表中
		if excludedProviderIDs != nil && slices.Contains(excludedProviderIDs, id) {
			continue
		}
		
		// 检查健康状态
		validation, err := GetProviderHealth(ctx, models.DB, id)
		if err == nil && validation.IsHealthy {
			healthyProviderIds = append(healthyProviderIds, id)
		}
	}
	
	// 如果没有健康的提供商，使用原始列表（允许降级）
	queryProviderIds := healthyProviderIds
	if len(queryProviderIds) == 0 {
		slog.Warn("No healthy providers found, falling back to all providers", "model", before.model)
		queryProviderIds = providerIds
	}
	
	provideritems, err := gorm.G[models.Provider](models.DB).Where("id IN ?", queryProviderIds).Where("type = ?", style).Find(ctx)
	if err != nil {
		return err
	}
	if len(provideritems) == 0 {
		return fmt.Errorf("no %s provider found for %s", style, before.model)
	}

	// 构建providerID到provider的映射，避免重复查找
	providerMap := make(map[uint]*models.Provider, len(provideritems))
	for i := range provideritems {
		provider := &provideritems[i]
		providerMap[provider.ID] = provider
	}

	items := make(map[uint]int)
	for _, modelWithProvider := range llmproviders {
		// 过滤是否开启工具调用
		if modelWithProvider.ToolCall != nil && before.toolCall && !*modelWithProvider.ToolCall {
			continue
		}
		// 过滤是否开启结构化输出
		if modelWithProvider.StructuredOutput != nil && before.structuredOutput && !*modelWithProvider.StructuredOutput {
			continue
		}
		// 过滤是否拥有视觉能力
		if modelWithProvider.Image != nil && before.image && !*modelWithProvider.Image {
			continue
		}
		provider := providerMap[modelWithProvider.ProviderID]
		// 过滤提供商类型
		if provider == nil || provider.Type != style {
			continue
		}
		items[modelWithProvider.ID] = modelWithProvider.Weight
	}

	if len(items) == 0 {
		return errors.New("no provider with tool_call or structured_output or image found for models " + before.model)
	}
	// 收集重试过程中的err日志
	retryErrLog := make(chan models.ChatLog, llmProvidersWithLimit.MaxRetry)
	defer close(retryErrLog)
	go func() {
		for log := range retryErrLog {
			_, err := SaveChatLog(context.Background(), log)
			if err != nil {
				slog.Error("save chat log error", "error", err)
			}
		}
	}()

	for retry := 0; retry < llmProvidersWithLimit.MaxRetry; retry++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * time.Duration(llmProvidersWithLimit.TimeOut)):
			return errors.New("retry time out !")
		default:
			// 加权负载均衡
			item, err := balancer.WeightedRandom(items)
			if err != nil {
				return err
			}
			modelWithProviderIndex := slices.IndexFunc(llmproviders, func(mp models.ModelWithProvider) bool {
				return mp.ID == *item
			})
			modelWithProvider := llmproviders[modelWithProviderIndex]

			provider := providerMap[modelWithProvider.ProviderID]

			chatModel, err := providers.New(style, provider.Config)
			if err != nil {
				return err
			}

			slog.Info("using provider", "provider", provider.Name, "model", modelWithProvider.ProviderModel)

			log := models.ChatLog{
				Name:          before.model,
				ProviderModel: modelWithProvider.ProviderModel,
				ProviderName:  provider.Name,
				Status:        "success",
				Style:         style,
				Retry:         retry,
				ProxyTime:     time.Since(proxyStart),
			}
			reqStart := time.Now()
			client := providers.GetClient(time.Second * time.Duration(llmProvidersWithLimit.TimeOut) / 3)
			res, err := chatModel.Chat(ctx, client, modelWithProvider.ProviderModel, before.raw)
			if err != nil {
				retryErrLog <- log.WithError(err)
				// 请求失败 移除待选
				delete(items, *item)
				
				// 更新健康检查状态
				go updateProviderHealthOnError(context.Background(), provider.ID, err.Error(), 0)
				continue
			}
			// 注意：连接池中的client会在使用后自动管理，这里使用的是缓存的client，不需要手动归还

			if res.StatusCode != http.StatusOK {
				byteBody, err := io.ReadAll(res.Body)
				if err != nil {
					slog.Error("read body error", "error", err)
				}
				errorMsg := fmt.Sprintf("status: %d, body: %s", res.StatusCode, string(byteBody))
				retryErrLog <- log.WithError(fmt.Errorf(errorMsg))

				// 更新健康检查状态
				go updateProviderHealthOnError(context.Background(), provider.ID, errorMsg, res.StatusCode)

				if res.StatusCode == http.StatusTooManyRequests {
					// 达到RPM限制 降低权重
					items[*item] -= items[*item] / 3
				} else {
					// 非RPM限制 移除待选
					delete(items, *item)
				}
				res.Body.Close()
				continue
			}
			defer res.Body.Close()

			// 成功请求，更新健康状态和使用统计
			go updateProviderHealthOnSuccess(context.Background(), provider.ID)

			logId, err := SaveChatLog(ctx, log)
			if err != nil {
				return err
			}
			
			// 更新使用统计
			go UpdateProviderUsageStats(context.Background(), models.DB, provider.ID, log)

			pr, pw := io.Pipe()
			tee := io.TeeReader(res.Body, pw)

			// 与客户端并行处理响应数据流 同时记录日志
			go func(ctx context.Context) {
				defer pr.Close()
				processer(ctx, pr, before.stream, logId, reqStart)
			}(context.Background())
			// 转发给客户端
			if before.stream {
				c.Header("Content-Type", "text/event-stream")
				c.Header("Cache-Control", "no-cache")
			} else {
				c.Header("Content-Type", "application/json")
			}
			c.Writer.Flush()
			if _, err := io.Copy(c.Writer, tee); err != nil {
				pw.CloseWithError(err)
				return err
			}

			pw.Close()

			return nil
		}
	}

	return errors.New("maximum retry attempts reached !")
}

func SaveChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
	if err := gorm.G[models.ChatLog](models.DB).Create(ctx, &log); err != nil {
		return 0, err
	}
	
	// updateProviderHealthOnError 在请求失败时更新健康状态
	func updateProviderHealthOnError(ctx context.Context, providerID uint, errorMsg string, statusCode int) {
		var validation models.ProviderValidation
		err := models.DB.Where("provider_id = ?", providerID).First(&validation).Error
		
		if err == gorm.ErrRecordNotFound {
			validation = models.ProviderValidation{
				ProviderID:      providerID,
				IsHealthy:       true,
				ErrorCount:      1,
				LastError:       errorMsg,
				LastStatusCode:  statusCode,
				LastValidatedAt: time.Now(),
			}
			
			if err := models.DB.Create(&validation).Error; err != nil {
				slog.Error("Failed to create validation record", "provider_id", providerID, "error", err)
			}
			return
		} else if err != nil {
			slog.Error("Failed to get validation record", "provider_id", providerID, "error", err)
			return
		}
		
		// 更新错误信息
		validation.ErrorCount++
		validation.LastError = errorMsg
		validation.LastStatusCode = statusCode
		validation.LastValidatedAt = time.Now()
		validation.ConsecutiveSuccesses = 0
		
		// 获取健康检查配置
		var config models.HealthCheckConfig
		if err := models.DB.First(&config).Error; err == nil {
			// 如果错误次数超过阈值，标记为不健康
			if validation.ErrorCount >= config.MaxErrorCount && validation.IsHealthy {
				slog.Warn("Provider marked as unhealthy due to errors",
					"provider_id", providerID,
					"error_count", validation.ErrorCount)
				validation.IsHealthy = false
				
				// 设置下次重试时间
				nextRetry := time.Now().Add(time.Duration(config.RetryAfterHours) * time.Hour)
				validation.NextRetryAt = &nextRetry
			}
		}
		
		if err := models.DB.Save(&validation).Error; err != nil {
			slog.Error("Failed to save validation record", "provider_id", providerID, "error", err)
		}
	}
	
	// updateProviderHealthOnSuccess 在请求成功时更新健康状态
	func updateProviderHealthOnSuccess(ctx context.Context, providerID uint) {
		var validation models.ProviderValidation
		err := models.DB.Where("provider_id = ?", providerID).First(&validation).Error
		
		now := time.Now()
		
		if err == gorm.ErrRecordNotFound {
			validation = models.ProviderValidation{
				ProviderID:           providerID,
				IsHealthy:            true,
				ErrorCount:           0,
				LastValidatedAt:      now,
				LastSuccessAt:        &now,
				ConsecutiveSuccesses: 1,
			}
			
			if err := models.DB.Create(&validation).Error; err != nil {
				slog.Error("Failed to create validation record", "provider_id", providerID, "error", err)
			}
			return
		} else if err != nil {
			slog.Error("Failed to get validation record", "provider_id", providerID, "error", err)
			return
		}
		
		// 更新成功信息
		wasUnhealthy := !validation.IsHealthy
		validation.ConsecutiveSuccesses++
		validation.LastSuccessAt = &now
		validation.LastValidatedAt = now
		
		// 如果之前不健康，现在恢复了
		if wasUnhealthy {
			slog.Info("Provider recovered from unhealthy state",
				"provider_id", providerID,
				"previous_errors", validation.ErrorCount)
			validation.IsHealthy = true
			validation.ErrorCount = 0
			validation.LastError = ""
			validation.NextRetryAt = nil
		}
		
		if err := models.DB.Save(&validation).Error; err != nil {
			slog.Error("Failed to save validation record", "provider_id", providerID, "error", err)
		}
	}
	return log.ID, nil
}

type ProvidersWithlimit struct {
	Providers []models.ModelWithProvider
	MaxRetry  int
	TimeOut   int
}

// ProvidersBymodelsName 获取模型对应的提供商列表，支持缓存
func ProvidersBymodelsName(ctx context.Context, modelsName string) (*ProvidersWithlimit, error) {
	return configCache.ProvidersBymodelsNameWithCache(ctx, modelsName)
}

// ProvidersBymodelsNameDirect 直接查询数据库的版本（用于测试和特殊情况）
func ProvidersBymodelsNameDirect(ctx context.Context, modelsName string) (*ProvidersWithlimit, error) {
	llmmodels, err := gorm.G[models.Model](models.DB).Where("name = ?", modelsName).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found model " + modelsName)
		}
		return nil, err
	}

	llmproviders, err := gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", llmmodels.ID).Find(ctx)
	if err != nil {
		return nil, err
	}

	if len(llmproviders) == 0 {
		return nil, errors.New("not provider for model " + modelsName)
	}
	return &ProvidersWithlimit{
		Providers: llmproviders,
		MaxRetry:  llmmodels.MaxRetry,
		TimeOut:   llmmodels.TimeOut,
	}, nil
}
