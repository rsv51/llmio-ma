-- LLMIO Enhanced Features Migration Script
-- 此脚本用于将现有数据库升级到增强版本
-- 执行前请备份数据库！

-- 1. 创建 ProviderValidation 表（如果不存在）
CREATE TABLE IF NOT EXISTS provider_validations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    provider_id INTEGER NOT NULL UNIQUE,
    is_healthy BOOLEAN DEFAULT 1,
    error_count INTEGER DEFAULT 0,
    last_error TEXT,
    last_status_code INTEGER DEFAULT 0,
    last_validated_at DATETIME,
    last_success_at DATETIME,
    next_retry_at DATETIME,
    consecutive_successes INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_provider_validations_provider_id ON provider_validations(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_validations_last_validated_at ON provider_validations(last_validated_at);
CREATE INDEX IF NOT EXISTS idx_provider_validations_next_retry_at ON provider_validations(next_retry_at);
CREATE INDEX IF NOT EXISTS idx_provider_validations_deleted_at ON provider_validations(deleted_at);

-- 2. 创建 ProviderUsageStats 表（如果不存在）
CREATE TABLE IF NOT EXISTS provider_usage_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
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
    UNIQUE(provider_id, date)
);

CREATE INDEX IF NOT EXISTS idx_provider_usage_stats_provider_date ON provider_usage_stats(provider_id, date);
CREATE INDEX IF NOT EXISTS idx_provider_usage_stats_last_used_at ON provider_usage_stats(last_used_at);
CREATE INDEX IF NOT EXISTS idx_provider_usage_stats_deleted_at ON provider_usage_stats(deleted_at);

-- 3. 创建 HealthCheckConfig 表（如果不存在）
CREATE TABLE IF NOT EXISTS health_check_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    enabled BOOLEAN DEFAULT 1,
    interval_minutes INTEGER DEFAULT 5,
    max_error_count INTEGER DEFAULT 5,
    retry_after_hours INTEGER DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_health_check_configs_deleted_at ON health_check_configs(deleted_at);

-- 4. 插入默认健康检查配置（如果不存在）
INSERT OR IGNORE INTO health_check_configs (created_at, updated_at, enabled, interval_minutes, max_error_count, retry_after_hours)
VALUES (datetime('now'), datetime('now'), 1, 5, 5, 1);

-- 5. 为现有提供商初始化验证记录
INSERT OR IGNORE INTO provider_validations (
    created_at,
    updated_at,
    provider_id,
    is_healthy,
    error_count,
    last_validated_at,
    consecutive_successes
)
SELECT 
    datetime('now'),
    datetime('now'),
    id,
    1,
    0,
    datetime('now'),
    0
FROM providers
WHERE deleted_at IS NULL;

-- 6. 根据历史日志初始化使用统计（最近30天）
INSERT OR IGNORE INTO provider_usage_stats (
    created_at,
    updated_at,
    provider_id,
    date,
    total_requests,
    success_requests,
    failed_requests,
    total_tokens,
    prompt_tokens,
    completion_tokens,
    avg_response_time,
    last_used_at
)
SELECT
    datetime('now') as created_at,
    datetime('now') as updated_at,
    p.id as provider_id,
    DATE(cl.created_at) as date,
    COUNT(*) as total_requests,
    SUM(CASE WHEN cl.status = 'success' THEN 1 ELSE 0 END) as success_requests,
    SUM(CASE WHEN cl.status != 'success' THEN 1 ELSE 0 END) as failed_requests,
    COALESCE(SUM(cl.total_tokens), 0) as total_tokens,
    COALESCE(SUM(cl.prompt_tokens), 0) as prompt_tokens,
    COALESCE(SUM(cl.completion_tokens), 0) as completion_tokens,
    AVG(cl.proxy_time) as avg_response_time,
    MAX(cl.created_at) as last_used_at
FROM providers p
INNER JOIN chat_logs cl ON cl.provider_name = p.name
WHERE cl.created_at >= date('now', '-30 days')
    AND p.deleted_at IS NULL
    AND cl.deleted_at IS NULL
GROUP BY p.id, DATE(cl.created_at);

-- 7. 验证迁移结果
SELECT 'Migration completed successfully!' as status;
SELECT 'Provider validations count: ' || COUNT(*) FROM provider_validations;
SELECT 'Provider usage stats count: ' || COUNT(*) FROM provider_usage_stats;
SELECT 'Health check config count: ' || COUNT(*) FROM health_check_configs;