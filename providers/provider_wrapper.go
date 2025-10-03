package providers

import (
	"context"
	"net/http"
	"time"
)

// PooledProviderWrapper 包装器，为现有Provider添加连接池支持
type PooledProviderWrapper struct {
	provider Provider
	host     string
	timeout  time.Duration
}

// NewPooledProviderWrapper 创建新的连接池包装器
func NewPooledProviderWrapper(provider Provider, host string, timeout time.Duration) *PooledProviderWrapper {
	return &PooledProviderWrapper{
		provider: provider,
		host:     host,
		timeout:  timeout,
	}
}

// Chat 使用连接池执行Chat请求
func (w *PooledProviderWrapper) Chat(ctx context.Context, client *http.Client, model string, rawBody []byte) (*http.Response, error) {
	// 如果提供了client，直接使用（向后兼容）
	if client != nil {
		return w.provider.Chat(ctx, client, model, rawBody)
	}

	// 使用连接池获取客户端
	pooledClient, err := GetPooledClientForProvider(ctx, w)
	if err != nil {
		return nil, err
	}

	// 执行请求
	resp, err := w.provider.Chat(ctx, pooledClient, model, rawBody)

	// 归还连接到池中
	if pooledClient != nil {
		ReturnPooledClientForProvider(w, pooledClient)
	}

	return resp, err
}

// Models 获取模型列表
func (w *PooledProviderWrapper) Models(ctx context.Context) ([]Model, error) {
	return w.provider.Models(ctx)
}

// GetHost 获取主机地址
func (w *PooledProviderWrapper) GetHost() string {
	return w.host
}

// GetTimeout 获取超时时间
func (w *PooledProviderWrapper) GetTimeout() time.Duration {
	return w.timeout
}

// GetUnderlyingProvider 获取底层Provider
func (w *PooledProviderWrapper) GetUnderlyingProvider() Provider {
	return w.provider
}

// PooledChat 便捷函数：使用连接池执行Chat请求
func PooledChat(ctx context.Context, provider PooledProvider, model string, rawBody []byte) (*http.Response, error) {
	if provider == nil {
		return nil, nil
	}

	// 使用连接池获取客户端
	client, err := GetPooledClientForProvider(ctx, provider)
	if err != nil {
		return nil, err
	}

	// 执行请求
	resp, err := provider.Chat(ctx, client, model, rawBody)

	// 归还连接到池中
	if client != nil {
		ReturnPooledClientForProvider(provider, client)
	}

	return resp, err
}

// PooledModels 便捷函数：使用连接池获取模型列表
func PooledModels(ctx context.Context, provider PooledProvider) ([]Model, error) {
	if provider == nil {
		return nil, nil
	}

	return provider.Models(ctx)
}