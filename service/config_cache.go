package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// ConfigCache 配置缓存结构体
type ConfigCache struct {
	cacheMutex       sync.RWMutex
	modelCache       map[string]*models.Model                    // 模型名称 -> 模型配置
	providerCache    map[uint]*models.Provider                   // 提供商ID -> 提供商配置
	modelProviderCache map[string][]models.ModelWithProvider     // 模型名称 -> 模型提供商列表
	lastRefreshTime  time.Time                                   // 最后刷新时间
	cacheTTL         time.Duration                              // 缓存TTL
	refreshing       sync.Mutex                                  // 刷新锁，防止并发刷新
}

// NewConfigCache 创建新的配置缓存实例
func NewConfigCache(ttl time.Duration) *ConfigCache {
	return &ConfigCache{
		modelCache:        make(map[string]*models.Model),
		providerCache:     make(map[uint]*models.Provider),
		modelProviderCache: make(map[string][]models.ModelWithProvider),
		cacheTTL:          ttl,
		lastRefreshTime:   time.Now(),
	}
}

// GetModel 获取模型配置，支持缓存
func (cc *ConfigCache) GetModel(ctx context.Context, modelName string) (*models.Model, error) {
	// 先尝试读取缓存
	cc.cacheMutex.RLock()
	model, exists := cc.modelCache[modelName]
	isExpired := cc.isCacheExpired()
	cc.cacheMutex.RUnlock()

	// 如果缓存过期，异步刷新（避免阻塞请求）
	if isExpired {
		go func() {
			if err := cc.refreshCache(context.Background()); err != nil {
				slog.Warn("refresh cache failed", "error", err)
			}
		}()
	}

	if exists && model != nil {
		slog.Debug("cache hit for model", "model", modelName)
		return model, nil
	}

	// 缓存未命中，查询数据库
	slog.Debug("cache miss for model, querying database", "model", modelName)
	model, err := cc.queryModelFromDB(ctx, modelName)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	cc.cacheMutex.Lock()
	cc.modelCache[modelName] = model
	cc.cacheMutex.Unlock()

	return model, nil
}

// GetProvider 获取提供商配置，支持缓存
func (cc *ConfigCache) GetProvider(ctx context.Context, providerID uint) (*models.Provider, error) {
	// 先尝试读取缓存
	cc.cacheMutex.RLock()
	provider, exists := cc.providerCache[providerID]
	isExpired := cc.isCacheExpired()
	cc.cacheMutex.RUnlock()

	// 如果缓存过期，异步刷新
	if isExpired {
		go func() {
			if err := cc.refreshCache(context.Background()); err != nil {
				slog.Warn("refresh cache failed", "error", err)
			}
		}()
	}

	if exists && provider != nil {
		slog.Debug("cache hit for provider", "providerID", providerID)
		return provider, nil
	}

	// 缓存未命中，查询数据库
	slog.Debug("cache miss for provider, querying database", "providerID", providerID)
	provider, err := cc.queryProviderFromDB(ctx, providerID)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	cc.cacheMutex.Lock()
	cc.providerCache[providerID] = provider
	cc.cacheMutex.Unlock()

	return provider, nil
}

// GetModelProviders 获取模型对应的提供商列表，支持缓存
func (cc *ConfigCache) GetModelProviders(ctx context.Context, modelName string) ([]models.ModelWithProvider, error) {
	// 先尝试读取缓存
	cc.cacheMutex.RLock()
	providers, exists := cc.modelProviderCache[modelName]
	isExpired := cc.isCacheExpired()
	cc.cacheMutex.RUnlock()

	// 如果缓存过期，异步刷新
	if isExpired {
		go func() {
			if err := cc.refreshCache(context.Background()); err != nil {
				slog.Warn("refresh cache failed", "error", err)
			}
		}()
	}

	if exists && providers != nil {
		slog.Debug("cache hit for model providers", "model", modelName, "count", len(providers))
		return providers, nil
	}

	// 缓存未命中，查询数据库
	slog.Debug("cache miss for model providers, querying database", "model", modelName)
	providers, err := cc.queryModelProvidersFromDB(ctx, modelName)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	cc.cacheMutex.Lock()
	cc.modelProviderCache[modelName] = providers
	cc.cacheMutex.Unlock()

	return providers, nil
}

// ProvidersBymodelsNameWithCache 带缓存的ProvidersBymodelsName函数
func (cc *ConfigCache) ProvidersBymodelsNameWithCache(ctx context.Context, modelName string) (*ProvidersWithlimit, error) {
	// 获取模型配置
	model, err := cc.GetModel(ctx, modelName)
	if err != nil {
		return nil, err
	}

	// 获取模型对应的提供商列表
	modelProviders, err := cc.GetModelProviders(ctx, modelName)
	if err != nil {
		return nil, err
	}

	if len(modelProviders) == 0 {
		return nil, errors.New("not provider for model " + modelName)
	}

	return &ProvidersWithlimit{
		Providers: modelProviders,
		MaxRetry:  model.MaxRetry,
		TimeOut:   model.TimeOut,
	}, nil
}

// refreshCache 刷新整个缓存
func (cc *ConfigCache) refreshCache(ctx context.Context) error {
	// 使用独立的刷新锁，防止并发刷新
	if !cc.refreshing.TryLock() {
		slog.Debug("cache refresh already in progress, skipping")
		return nil
	}
	defer cc.refreshing.Unlock()

	// 再次检查是否需要刷新（双重检查）
	cc.cacheMutex.RLock()
	if !cc.isCacheExpired() {
		cc.cacheMutex.RUnlock()
		return nil
	}
	cc.cacheMutex.RUnlock()

	slog.Info("refreshing config cache")

	cc.cacheMutex.Lock()
	defer cc.cacheMutex.Unlock()

	// 清空缓存
	cc.modelCache = make(map[string]*models.Model)
	cc.providerCache = make(map[uint]*models.Provider)
	cc.modelProviderCache = make(map[string][]models.ModelWithProvider)

	// 使用JOIN查询一次性获取所有相关数据，避免N+1问题
	var modelProviders []struct {
		models.ModelWithProvider
		ModelName    string `gorm:"column:model_name"`
		ProviderName string `gorm:"column:provider_name"`
		ProviderType string `gorm:"column:provider_type"`
	}

	// 执行JOIN查询获取模型提供商关系及其关联信息
	err := models.DB.Table("model_with_providers").
		Select(`model_with_providers.*, 
			models.name as model_name, 
			providers.name as provider_name, 
			providers.type as provider_type`).
		Joins("LEFT JOIN models ON model_with_providers.model_id = models.id").
		Joins("LEFT JOIN providers ON model_with_providers.provider_id = providers.id").
		Find(&modelProviders).Error
	
	if err != nil {
		return err
	}

	// 查询所有模型
	var allModels []models.Model
	allModels, err = gorm.G[models.Model](models.DB).Find(ctx)
	if err != nil {
		return err
	}

	for i := range allModels {
		model := &allModels[i]
		cc.modelCache[model.Name] = model
	}

	// 查询所有提供商
	var allProviders []models.Provider
	allProviders, err = gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		return err
	}

	for i := range allProviders {
		provider := &allProviders[i]
		cc.providerCache[provider.ID] = provider
	}

	// 按模型名称分组模型提供商关系
	for _, mp := range modelProviders {
		if mp.ModelName != "" {
			cc.modelProviderCache[mp.ModelName] = append(cc.modelProviderCache[mp.ModelName], mp.ModelWithProvider)
		}
	}

	cc.lastRefreshTime = time.Now()
	slog.Info("config cache refreshed successfully", "models", len(allModels), "providers", len(allProviders), "modelProviders", len(modelProviders))

	return nil
}

// isCacheExpired 检查缓存是否过期
func (cc *ConfigCache) isCacheExpired() bool {
	return time.Since(cc.lastRefreshTime) > cc.cacheTTL
}

// queryModelFromDB 从数据库查询模型配置
func (cc *ConfigCache) queryModelFromDB(ctx context.Context, modelName string) (*models.Model, error) {
	model, err := gorm.G[models.Model](models.DB).Where("name = ?", modelName).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found model " + modelName)
		}
		return nil, err
	}
	return &model, nil
}

// queryProviderFromDB 从数据库查询提供商配置
func (cc *ConfigCache) queryProviderFromDB(ctx context.Context, providerID uint) (*models.Provider, error) {
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", providerID).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found provider " + strconv.FormatUint(uint64(providerID), 10))
		}
		return nil, err
	}
	return &provider, nil
}

// queryModelProvidersFromDB 从数据库查询模型提供商关系
func (cc *ConfigCache) queryModelProvidersFromDB(ctx context.Context, modelName string) ([]models.ModelWithProvider, error) {
	// 先获取模型ID
	var model models.Model
	model, err := gorm.G[models.Model](models.DB).Where("name = ?", modelName).First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found model " + modelName)
		}
		return nil, err
	}

	// 获取模型对应的提供商列表
	var modelProviders []models.ModelWithProvider
	modelProviders, err = gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", model.ID).Find(ctx)
	if err != nil {
		return nil, err
	}

	return modelProviders, nil
}

// GetCacheStats 获取缓存统计信息
func (cc *ConfigCache) GetCacheStats() map[string]interface{} {
	cc.cacheMutex.RLock()
	defer cc.cacheMutex.RUnlock()

	return map[string]interface{}{
		"models_cached":        len(cc.modelCache),
		"providers_cached":     len(cc.providerCache),
		"model_providers_cached": len(cc.modelProviderCache),
		"last_refresh_time":    cc.lastRefreshTime.Format(time.RFC3339),
		"cache_ttl":            cc.cacheTTL.String(),
		"is_expired":           cc.isCacheExpired(),
	}
}

// ClearCache 清空缓存
func (cc *ConfigCache) ClearCache() {
	cc.cacheMutex.Lock()
	defer cc.cacheMutex.Unlock()

	cc.modelCache = make(map[string]*models.Model)
	cc.providerCache = make(map[uint]*models.Provider)
	cc.modelProviderCache = make(map[string][]models.ModelWithProvider)
	cc.lastRefreshTime = time.Now()

	slog.Info("config cache cleared")
}