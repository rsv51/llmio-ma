# LLMIO OrchestrationApi 增强功能文档

本文档详细说明了从 OrchestrationApi 借鉴并实现到 llmio 的核心功能增强。

## 📋 目录

- [功能概览](#功能概览)
- [1. 智能密钥健康检查系统](#1-智能密钥健康检查系统)
- [2. 密钥验证状态持久化追踪](#2-密钥验证状态持久化追踪)
- [3. 多层级智能故障转移](#3-多层级智能故障转移)
- [4. 基于数据库的使用统计](#4-基于数据库的使用统计)
- [5. 配置化后台服务](#5-配置化后台服务)
- [6. 增强的健康状态API](#6-增强的健康状态api)
- [数据库架构](#数据库架构)
- [API 端点](#api-端点)
- [配置说明](#配置说明)
- [迁移指南](#迁移指南)
- [最佳实践](#最佳实践)

---

## 功能概览

### 实施的核心功能 ✅

| 功能 | OrchestrationApi | llmio (增强后) | 优先级 |
|------|-----------------|---------------|--------|
| 后台健康检查 | ✅ 每5分钟自动检查 | ✅ 可配置间隔 | ⭐⭐⭐ |
| 状态持久化 | ✅ KeyValidation表 | ✅ ProviderValidation表 | ⭐⭐⭐ |
| 智能故障转移 | ✅ 多层级降级 | ✅ excludedProviderIDs | ⭐⭐⭐ |
| 使用统计 | ✅ 数据库查询 | ✅ ProviderUsageStats表 | ⭐⭐ |
| 配置化服务 | ✅ appsettings.json | ✅ 数据库配置 | ⭐⭐ |
| 详细健康API | ✅ 完整错误信息 | ✅ 增强响应 | ⭐ |

---

## 1. 智能密钥健康检查系统

### 功能描述

后台服务自动定期检查所有提供商的健康状态，无需手动干预。

### 核心特性

- ✅ **自动恢复检测**：定期检查标记为 unhealthy 的提供商，自动恢复可用性
- ✅ **真实 API 调用验证**：使用实际 API 请求验证提供商状态
- ✅ **详细状态记录**：记录状态码、错误信息、响应时间等
- ✅ **智能重试机制**：根据配置的时间间隔自动重试

### 实现文件

```
service/health_check.go          # 健康检查服务核心实现
models/model.go                  # ProviderValidation 数据模型
main.go                          # 服务启动和优雅关闭
```

### 工作流程

```
启动应用
    ↓
初始化 HealthCheckService
    ↓
立即执行首次检查
    ↓
启动定时器 (默认5分钟)
    ↓
┌─────────────────────────┐
│  每隔 N 分钟执行检查     │
│  ┌──────────────────┐   │
│  │ 获取所有提供商    │   │
│  └──────────────────┘   │
│          ↓              │
│  ┌──────────────────┐   │
│  │ 检查健康状态      │   │
│  └──────────────────┘   │
│          ↓              │
│  ┌──────────────────┐   │
│  │ 更新数据库        │   │
│  └──────────────────┘   │
└─────────────────────────┘
    ↓
收到停止信号
    ↓
优雅关闭
```

### 使用示例

```go
// 服务自动启动，无需手动操作
// 在 main.go 中已集成

// 查看健康状态
GET /api/providers/health

// 查看单个提供商健康状态
GET /api/providers/health/:id

// 强制检查特定提供商
POST /api/health-check/force/:id
```

---

## 2. 密钥验证状态持久化追踪

### 功能描述

将提供商的验证状态持久化到数据库，替代简单的内存统计。

### 数据模型

```go
type ProviderValidation struct {
    gorm.Model
    ProviderID           uint       // 提供商ID
    IsHealthy            bool       // 是否健康
    ErrorCount           int        // 连续错误次数
    LastError            string     // 最后错误信息
    LastStatusCode       int        // 最后HTTP状态码
    LastValidatedAt      time.Time  // 最后验证时间
    LastSuccessAt        *time.Time // 最后成功时间
    NextRetryAt          *time.Time // 下次重试时间
    ConsecutiveSuccesses int        // 连续成功次数
}
```

### 智能标记机制

```
请求成功
    ↓
ConsecutiveSuccesses++
    ↓
ErrorCount = 0
    ↓
IsHealthy = true

请求失败
    ↓
ErrorCount++
    ↓
ConsecutiveSuccesses = 0
    ↓
ErrorCount >= MaxErrorCount?
    ↓ Yes
IsHealthy = false
NextRetryAt = Now + RetryAfterHours
```

### 状态查询

```go
// 获取提供商健康状态
validation, err := service.GetProviderHealth(ctx, db, providerID)

// 获取所有提供商健康状态
validations, err := service.GetAllProvidersHealth(ctx, db)
```

---

## 3. 多层级智能故障转移

### 功能描述

支持排除失败的提供商，自动降级到备用提供商。

### 增强的路由函数

```go
// 原始函数（向后兼容）
func BalanceChat(c *gin.Context, style string, Beforer Beforer, processer Processer) error

// 增强函数（支持排除）
func BalanceChatWithExclusions(c *gin.Context, style string, Beforer Beforer, processer Processer, excludedProviderIDs []uint) error
```

### 故障转移策略

```
请求到达
    ↓
获取所有可用提供商
    ↓
过滤排除列表中的提供商
    ↓
过滤 IsHealthy = false 的提供商
    ↓
优先使用健康提供商
    ↓
如果没有健康提供商
    ↓
降级：使用所有提供商
    ↓
加权随机选择
    ↓
请求失败？
    ↓ Yes
更新健康状态
移除该提供商
重试下一个
```

### 实时健康状态更新

```go
// 请求成功时
go updateProviderHealthOnSuccess(context.Background(), providerID)

// 请求失败时
go updateProviderHealthOnError(context.Background(), providerID, errorMsg, statusCode)
```

---

## 4. 基于数据库的使用统计

### 功能描述

将使用统计持久化到数据库，支持历史数据分析和长期趋势追踪。

### 数据模型

```go
type ProviderUsageStats struct {
    gorm.Model
    ProviderID       uint      // 提供商ID
    Date             time.Time // 统计日期
    TotalRequests    int64     // 总请求数
    SuccessRequests  int64     // 成功请求数
    FailedRequests   int64     // 失败请求数
    TotalTokens      int64     // 总token数
    PromptTokens     int64     // prompt token数
    CompletionTokens int64     // completion token数
    AvgResponseTime  float64   // 平均响应时间(毫秒)
    LastUsedAt       time.Time // 最后使用时间
}
```

### 核心功能

#### 自动统计更新

```go
// 在每次请求完成后自动更新
go UpdateProviderUsageStats(context.Background(), models.DB, providerID, log)
```

#### 查询使用统计

```go
// 获取提供商N天的使用统计
stats, err := service.GetProviderUsageStats(ctx, db, providerID, days)

// 获取所有提供商的使用统计
allStats, err := service.GetAllProvidersUsageStats(ctx, db, days)

// 获取提供商成功率
successRate, err := service.GetProviderSuccessRate(ctx, db, providerID, days)
```

#### 最少使用选择

```go
// 选择使用次数最少的提供商（负载均衡）
leastUsedID, err := service.SelectLeastUsedProvider(ctx, db, providerIDs)
```

#### 数据清理

```go
// 清理旧的统计数据（保留N天）
err := service.CleanOldUsageStats(ctx, db, daysToKeep)
```

---

## 5. 配置化后台服务

### 功能描述

通过数据库配置控制健康检查服务的行为，支持动态调整。

### 配置模型

```go
type HealthCheckConfig struct {
    gorm.Model
    Enabled         bool // 是否启用健康检查
    IntervalMinutes int  // 检查间隔(分钟)
    MaxErrorCount   int  // 最大错误次数
    RetryAfterHours int  // 错误后多久重试(小时)
}
```

### 默认配置

```json
{
  "enabled": true,
  "interval_minutes": 5,
  "max_error_count": 5,
  "retry_after_hours": 1
}
```

### API 管理

```bash
# 获取配置
GET /api/health-check/config

# 更新配置
PUT /api/health-check/config
Content-Type: application/json

{
  "enabled": true,
  "interval_minutes": 10,
  "max_error_count": 3,
  "retry_after_hours": 2
}
```

### 动态调整

配置更新后，健康检查服务会在下次检查周期自动应用新配置。

---

## 6. 增强的健康状态API

### 响应格式

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "provider_id": 1,
    "provider_name": "openai-primary",
    "provider_type": "openai",
    "status": "healthy",
    "is_healthy": true,
    "response_time_ms": 234,
    "last_checked": "2024-01-15T10:30:00Z",
    "last_success": "2024-01-15T10:30:00Z",
    "error_message": "",
    "error_count": 0,
    "consecutive_successes": 125,
    "next_retry_at": null,
    "last_status_code": 200,
    "success_rate_24h": 99.5,
    "total_requests_24h": 1500,
    "avg_response_time_ms": 245.3
  }
}
```

### 状态说明

- **healthy**: 提供商完全正常
- **degraded**: 提供商部分可用（成功率低或有错误）
- **unhealthy**: 提供商不可用（连续错误超过阈值）
- **unknown**: 状态未知（未进行检查）

---

## 数据库架构

### 新增表

```sql
-- 提供商验证状态表
CREATE TABLE provider_validations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER UNIQUE NOT NULL,
    is_healthy BOOLEAN DEFAULT 1,
    error_count INTEGER DEFAULT 0,
    last_error TEXT,
    last_status_code INTEGER,
    last_validated_at DATETIME,
    last_success_at DATETIME,
    next_retry_at DATETIME,
    consecutive_successes INTEGER DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);

-- 提供商使用统计表
CREATE TABLE provider_usage_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER NOT NULL,
    date DATE NOT NULL,
    total_requests INTEGER DEFAULT 0,
    success_requests INTEGER DEFAULT 0,
    failed_requests INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    avg_response_time REAL DEFAULT 0,
    last_used_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    UNIQUE(provider_id, date)
);

-- 健康检查配置表
CREATE TABLE health_check_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    enabled BOOLEAN DEFAULT 1,
    interval_minutes INTEGER DEFAULT 5,
    max_error_count INTEGER DEFAULT 5,
    retry_after_hours INTEGER DEFAULT 1,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);
```

---

## API 端点

### 健康检查相关

```
GET    /api/providers/health          # 获取所有提供商健康状态
GET    /api/providers/health/:id      # 获取单个提供商健康状态
POST   /api/health-check/force/:id    # 强制检查指定提供商
GET    /api/health-check/config       # 获取健康检查配置
PUT    /api/health-check/config       # 更新健康检查配置
```

### 使用示例

```bash
# 查看所有提供商健康状态
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:7070/api/providers/health

# 强制检查提供商
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:7070/api/health-check/force/1
🔄 Prometheus metrics 导出
- 🔄 告警通知系统
- 🔄 提供商分组管理
- 🔄 自定义健康检查规则

---

## 许可证

本项目采用 MIT 许可证。

---

## 支持

如有问题或建议，请提交 Issue 或 Pull Request。

**项目地址**: https://github.com/atopos31/llmio

**文档更新日期**: 2024-01-15