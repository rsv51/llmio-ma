package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"gorm.io/gorm"
)

// HealthCheckService 健康检查服务
type HealthCheckService struct {
	db       *gorm.DB
	stopChan chan struct{}
	running  bool
}

// NewHealthCheckService 创建健康检查服务实例
func NewHealthCheckService(db *gorm.DB) *HealthCheckService {
	return &HealthCheckService{
		db:       db,
		stopChan: make(chan struct{}),
		running:  false,
	}
}

// Start 启动健康检查服务
func (s *HealthCheckService) Start() error {
	if s.running {
		return fmt.Errorf("health check service is already running")
	}

	s.running = true
	go s.run()
	slog.Info("Health check service started")
	return nil
}

// Stop 停止健康检查服务
func (s *HealthCheckService) Stop() {
	if !s.running {
		return
	}

	close(s.stopChan)
	s.running = false
	slog.Info("Health check service stopped")
}

// run 运行健康检查循环
func (s *HealthCheckService) run() {
	// 立即执行一次检查
	s.checkAllProviders()

	ticker := time.NewTicker(s.getCheckInterval())
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			// 重新获取配置以支持动态调整
			ticker.Reset(s.getCheckInterval())
			s.checkAllProviders()
		}
	}
}

// getCheckInterval 获取检查间隔
func (s *HealthCheckService) getCheckInterval() time.Duration {
	var config models.HealthCheckConfig
	if err := s.db.First(&config).Error; err != nil {
		slog.Warn("Failed to get health check config, using default 5 minutes", "error", err)
		return 5 * time.Minute
	}

	if !config.Enabled {
		// 如果禁用，返回一个较长的间隔以减少资源消耗
		return 1 * time.Hour
	}

	return time.Duration(config.IntervalMinutes) * time.Minute
}

// checkAllProviders 检查所有提供商
func (s *HealthCheckService) checkAllProviders() {
	ctx := context.Background()

	// 获取健康检查配置
	var config models.HealthCheckConfig
	if err := s.db.First(&config).Error; err != nil {
		slog.Error("Failed to get health check config", "error", err)
		return
	}

	if !config.Enabled {
		return
	}

	// 获取所有提供商
	var providers []models.Provider
	if err := s.db.Find(&providers).Error; err != nil {
		slog.Error("Failed to get providers for health check", "error", err)
		return
	}

	slog.Info("Starting health check", "provider_count", len(providers))

	for _, provider := range providers {
		s.checkProvider(ctx, &provider, &config)
	}

	slog.Info("Health check completed")
}

// checkProvider 检查单个提供商
func (s *HealthCheckService) checkProvider(ctx context.Context, provider *models.Provider, config *models.HealthCheckConfig) {
	// 获取或创建验证记录
	var validation models.ProviderValidation
	err := s.db.Where("provider_id = ?", provider.ID).First(&validation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新记录
			validation = models.ProviderValidation{
				ProviderID:  provider.ID,
				IsHealthy:   true,
				ErrorCount:  0,
				LastValidatedAt: time.Now(),
			}
			if err := s.db.Create(&validation).Error; err != nil {
				slog.Error("Failed to create validation record", "provider", provider.Name, "error", err)
				return
			}
		} else {
			slog.Error("Failed to get validation record", "provider", provider.Name, "error", err)
			return
		}
	}

	// 如果提供商不健康且未到重试时间，跳过检查
	if !validation.IsHealthy && validation.NextRetryAt != nil && time.Now().Before(*validation.NextRetryAt) {
		slog.Debug("Provider not ready for retry", "provider", provider.Name, "next_retry", validation.NextRetryAt)
		return
	}

	// 执行健康检查
	slog.Debug("Checking provider health", "provider", provider.Name, "type", provider.Type)
	
	isHealthy, statusCode, errMsg := s.performHealthCheck(ctx, provider)
	
	now := time.Now()
	validation.LastValidatedAt = now
	validation.LastStatusCode = statusCode

	if isHealthy {
		// 成功
		validation.ConsecutiveSuccesses++
		validation.LastSuccessAt = &now
		
		// 如果之前不健康，现在恢复了
		if !validation.IsHealthy {
			slog.Info("Provider recovered", "provider", provider.Name, "previous_errors", validation.ErrorCount)
			validation.IsHealthy = true
			validation.ErrorCount = 0
			validation.LastError = ""
			validation.NextRetryAt = nil
		}
	} else {
		// 失败
		validation.ErrorCount++
		validation.LastError = errMsg
		validation.ConsecutiveSuccesses = 0
		
		slog.Warn("Provider health check failed", 
			"provider", provider.Name, 
			"error_count", validation.ErrorCount,
			"status_code", statusCode,
			"error", errMsg)

		// 如果错误次数超过阈值，标记为不健康
		if validation.ErrorCount >= config.MaxErrorCount {
			if validation.IsHealthy {
				slog.Error("Provider marked as unhealthy", "provider", provider.Name, "error_count", validation.ErrorCount)
			}
			validation.IsHealthy = false
			
			// 设置下次重试时间
			nextRetry := now.Add(time.Duration(config.RetryAfterHours) * time.Hour)
			validation.NextRetryAt = &nextRetry
		}
	}

	// 保存验证结果
	if err := s.db.Save(&validation).Error; err != nil {
		slog.Error("Failed to save validation record", "provider", provider.Name, "error", err)
	}
}

// performHealthCheck 执行实际的健康检查
func (s *HealthCheckService) performHealthCheck(ctx context.Context, provider *models.Provider) (bool, int, string) {
	// 创建提供商实例
	chatModel, err := providers.New(provider.Type, provider.Config)
	if err != nil {
		return false, 0, fmt.Sprintf("failed to create provider: %v", err)
	}

	// 构造一个简单的测试请求
	testRequest := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"max_tokens": 5,
	}

	requestBody, err := json.Marshal(testRequest)
	if err != nil {
		return false, 0, fmt.Sprintf("failed to marshal request: %v", err)
	}

	// 创建HTTP客户端，设置较短的超时时间
	client := providers.GetClient(10 * time.Second)
	
	// 执行请求
	resp, err := chatModel.Chat(ctx, client, "test-model", requestBody)
	if err != nil {
		return false, 0, fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	// 200-299 视为成功
	// 401, 403 视为配置错误但提供商本身可用
	// 404 视为模型不存在但提供商可用
	// 429 视为速率限制但提供商可用
	// 其他视为失败
	statusCode := resp.StatusCode
	
	switch {
	case statusCode >= 200 && statusCode < 300:
		return true, statusCode, ""
	case statusCode == 401 || statusCode == 403:
		return true, statusCode, "authentication error (provider is reachable)"
	case statusCode == 404:
		return true, statusCode, "model not found (provider is reachable)"
	case statusCode == 429:
		return true, statusCode, "rate limited (provider is reachable)"
	case statusCode >= 500:
		return false, statusCode, fmt.Sprintf("server error: %d", statusCode)
	default:
		return false, statusCode, fmt.Sprintf("unexpected status: %d", statusCode)
	}
}

// GetProviderHealth 获取提供商健康状态
func GetProviderHealth(ctx context.Context, db *gorm.DB, providerID uint) (*models.ProviderValidation, error) {
	var validation models.ProviderValidation
	err := db.Where("provider_id = ?", providerID).First(&validation).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果没有记录，返回默认健康状态
			return &models.ProviderValidation{
				ProviderID:  providerID,
				IsHealthy:   true,
				ErrorCount:  0,
				LastValidatedAt: time.Now(),
			}, nil
		}
		return nil, err
	}
	return &validation, nil
}

// GetAllProvidersHealth 获取所有提供商健康状态
func GetAllProvidersHealth(ctx context.Context, db *gorm.DB) ([]models.ProviderValidation, error) {
	var validations []models.ProviderValidation
	err := db.Find(&validations).Error
	return validations, err
}

// ForceCheckProvider 强制检查指定提供商
func ForceCheckProvider(ctx context.Context, db *gorm.DB, providerID uint) error {
	var provider models.Provider
	if err := db.First(&provider, providerID).Error; err != nil {
		return fmt.Errorf("provider not found: %w", err)
	}

	var config models.HealthCheckConfig
	if err := db.First(&config).Error; err != nil {
		return fmt.Errorf("failed to get health check config: %w", err)
	}

	service := NewHealthCheckService(db)
	service.checkProvider(ctx, &provider, &config)
	
	return nil
}