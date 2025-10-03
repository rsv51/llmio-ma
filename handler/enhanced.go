package handler

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProviderHealthStatus 提供商健康状态（增强版）
type ProviderHealthStatus struct {
	ProviderID           uint       `json:"provider_id"`
	ProviderName         string     `json:"provider_name"`
	ProviderType         string     `json:"provider_type"`
	Status               string     `json:"status"` // healthy, degraded, unhealthy, unknown
	IsHealthy            bool       `json:"is_healthy"`
	ResponseTime         int64      `json:"response_time_ms"`
	LastChecked          time.Time  `json:"last_checked"`
	LastSuccess          *time.Time `json:"last_success,omitempty"`
	ErrorMessage         string     `json:"error_message,omitempty"`
	ErrorCount           int        `json:"error_count"`
	ConsecutiveSuccesses int        `json:"consecutive_successes"`
	NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
	LastStatusCode       int        `json:"last_status_code,omitempty"`
	SuccessRate24h       float64    `json:"success_rate_24h"`
	TotalRequests24h     int64      `json:"total_requests_24h"`
	AvgResponseTime      float64    `json:"avg_response_time_ms"`
}

// DashboardStats 仪表板统计数据
type DashboardStats struct {
	TotalProviders     int     `json:"total_providers"`
	HealthyProviders   int     `json:"healthy_providers"`
	TotalModels        int     `json:"total_models"`
	TotalRequests24h   int64   `json:"total_requests_24h"`
	SuccessRequests24h int64   `json:"success_requests_24h"`
	FailedRequests24h  int64   `json:"failed_requests_24h"`
	AvgResponseTime    float64 `json:"avg_response_time_ms"`
	TotalTokens24h     int64   `json:"total_tokens_24h"`
	TopModels          []ModelUsageStats `json:"top_models"`
	TopProviders       []ProviderUsageStats `json:"top_providers"`
}

// ModelUsageStats 模型使用统计
type ModelUsageStats struct {
	ModelName     string  `json:"model_name"`
	RequestCount  int64   `json:"request_count"`
	SuccessRate   float64 `json:"success_rate"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
}

// ProviderUsageStats 提供商使用统计
type ProviderUsageStats struct {
	ProviderName  string  `json:"provider_name"`
	RequestCount  int64   `json:"request_count"`
	SuccessRate   float64 `json:"success_rate"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// ProviderValidationResult 提供商验证结果
type ProviderValidationResult struct {
	Valid        bool     `json:"valid"`
	ErrorMessage string   `json:"error_message,omitempty"`
	Models       []string `json:"models,omitempty"`
	ResponseTime int64    `json:"response_time_ms"`
}

// GetProviderHealth 获取提供商健康状态
func GetProviderHealth(c *gin.Context) {
	providerIDStr := c.Param("id")
	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid provider ID format")
		return
	}

	// 获取提供商信息
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", providerID).First(c.Request.Context())
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Provider not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	// 执行健康检查
	healthStatus := checkProviderHealth(c.Request.Context(), &provider)
	common.Success(c, healthStatus)
}

// GetAllProvidersHealth 获取所有提供商健康状态
func GetAllProvidersHealth(c *gin.Context) {
	providers, err := gorm.G[models.Provider](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve providers: "+err.Error())
		return
	}

	healthStatuses := make([]ProviderHealthStatus, 0, len(providers))
	for _, provider := range providers {
		healthStatus := checkProviderHealth(c.Request.Context(), &provider)
		healthStatuses = append(healthStatuses, healthStatus)
	}

	common.Success(c, healthStatuses)
}

// checkProviderHealth 检查提供商健康状态（增强版，使用ProviderValidation表）
func checkProviderHealth(ctx context.Context, provider *models.Provider) ProviderHealthStatus {
	status := ProviderHealthStatus{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		ProviderType: provider.Type,
		LastChecked:  time.Now(),
	}

	// 从ProviderValidation表获取验证状态
	validation, err := service.GetProviderHealth(ctx, models.DB, provider.ID)
	if err != nil {
		slog.Warn("Failed to get provider validation", "provider", provider.Name, "error", err)
	} else {
		status.IsHealthy = validation.IsHealthy
		status.ErrorCount = validation.ErrorCount
		status.LastSuccess = validation.LastSuccessAt
		status.ConsecutiveSuccesses = validation.ConsecutiveSuccesses
		status.NextRetryAt = validation.NextRetryAt
		status.LastStatusCode = validation.LastStatusCode
		status.ErrorMessage = validation.LastError
		status.LastChecked = validation.LastValidatedAt
	}

	// 获取最近24小时的统计数据
	since := time.Now().Add(-24 * time.Hour)
	
	var total, success int64
	var avgResponseTime float64
	
	if err := models.DB.Model(&models.ChatLog{}).
		Where("provider_name = ? AND created_at > ?", provider.Name, since).
		Count(&total).Error; err != nil {
		slog.Error("Failed to count total requests", "error", err)
	}
	
	if err := models.DB.Model(&models.ChatLog{}).
		Where("provider_name = ? AND created_at > ? AND status = ?", provider.Name, since, "success").
		Count(&success).Error; err != nil {
		slog.Error("Failed to count success requests", "error", err)
	}

	if err := models.DB.Model(&models.ChatLog{}).
		Select("AVG(proxy_time) as avg_time").
		Where("provider_name = ? AND created_at > ? AND status = ?", provider.Name, since, "success").
		Row().Scan(&avgResponseTime); err != nil {
		slog.Error("Failed to get avg response time", "error", err)
	}

	status.TotalRequests24h = total
	status.AvgResponseTime = avgResponseTime / float64(time.Millisecond)
	
	if total > 0 {
		status.SuccessRate24h = float64(success) / float64(total) * 100
	}

	// 确定整体状态
	if !status.IsHealthy {
		status.Status = "unhealthy"
		if status.ErrorMessage == "" {
			status.ErrorMessage = "Provider marked as unhealthy"
		}
	} else if status.SuccessRate24h < 50 && total > 10 {
		status.Status = "degraded"
		if status.ErrorMessage == "" {
			status.ErrorMessage = "Low success rate in last 24h"
		}
	} else if status.ErrorCount > 0 {
		status.Status = "degraded"
	} else {
		status.Status = "healthy"
	}

	return status
}

// GetDashboardStats 获取仪表板统计数据
func GetDashboardStats(c *gin.Context) {
	stats := DashboardStats{}
	ctx := c.Request.Context()
	since := time.Now().Add(-24 * time.Hour)

	// 获取提供商总数
	var totalProviders int64
	if err := models.DB.Model(&models.Provider{}).Count(&totalProviders).Error; err != nil {
		common.InternalServerError(c, "Failed to count providers: "+err.Error())
		return
	}
	stats.TotalProviders = int(totalProviders)

	// 获取模型总数
	var totalModels int64
	if err := models.DB.Model(&models.Model{}).Count(&totalModels).Error; err != nil {
		common.InternalServerError(c, "Failed to count models: "+err.Error())
		return
	}
	stats.TotalModels = int(totalModels)

	// 获取24小时内的请求统计
	if err := models.DB.Model(&models.ChatLog{}).
		Where("created_at > ?", since).
		Count(&stats.TotalRequests24h).Error; err != nil {
		common.InternalServerError(c, "Failed to count total requests: "+err.Error())
		return
	}

	if err := models.DB.Model(&models.ChatLog{}).
		Where("created_at > ? AND status = ?", since, "success").
		Count(&stats.SuccessRequests24h).Error; err != nil {
		common.InternalServerError(c, "Failed to count success requests: "+err.Error())
		return
	}

	stats.FailedRequests24h = stats.TotalRequests24h - stats.SuccessRequests24h

	// 获取平均响应时间
	if err := models.DB.Model(&models.ChatLog{}).
		Select("AVG(proxy_time) as avg_time").
		Where("created_at > ? AND status = ?", since, "success").
		Row().Scan(&stats.AvgResponseTime); err != nil {
		slog.Error("Failed to get avg response time", "error", err)
	}
	stats.AvgResponseTime = stats.AvgResponseTime / float64(time.Millisecond)

	// 获取总token数
	var totalTokens int64
	if err := models.DB.Model(&models.ChatLog{}).
		Select("COALESCE(SUM(total_tokens), 0)").
		Where("created_at > ?", since).
		Row().Scan(&totalTokens); err != nil {
		slog.Error("Failed to get total tokens", "error", err)
	}
	stats.TotalTokens24h = totalTokens

	// 获取Top 5模型
	type ModelStats struct {
		Name        string
		Total       int64
		Success     int64
		TotalTokens int64
		AvgTime     float64
	}
	
	var modelStats []ModelStats
	if err := models.DB.Model(&models.ChatLog{}).
		Select("name, COUNT(*) as total, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success, COALESCE(SUM(total_tokens), 0) as total_tokens, AVG(proxy_time) as avg_time").
		Where("created_at > ?", since).
		Group("name").
		Order("total DESC").
		Limit(5).
		Scan(&modelStats).Error; err != nil {
		slog.Error("Failed to get model stats", "error", err)
	}

	stats.TopModels = make([]ModelUsageStats, 0, len(modelStats))
	for _, ms := range modelStats {
		successRate := float64(0)
		if ms.Total > 0 {
			successRate = float64(ms.Success) / float64(ms.Total) * 100
		}
		stats.TopModels = append(stats.TopModels, ModelUsageStats{
			ModelName:       ms.Name,
			RequestCount:    ms.Total,
			SuccessRate:     successRate,
			TotalTokens:     ms.TotalTokens,
			AvgResponseTime: ms.AvgTime / float64(time.Millisecond),
		})
	}

	// 获取Top 5提供商
	type ProviderStats struct {
		Name        string
		Total       int64
		Success     int64
		TotalTokens int64
		AvgTime     float64
	}
	
	var providerStats []ProviderStats
	if err := models.DB.Model(&models.ChatLog{}).
		Select("provider_name, COUNT(*) as total, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success, COALESCE(SUM(total_tokens), 0) as total_tokens, AVG(proxy_time) as avg_time").
		Where("created_at > ?", since).
		Group("provider_name").
		Order("total DESC").
		Limit(5).
		Scan(&providerStats).Error; err != nil {
		slog.Error("Failed to get provider stats", "error", err)
	}

	stats.TopProviders = make([]ProviderUsageStats, 0, len(providerStats))
	for _, ps := range providerStats {
		successRate := float64(0)
		if ps.Total > 0 {
			successRate = float64(ps.Success) / float64(ps.Total) * 100
		}
		stats.TopProviders = append(stats.TopProviders, ProviderUsageStats{
			ProviderName:    ps.Name,
			RequestCount:    ps.Total,
			SuccessRate:     successRate,
			TotalTokens:     ps.TotalTokens,
			AvgResponseTime: ps.AvgTime / float64(time.Millisecond),
		})
	}

	// 计算健康提供商数量
	providers, _ := gorm.G[models.Provider](models.DB).Find(ctx)
	for _, provider := range providers {
		health := checkProviderHealth(ctx, &provider)
		if health.Status == "healthy" {
			stats.HealthyProviders++
		}
	}

	common.Success(c, stats)
}

// BatchDeleteProviders 批量删除提供商
func BatchDeleteProviders(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	// 开始事务
	tx := models.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除关联的模型提供商
	if err := tx.Where("provider_id IN ?", req.IDs).Delete(&models.ModelWithProvider{}).Error; err != nil {
		tx.Rollback()
		common.InternalServerError(c, "Failed to delete model-provider associations: "+err.Error())
		return
	}

	// 删除提供商
	result := tx.Where("id IN ?", req.IDs).Delete(&models.Provider{})
	if result.Error != nil {
		tx.Rollback()
		common.InternalServerError(c, "Failed to delete providers: "+result.Error.Error())
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		common.InternalServerError(c, "Failed to commit transaction: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted_count": result.RowsAffected,
		"deleted_ids":   req.IDs,
	})
}

// BatchDeleteModels 批量删除模型
func BatchDeleteModels(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	// 开始事务
	tx := models.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除关联的模型提供商
	if err := tx.Where("model_id IN ?", req.IDs).Delete(&models.ModelWithProvider{}).Error; err != nil {
		tx.Rollback()
		common.InternalServerError(c, "Failed to delete model-provider associations: "+err.Error())
		return
	}

	// 删除模型
	result := tx.Where("id IN ?", req.IDs).Delete(&models.Model{})
	if result.Error != nil {
		tx.Rollback()
		common.InternalServerError(c, "Failed to delete models: "+result.Error.Error())
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		common.InternalServerError(c, "Failed to commit transaction: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted_count": result.RowsAffected,
		"deleted_ids":   req.IDs,
	})
}

// ValidateProviderConfig 验证提供商配置
func ValidateProviderConfig(c *gin.Context) {
	var req ProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	result := ProviderValidationResult{}
	start := time.Now()

	// 尝试创建提供商实例
	chatModel, err := providers.New(req.Type, req.Config)
	if err != nil {
		result.Valid = false
		result.ErrorMessage = "Failed to initialize provider: " + err.Error()
		result.ResponseTime = time.Since(start).Milliseconds()
		common.Success(c, result)
		return
	}

	// 尝试获取模型列表
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	modelList, err := chatModel.Models(ctx)
	result.ResponseTime = time.Since(start).Milliseconds()

	if err != nil {
		result.Valid = false
		result.ErrorMessage = "Failed to fetch models: " + err.Error()
	} else {
		result.Valid = true
		result.Models = make([]string, 0, len(modelList))
		for _, model := range modelList {
			result.Models = append(result.Models, model.ID)
		}
	}

	common.Success(c, result)
}

// ExportLogs 导出日志为CSV
func ExportLogs(c *gin.Context) {
	// 获取筛选参数
	providerName := c.Query("provider_name")
	name := c.Query("name")
	status := c.Query("status")
	style := c.Query("style")
	
	// 时间范围参数
	daysStr := c.Query("days")
	days := 7 // 默认7天
	if daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 {
			days = parsedDays
		}
	}

	since := time.Now().AddDate(0, 0, -days)

	// 构建查询
	query := models.DB.Model(&models.ChatLog{}).Where("created_at > ?", since)

	if providerName != "" {
		query = query.Where("provider_name = ?", providerName)
	}
	if name != "" {
		query = query.Where("name = ?", name)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if style != "" {
		query = query.Where("style = ?", style)
	}

	// 获取数据
	var logs []models.ChatLog
	if err := query.Order("created_at DESC").Limit(10000).Find(&logs).Error; err != nil {
		common.InternalServerError(c, "Failed to query logs: "+err.Error())
		return
	}

	// 设置响应头
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=llmio_logs_%s.csv", time.Now().Format("20060102_150405")))

	// 创建CSV writer
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// 写入表头
	headers := []string{
		"ID", "CreatedAt", "ModelName", "ProviderModel", "ProviderName",
		"Status", "Style", "Error", "Retry", "ProxyTime(ms)", "FirstChunkTime(ms)",
		"ChunkTime(ms)", "TPS", "PromptTokens", "CompletionTokens", "TotalTokens",
	}
	if err := writer.Write(headers); err != nil {
		slog.Error("Failed to write CSV headers", "error", err)
		return
	}

	// 写入数据
	for _, log := range logs {
		record := []string{
			strconv.FormatUint(uint64(log.ID), 10),
			log.CreatedAt.Format("2006-01-02 15:04:05"),
			log.Name,
			log.ProviderModel,
			log.ProviderName,
			log.Status,
			log.Style,
			log.Error,
			strconv.Itoa(log.Retry),
			strconv.FormatInt(log.ProxyTime.Milliseconds(), 10),
			strconv.FormatInt(log.FirstChunkTime.Milliseconds(), 10),
			strconv.FormatInt(log.ChunkTime.Milliseconds(), 10),
			fmt.Sprintf("%.2f", log.Tps),
			strconv.FormatInt(log.PromptTokens, 10),
			strconv.FormatInt(log.CompletionTokens, 10),
			strconv.FormatInt(log.TotalTokens, 10),
		}
		if err := writer.Write(record); err != nil {
			slog.Error("Failed to write CSV record", "error", err)
			continue
		}
	}
}

// ExportConfig 导出配置为JSON
func ExportConfig(c *gin.Context) {
	config := make(map[string]interface{})

	// 获取所有提供商
	providers, err := gorm.G[models.Provider](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve providers: "+err.Error())
		return
	}

	// 脱敏处理API密钥
	for i := range providers {
		// 解析配置并脱敏
		configStr := providers[i].Config
		if strings.Contains(configStr, "api_key") {
			// 简单替换，实际应该解析JSON后处理
			providers[i].Config = strings.ReplaceAll(configStr, `"api_key"`, `"api_key":"***REDACTED***","original_api_key"`)
		}
	}
	config["providers"] = providers

	// 获取所有模型
	modelsData, err := gorm.G[models.Model](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve models: "+err.Error())
		return
	}
	config["models"] = modelsData

	// 获取所有模型提供商关联
	modelProviders, err := gorm.G[models.ModelWithProvider](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve model-provider associations: "+err.Error())
		return
	}
	config["model_providers"] = modelProviders

	// 添加导出元数据
	config["exported_at"] = time.Now().Format(time.RFC3339)
	config["version"] = "1.0"

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=llmio_config_%s.json", time.Now().Format("20060102_150405")))
	
	common.SuccessRaw(c, config)
}

// GetHealthCheckConfig 获取健康检查配置
func GetHealthCheckConfig(c *gin.Context) {
	var config models.HealthCheckConfig
	if err := models.DB.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认配置
			config = models.HealthCheckConfig{
				Enabled:         true,
				IntervalMinutes: 5,
				MaxErrorCount:   5,
				RetryAfterHours: 1,
			}
			common.Success(c, config)
			return
		}
		common.InternalServerError(c, "Failed to get config: "+err.Error())
		return
	}
	common.Success(c, config)
}

// UpdateHealthCheckConfig 更新健康检查配置
func UpdateHealthCheckConfig(c *gin.Context) {
	var req models.HealthCheckConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// 验证配置值
	if req.IntervalMinutes < 1 {
		common.BadRequest(c, "IntervalMinutes must be at least 1")
		return
	}
	if req.MaxErrorCount < 1 {
		common.BadRequest(c, "MaxErrorCount must be at least 1")
		return
	}
	if req.RetryAfterHours < 0 {
		common.BadRequest(c, "RetryAfterHours cannot be negative")
		return
	}

	// 获取现有配置
	var config models.HealthCheckConfig
	err := models.DB.First(&config).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		common.InternalServerError(c, "Failed to get config: "+err.Error())
		return
	}

	// 更新配置
	config.Enabled = req.Enabled
	config.IntervalMinutes = req.IntervalMinutes
	config.MaxErrorCount = req.MaxErrorCount
	config.RetryAfterHours = req.RetryAfterHours

	if err == gorm.ErrRecordNotFound {
		if err := models.DB.Create(&config).Error; err != nil {
			common.InternalServerError(c, "Failed to create config: "+err.Error())
			return
		}
	} else {
		if err := models.DB.Save(&config).Error; err != nil {
			common.InternalServerError(c, "Failed to update config: "+err.Error())
			return
		}
	}

	slog.Info("Health check config updated",
		"enabled", config.Enabled,
		"interval", config.IntervalMinutes,
		"max_errors", config.MaxErrorCount,
		"retry_after", config.RetryAfterHours)

	common.Success(c, config)
}

// ForceHealthCheck 强制执行健康检查
func ForceHealthCheck(c *gin.Context) {
	providerIDStr := c.Param("id")
	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid provider ID format")
		return
	}

	if err := service.ForceCheckProvider(c.Request.Context(), models.DB, uint(providerID)); err != nil {
		common.InternalServerError(c, "Failed to check provider: "+err.Error())
		return
	}

	// 获取更新后的健康状态
	var provider models.Provider
	if err := models.DB.First(&provider, providerID).Error; err != nil {
		common.NotFound(c, "Provider not found")
		return
	}

	healthStatus := checkProviderHealth(c.Request.Context(), &provider)
	common.Success(c, healthStatus)
}

// GetRealtimeStats 获取实时统计数据（用于仪表板刷新）
func GetRealtimeStats(c *gin.Context) {
	stats := make(map[string]interface{})
	
	// 最近1小时的统计
	since := time.Now().Add(-1 * time.Hour)
	
	var total, success int64
	var avgResponseTime float64
	
	models.DB.Model(&models.ChatLog{}).
		Where("created_at > ?", since).
		Count(&total)
	
	models.DB.Model(&models.ChatLog{}).
		Where("created_at > ? AND status = ?", since, "success").
		Count(&success)
	
	models.DB.Model(&models.ChatLog{}).
		Select("AVG(proxy_time) as avg_time").
		Where("created_at > ? AND status = ?", since, "success").
		Row().Scan(&avgResponseTime)
	
	successRate := float64(0)
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}
	
	stats["requests_1h"] = total
	stats["success_rate_1h"] = successRate
	stats["avg_response_time_1h"] = avgResponseTime / float64(time.Millisecond)
	stats["timestamp"] = time.Now().Unix()
	
	common.Success(c, stats)
}

// ImportConfig 导入配置
func ImportConfig(c *gin.Context) {
	var config struct {
		Providers       []models.Provider          `json:"providers"`
		Models          []models.Model             `json:"models"`
		ModelProviders  []models.ModelWithProvider `json:"model_providers"`
	}

	if err := c.ShouldBindJSON(&config); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// 开始事务
	tx := models.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	importedCount := 0

	// 导入提供商
	for _, provider := range config.Providers {
		// 检查是否已存在同名提供商
		var existing models.Provider
		if err := tx.Where("name = ?", provider.Name).First(&existing).Error; err == nil {
			// 已存在,跳过
			continue
		}

		provider.ID = 0 // 重置ID让数据库自动生成
		if err := tx.Create(&provider).Error; err != nil {
			tx.Rollback()
			common.InternalServerError(c, "Failed to import provider: "+err.Error())
			return
		}
		importedCount++
	}

	// 导入模型
	for _, model := range config.Models {
		// 检查是否已存在同名模型
		var existing models.Model
		if err := tx.Where("name = ?", model.Name).First(&existing).Error; err == nil {
			// 已存在,跳过
			continue
		}

		model.ID = 0
		if err := tx.Create(&model).Error; err != nil {
			tx.Rollback()
			common.InternalServerError(c, "Failed to import model: "+err.Error())
			return
		}
		importedCount++
	}

	// 导入模型-提供商关联
	for _, mp := range config.ModelProviders {
		mp.ID = 0
		// 需要根据名称找到新的模型ID和提供商ID
		var model models.Model
		var provider models.Provider
		
		if err := tx.First(&model, mp.ModelID).Error; err != nil {
			continue // 模型不存在,跳过
		}
		if err := tx.First(&provider, mp.ProviderID).Error; err != nil {
			continue // 提供商不存在,跳过
		}

		// 检查关联是否已存在
		var existing models.ModelWithProvider
		if err := tx.Where("model_id = ? AND provider_id = ? AND provider_model = ?",
			model.ID, provider.ID, mp.ProviderModel).First(&existing).Error; err == nil {
			continue // 已存在,跳过
		}

		if err := tx.Create(&mp).Error; err != nil {
			tx.Rollback()
			common.InternalServerError(c, "Failed to import model-provider association: "+err.Error())
			return
		}
		importedCount++
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		common.InternalServerError(c, "Failed to commit transaction: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"imported_count": importedCount,
		"message": "Configuration imported successfully",
	})
}

// ClearLogs 清理请求日志
func ClearLogs(c *gin.Context) {
	// 获取清理参数
	daysStr := c.Query("days")
	if daysStr == "" {
		common.BadRequest(c, "days parameter is required")
		return
	}

	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 0 {
		common.BadRequest(c, "Invalid days parameter")
		return
	}

	// 计算截止时间
	cutoffTime := time.Now().AddDate(0, 0, -days)

	// 删除日志
	result := models.DB.Where("created_at < ?", cutoffTime).Delete(&models.ChatLog{})
	if result.Error != nil {
		common.InternalServerError(c, "Failed to clear logs: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted_count": result.RowsAffected,
		"cutoff_date": cutoffTime.Format("2006-01-02 15:04:05"),
	})
}