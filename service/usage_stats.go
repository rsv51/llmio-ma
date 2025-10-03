package service

import (
	"context"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// UpdateProviderUsageStats 更新提供商使用统计
func UpdateProviderUsageStats(ctx context.Context, db *gorm.DB, providerID uint, log models.ChatLog) error {
	today := time.Now().Truncate(24 * time.Hour)
	
	var stats models.ProviderUsageStats
	err := db.Where("provider_id = ? AND date = ?", providerID, today).First(&stats).Error
	
	if err == gorm.ErrRecordNotFound {
		// 创建新记录
		stats = models.ProviderUsageStats{
			ProviderID:    providerID,
			Date:          today,
			TotalRequests: 1,
			LastUsedAt:    time.Now(),
		}
		
		if log.Status == "success" {
			stats.SuccessRequests = 1
			stats.TotalTokens = log.TotalTokens
			stats.PromptTokens = log.PromptTokens
			stats.CompletionTokens = log.CompletionTokens
			stats.AvgResponseTime = float64(log.ProxyTime.Milliseconds())
		} else {
			stats.FailedRequests = 1
		}
		
		return db.Create(&stats).Error
	} else if err != nil {
		return err
	}
	
	// 更新现有记录
	stats.TotalRequests++
	stats.LastUsedAt = time.Now()
	
	if log.Status == "success" {
		stats.SuccessRequests++
		stats.TotalTokens += log.TotalTokens
		stats.PromptTokens += log.PromptTokens
		stats.CompletionTokens += log.CompletionTokens
		
		// 计算新的平均响应时间
		totalRequests := float64(stats.SuccessRequests)
		oldTotal := stats.AvgResponseTime * (totalRequests - 1)
		stats.AvgResponseTime = (oldTotal + float64(log.ProxyTime.Milliseconds())) / totalRequests
	} else {
		stats.FailedRequests++
	}
	
	return db.Save(&stats).Error
}

// GetProviderUsageStats 获取提供商使用统计
func GetProviderUsageStats(ctx context.Context, db *gorm.DB, providerID uint, days int) ([]models.ProviderUsageStats, error) {
	startDate := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	
	var stats []models.ProviderUsageStats
	err := db.Where("provider_id = ? AND date >= ?", providerID, startDate).
		Order("date DESC").
		Find(&stats).Error
	
	return stats, err
}

// GetAllProvidersUsageStats 获取所有提供商的使用统计
func GetAllProvidersUsageStats(ctx context.Context, db *gorm.DB, days int) ([]models.ProviderUsageStats, error) {
	startDate := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	
	var stats []models.ProviderUsageStats
	err := db.Where("date >= ?", startDate).
		Order("provider_id ASC, date DESC").
		Find(&stats).Error
	
	return stats, err
}

// GetProviderSuccessRate 获取提供商成功率
func GetProviderSuccessRate(ctx context.Context, db *gorm.DB, providerID uint, days int) (float64, error) {
	startDate := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	
	var stats []models.ProviderUsageStats
	err := db.Where("provider_id = ? AND date >= ?", providerID, startDate).Find(&stats).Error
	if err != nil {
		return 0, err
	}
	
	if len(stats) == 0 {
		return 0, nil
	}
	
	var totalRequests, successRequests int64
	for _, stat := range stats {
		totalRequests += stat.TotalRequests
		successRequests += stat.SuccessRequests
	}
	
	if totalRequests == 0 {
		return 0, nil
	}
	
	return float64(successRequests) / float64(totalRequests) * 100, nil
}

// SelectLeastUsedProvider 选择使用最少的提供商
func SelectLeastUsedProvider(ctx context.Context, db *gorm.DB, providerIDs []uint) (uint, error) {
	if len(providerIDs) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	
	today := time.Now().Truncate(24 * time.Hour)
	
	// 查询今日各提供商的使用统计
	var stats []models.ProviderUsageStats
	err := db.Where("provider_id IN ? AND date = ?", providerIDs, today).Find(&stats).Error
	if err != nil {
		return 0, err
	}
	
	// 创建使用次数映射
	usageMap := make(map[uint]int64)
	for _, id := range providerIDs {
		usageMap[id] = 0
	}
	
	for _, stat := range stats {
		usageMap[stat.ProviderID] = stat.TotalRequests
	}
	
	// 找到使用次数最少的提供商
	var minUsage int64 = -1
	var selectedID uint
	
	for id, usage := range usageMap {
		if minUsage == -1 || usage < minUsage {
			minUsage = usage
			selectedID = id
		}
	}
	
	return selectedID, nil
}

// CleanOldUsageStats 清理旧的使用统计数据
func CleanOldUsageStats(ctx context.Context, db *gorm.DB, daysToKeep int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep).Truncate(24 * time.Hour)
	return db.Where("date < ?", cutoffDate).Delete(&models.ProviderUsageStats{}).Error
}