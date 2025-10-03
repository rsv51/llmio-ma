package models

import (
	"time"

	"gorm.io/gorm"
)

type Provider struct {
	gorm.Model
	Name    string
	Type    string `gorm:"index"` // 为type字段创建索引
	Config  string
	Console string // 控制台地址
}

type AnthropicConfig struct {
	BaseUrl string `json:"base_url"`
	ApiKey  string `json:"api_key"`
	Version string `json:"version"`
}

type Model struct {
	gorm.Model
	Name     string `gorm:"index"` // 为name字段创建索引
	Remark   string
	MaxRetry int // 重试次数限制
	TimeOut  int // 超时时间 单位秒
}

type ModelWithProvider struct {
	gorm.Model
	ModelID          uint   `gorm:"index:idx_model_provider"` // 复合索引的一部分
	ProviderModel    string
	ProviderID       uint   `gorm:"index:idx_model_provider"` // 复合索引的一部分
	ToolCall         *bool  // 能否接受带有工具调用的请求
	StructuredOutput *bool  // 能否接受带有结构化输出的请求
	Image            *bool  // 能否接受带有图片的请求(视觉)
	Weight           int
}

type ChatLog struct {
	gorm.Model
	Name          string
	ProviderModel string
	ProviderName  string `gorm:"index:idx_provider_status"` // 复合索引的一部分
	Status        string `gorm:"index:idx_provider_status"` // 复合索引的一部分
	Style         string // 类型

	Error          string        // if status is error, this field will be set
	Retry          int           // 重试次数
	ProxyTime      time.Duration // 代理耗时
	FirstChunkTime time.Duration // 首个chunk耗时
	ChunkTime      time.Duration // chunk耗时
	Tps            float64
	Usage
}

func (ChatLog) TableIndexes() [][]string {
	return [][]string{{"CreatedAt"}}
}

func (l ChatLog) WithError(err error) ChatLog {
	l.Error = err.Error()
	l.Status = "error"
	return l
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// ProviderValidation 提供商验证状态表 - 用于智能健康检查
type ProviderValidation struct {
	gorm.Model
	ProviderID       uint      `gorm:"uniqueIndex;not null"` // 提供商ID，唯一索引
	IsHealthy        bool      `gorm:"default:true"`         // 是否健康
	ErrorCount       int       `gorm:"default:0"`            // 连续错误次数
	LastError        string    `gorm:"type:text"`            // 最后一次错误信息
	LastStatusCode   int       `gorm:"default:0"`            // 最后一次HTTP状态码
	LastValidatedAt  time.Time `gorm:"index"`                // 最后一次验证时间
	LastSuccessAt    *time.Time                              // 最后一次成功时间
	NextRetryAt      *time.Time `gorm:"index"`               // 下次重试时间
	ConsecutiveSuccesses int    `gorm:"default:0"`           // 连续成功次数
}

// ProviderUsageStats 提供商使用统计表 - 持久化统计数据
type ProviderUsageStats struct {
	gorm.Model
	ProviderID       uint      `gorm:"uniqueIndex:idx_provider_date;not null"` // 提供商ID
	Date             time.Time `gorm:"uniqueIndex:idx_provider_date;not null;type:date"` // 统计日期
	TotalRequests    int64     `gorm:"default:0"`  // 总请求数
	SuccessRequests  int64     `gorm:"default:0"`  // 成功请求数
	FailedRequests   int64     `gorm:"default:0"`  // 失败请求数
	TotalTokens      int64     `gorm:"default:0"`  // 总token数
	PromptTokens     int64     `gorm:"default:0"`  // prompt token数
	CompletionTokens int64     `gorm:"default:0"`  // completion token数
	AvgResponseTime  float64   `gorm:"default:0"`  // 平均响应时间(毫秒)
	LastUsedAt       time.Time `gorm:"index"`      // 最后使用时间
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	gorm.Model
	Enabled         bool `gorm:"default:true"`  // 是否启用健康检查
	IntervalMinutes int  `gorm:"default:5"`     // 检查间隔(分钟)
	MaxErrorCount   int  `gorm:"default:5"`     // 最大错误次数
	RetryAfterHours int  `gorm:"default:1"`     // 错误后多久重试(小时)
}
