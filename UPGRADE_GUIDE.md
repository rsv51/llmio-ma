# LLMIO 增强版升级指南

本指南帮助您快速了解和使用从 OrchestrationApi 借鉴的新功能。

## 🚀 新功能概览

### 1. 自动健康检查系统 ⭐⭐⭐
- ✅ 后台每5分钟自动检查所有提供商
- ✅ 自动恢复不健康的提供商
- ✅ 详细的错误追踪和状态码记录

### 2. 智能故障转移 ⭐⭐⭐
- ✅ 自动排除不健康的提供商
- ✅ 多层级降级策略
- ✅ 实时更新健康状态

### 3. 持久化统计 ⭐⭐
- ✅ 使用数据库永久保存统计数据
- ✅ 按日期聚合使用情况
- ✅ 支持历史数据分析

## 📦 快速开始

### 升级步骤

```bash
# 1. 备份数据库
cp ./db/llmio.db ./db/llmio.db.backup

# 2. 更新代码
git pull origin main
go mod tidy

# 3. 运行（自动迁移）
go run main.go
```

### 验证安装

```bash
# 检查健康检查服务
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:7070/api/providers/health

# 查看配置
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:7070/api/health-check/config
```

## 🎯 核心API

### 健康检查

```bash
# 获取所有提供商健康状态
GET /api/providers/health

# 获取单个提供商健康状态  
GET /api/providers/health/:id

# 强制检查指定提供商
POST /api/health-check/force/:id
```

### 配置管理

```bash
# 获取健康检查配置
GET /api/health-check/config

# 更新配置
PUT /api/health-check/config
{
  "enabled": true,
  "interval_minutes": 5,
  "max_error_count": 5,
  "retry_after_hours": 1
}
```

## 💡 使用示例

### 查看健康状态

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:7070/api/providers/health
```

响应示例：
```json
{
  "code": 200,
  "data": [
    {
      "provider_id": 1,
      "provider_name": "openai-primary",
      "status": "healthy",
      "is_healthy": true,
      "error_count": 0,
      "consecutive_successes": 125,
      "success_rate_24h": 99.5,
      "total_requests_24h": 1500
    }
  ]
}
```

### 调整检查间隔

```bash
# 改为每10分钟检查一次
curl -X PUT \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "interval_minutes": 10, "max_error_count": 5, "retry_after_hours": 1}' \
  http://localhost:7070/api/health-check/config
```

## 📊 新增数据表

自动创建以下表：

- `provider_validations` - 提供商健康状态
- `provider_usage_stats` - 使用统计数据
- `health_check_configs` - 健康检查配置

## ⚙️ 配置说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| enabled | true | 是否启用健康检查 |
| interval_minutes | 5 | 检查间隔（分钟） |
| max_error_count | 5 | 标记为不健康前的最大错误次数 |
| retry_after_hours | 1 | 不健康后多久重试（小时） |

## 🔧 故障排查

### 问题：健康检查未运行

**检查日志**：
```
INFO Health check service started
```

**解决方案**：
1. 确认配置 `enabled: true`
2. 检查数据库连接
3. 查看错误日志

### 问题：提供商被误判为不健康

**解决方案**：
1. 调高 `max_error_count`
2. 查看 `last_error` 字段
3. 手动强制检查验证

## 📚 完整文档

详细功能说明和最佳实践请参考：
- [完整功能文档](./ORCHESTRATION_ENHANCEMENTS.md)
- [数据库迁移脚本](./scripts/migrate_to_enhanced.sql)

## 🎉 主要改进

相比 OrchestrationApi 的优势：

1. **更轻量**：使用 SQLite，无需额外数据库
2. **更灵活**：配置存储在数据库，支持动态调整
3. **更真实**：使用实际 API 调用验证健康状态
4. **更高效**：Go 并发处理，资源占用更少

## ⚠️ 注意事项

1. 首次运行会自动创建新表和索引
2. 建议在生产环境前先在测试环境验证
3. 定期备份数据库（特别是在升级前）
4. 监控健康检查服务的资源使用

## 🆘 需要帮助？

- 查看 [完整文档](./ORCHESTRATION_ENHANCEMENTS.md)
- 提交 [Issue](https://github.com/atopos31/llmio/issues)
- 参考 [最佳实践](./ORCHESTRATION_ENHANCEMENTS.md#最佳实践)

---

**版本**: v2.0.0  
**更新日期**: 2024-01-15