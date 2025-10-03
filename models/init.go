package models

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(name string) {
	db, err := gorm.Open(sqlite.Open(name))
	if err != nil {
		panic(err)
	}
	DB = db
	
	// 执行自动迁移
	if err := db.AutoMigrate(
		&Provider{},
		&Model{},
		&ModelWithProvider{},
		&ChatLog{},
		&ProviderValidation{},
		&ProviderUsageStats{},
		&HealthCheckConfig{},
	); err != nil {
		panic(err)
	}
	
	// 初始化默认健康检查配置
	initHealthCheckConfig(db)
	
	// 创建性能优化索引
	createPerformanceIndexes(db)
}

// initHealthCheckConfig 初始化健康检查配置
func initHealthCheckConfig(db *gorm.DB) {
	var config HealthCheckConfig
	if err := db.First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建默认配置
			config = HealthCheckConfig{
				Enabled:         true,
				IntervalMinutes: 5,
				MaxErrorCount:   5,
				RetryAfterHours: 1,
			}
			db.Create(&config)
		}
	}
}

// createPerformanceIndexes 创建数据库性能优化索引
func createPerformanceIndexes(db *gorm.DB) {
	// ChatLogs表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_logs_created_at ON chat_logs(created_at DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_logs_provider_name ON chat_logs(provider_name)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_logs_name ON chat_logs(name)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_logs_status ON chat_logs(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_logs_style ON chat_logs(style)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_logs_filter_composite ON chat_logs(provider_name, name, status, style)")
	
	// ModelWithProvider表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_model_with_provider_model_id ON model_with_providers(model_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_model_with_provider_provider_id ON model_with_providers(provider_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_model_with_provider_composite ON model_with_providers(model_id, provider_id)")
	
	// Provider表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_providers_type ON providers(type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_providers_name ON providers(name)")
	
	// Model表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_models_name ON models(name)")
}
