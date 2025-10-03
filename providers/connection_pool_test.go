package providers

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestConnectionPool(t *testing.T) {
	// 创建测试连接池
	pool := NewConnectionPool(10, 5, 1*time.Minute, 5*time.Second, 30*time.Second)

	// 测试获取客户端
	ctx := context.Background()
	host := "http://example.com"
	timeout := 10 * time.Second

	client1, err := pool.GetClient(ctx, host, timeout)
	if err != nil {
		t.Fatalf("Failed to get client: %v", err)
	}

	// 测试归还客户端
	pool.ReturnClient(host, client1)

	// 测试连接复用
	client2, err := pool.GetClient(ctx, host, timeout)
	if err != nil {
		t.Fatalf("Failed to get client: %v", err)
	}
	pool.ReturnClient(host, client2)

	// 验证连接池统计
	stats := pool.GetStats()
	if stats.TotalHosts != 1 {
		t.Errorf("Expected 1 host, got %d", stats.TotalHosts)
	}

	// 清理连接池
	pool.Cleanup()

	t.Log("Connection pool test passed")
}

func TestConnectionPoolLimits(t *testing.T) {
	// 创建限制较小的连接池
	pool := NewConnectionPool(2, 1, 1*time.Minute, 5*time.Second, 30*time.Second)

	ctx := context.Background()
	host := "http://example.com"
	timeout := 10 * time.Second

	// 获取第一个客户端
	client1, err := pool.GetClient(ctx, host, timeout)
	if err != nil {
		t.Fatalf("Failed to get client1: %v", err)
	}

	// 获取第二个客户端
	client2, err := pool.GetClient(ctx, host, timeout)
	if err != nil {
		t.Fatalf("Failed to get client2: %v", err)
	}

	// 尝试获取第三个客户端（应该失败）
	_, err = pool.GetClient(ctx, host, timeout)
	if err == nil {
		t.Error("Expected connection limit error, but got none")
	}

	// 归还一个客户端
	pool.ReturnClient(host, client1)

	// 现在应该可以获取第三个客户端
	client3, err := pool.GetClient(ctx, host, timeout)
	if err != nil {
		t.Fatalf("Failed to get client3 after returning client1: %v", err)
	}

	// 清理
	pool.ReturnClient(host, client2)
	pool.ReturnClient(host, client3)
	pool.Cleanup()

	t.Log("Connection pool limits test passed")
}

func TestGlobalConnectionPool(t *testing.T) {
	ctx := context.Background()
	host := "http://test.com"
	timeout := 5 * time.Second

	// 测试全局连接池
	client1, err := GetPooledClient(ctx, host, timeout)
	if err != nil {
		t.Fatalf("Failed to get client from global pool: %v", err)
	}

	// 归还客户端
	ReturnPooledClient(host, client1)

	// 获取统计信息
	stats := GetPoolStats()
	if stats.TotalHosts < 0 {
		t.Error("Invalid stats from global pool")
	}

	t.Log("Global connection pool test passed")
}

func TestPooledProviderWrapper(t *testing.T) {
	// 创建测试Provider
	testProvider := &testProvider{
		host:    "http://test.com",
		timeout: 10 * time.Second,
	}

	// 创建包装器
	wrapper := NewPooledProviderWrapper(testProvider, testProvider.host, testProvider.timeout)

	// 测试GetHost方法
	if wrapper.GetHost() != testProvider.host {
		t.Errorf("Expected host %s, got %s", testProvider.host, wrapper.GetHost())
	}

	// 测试GetTimeout方法
	if wrapper.GetTimeout() != testProvider.timeout {
		t.Errorf("Expected timeout %v, got %v", testProvider.timeout, wrapper.GetTimeout())
	}

	t.Log("Pooled provider wrapper test passed")
}

// 测试用的Provider实现
type testProvider struct {
	host    string
	timeout time.Duration
}

func (t *testProvider) Chat(ctx context.Context, client *http.Client, model string, rawBody []byte) (*http.Response, error) {
	return nil, nil
}

func (t *testProvider) Models(ctx context.Context) ([]Model, error) {
	return nil, nil
}

func (t *testProvider) GetHost() string {
	return t.host
}

func (t *testProvider) GetTimeout() time.Duration {
	return t.timeout
}