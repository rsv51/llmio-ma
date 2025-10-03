package providers

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type clientCache struct {
	mu      sync.RWMutex
	clients map[time.Duration]*http.Client
}

var cache = &clientCache{
	clients: make(map[time.Duration]*http.Client),
}

var dialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

// GetClient returns an http.Client with the specified responseHeaderTimeout.
// If a client with the same timeout already exists, it returns the cached one.
// Otherwise, it creates a new client and caches it.
func GetClient(responseHeaderTimeout time.Duration) *http.Client {
	cache.mu.RLock()
	if client, exists := cache.clients[responseHeaderTimeout]; exists {
		cache.mu.RUnlock()
		return client
	}
	cache.mu.RUnlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := cache.clients[responseHeaderTimeout]; exists {
		return client
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0, // No overall timeout, let ResponseHeaderTimeout control header timing
	}

	cache.clients[responseHeaderTimeout] = client
	return client
}

// GetPooledClientForProvider 为Provider获取带连接池的HTTP客户端
func GetPooledClientForProvider(ctx context.Context, provider PooledProvider) (*http.Client, error) {
	if provider == nil {
		return nil, nil
	}

	host := provider.GetHost()
	timeout := provider.GetTimeout()

	// 使用全局连接池获取客户端
	client, err := GetPooledClient(ctx, host, timeout)
	if err != nil {
		// 如果连接池获取失败，回退到传统方式
		slog.Debug("connection pool failed, falling back to traditional client", "error", err)
		return GetClient(timeout), nil
	}

	return client, nil
}

// ReturnPooledClientForProvider 归还Provider使用的HTTP客户端到连接池
func ReturnPooledClientForProvider(provider PooledProvider, client *http.Client) {
	if provider == nil || client == nil {
		return
	}

	host := provider.GetHost()
	ReturnPooledClient(host, client)
}
