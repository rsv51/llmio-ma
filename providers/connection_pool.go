package providers

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// ConnectionPool 连接池管理器
type ConnectionPool struct {
	mu                sync.RWMutex
	pools             map[string]*HostPool // 主机地址 -> 连接池
	maxConnsPerHost   int                  // 每个主机的最大连接数
	maxIdleConns      int                  // 最大空闲连接数
	idleTimeout       time.Duration       // 空闲连接超时时间
	dialTimeout       time.Duration       // 连接建立超时时间
	keepAlive         time.Duration       // 连接保活时间
	maxConnLifetime   time.Duration       // 连接最大生命周期
	healthCheckInterval time.Duration     // 健康检查间隔
	stopHealthCheck   chan struct{}       // 停止健康检查信号
}

// HostPool 主机级别的连接池
type HostPool struct {
	mu          sync.RWMutex
	activeConns int                    // 活跃连接数
	idleConns   chan *http.Client      // 空闲连接队列
	inUse       map[*http.Client]bool  // 使用中的连接
	maxConns    int                    // 最大连接数
	createdAt   time.Time              // 连接池创建时间
	lastCheck   time.Time              // 最后健康检查时间
	connInfo    map[*http.Client]*ConnectionInfo // 连接详细信息
}

// ConnectionInfo 连接信息，用于监控连接使用情况
type ConnectionInfo struct {
	Client      *http.Client
	CreatedAt   time.Time
	LastUsedAt  time.Time
	UseCount    int64
	IsHealthy   bool
}

// PoolStats 连接池统计信息
type PoolStats struct {
	TotalHosts          int           `json:"total_hosts"`
	TotalActive         int           `json:"total_active"`
	TotalIdle           int           `json:"total_idle"`
	MaxConnsPerHost     int           `json:"max_conns_per_host"`
	TotalConnections    int           `json:"total_connections"`
	LeakedConnections   int           `json:"leaked_connections"`
	HealthCheckCount    int64         `json:"health_check_count"`
	RecycledConnections int64         `json:"recycled_connections"`
	Uptime              time.Duration `json:"uptime"`
}

// NewConnectionPool 创建新的连接池管理器
func NewConnectionPool(maxConnsPerHost, maxIdleConns int, idleTimeout, dialTimeout, keepAlive time.Duration) *ConnectionPool {
	cp := &ConnectionPool{
		pools:              make(map[string]*HostPool),
		maxConnsPerHost:    maxConnsPerHost,
		maxIdleConns:       maxIdleConns,
		idleTimeout:        idleTimeout,
		dialTimeout:        dialTimeout,
		keepAlive:          keepAlive,
		maxConnLifetime:    30 * time.Minute,    // 默认30分钟连接生命周期
		healthCheckInterval: 1 * time.Minute,     // 默认1分钟健康检查间隔
		stopHealthCheck:    make(chan struct{}),
	}
	
	// 启动健康检查协程
	go cp.startHealthCheck()
	
	return cp
}

// GetClient 获取HTTP客户端，支持连接复用
func (cp *ConnectionPool) GetClient(ctx context.Context, host string, timeout time.Duration) (*http.Client, error) {
	cp.mu.RLock()
	pool, exists := cp.pools[host]
	cp.mu.RUnlock()

	if !exists {
		cp.mu.Lock()
		// 双重检查
		if pool, exists = cp.pools[host]; !exists {
			pool = cp.createHostPool(host)
			cp.pools[host] = pool
		}
		cp.mu.Unlock()
	}

	return pool.getClient(ctx, timeout)
}

// ReturnClient 归还HTTP客户端到连接池
func (cp *ConnectionPool) ReturnClient(host string, client *http.Client) {
	cp.mu.RLock()
	pool, exists := cp.pools[host]
	cp.mu.RUnlock()

	if exists {
		pool.returnClient(client)
	}
}

// createHostPool 为主机创建连接池
func (cp *ConnectionPool) createHostPool(host string) *HostPool {
	return &HostPool{
		idleConns: make(chan *http.Client, cp.maxIdleConns),
		inUse:     make(map[*http.Client]bool),
		connInfo:  make(map[*http.Client]*ConnectionInfo),
		maxConns:  cp.maxConnsPerHost,
		createdAt: time.Now(),
		lastCheck: time.Now(),
	}
}

// getClient 从主机连接池获取客户端
func (hp *HostPool) getClient(ctx context.Context, timeout time.Duration) (*http.Client, error) {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	// 尝试从空闲队列获取连接
	select {
	case client := <-hp.idleConns:
		if hp.isClientHealthy(client) {
			hp.inUse[client] = true
			hp.activeConns++
			
			// 更新连接使用信息
			if info, exists := hp.connInfo[client]; exists {
				info.LastUsedAt = time.Now()
				info.UseCount++
			}
			
			slog.Debug("reusing idle connection", "host", getHostFromClient(client))
			return client, nil
		}
		// 不健康的连接，关闭并继续
		client.CloseIdleConnections()
		delete(hp.connInfo, client)
		hp.activeConns--
	default:
		// 没有空闲连接
	}

	// 检查是否达到最大连接数限制
	if hp.activeConns >= hp.maxConns {
		return nil, fmt.Errorf("connection limit reached for host: %d/%d", hp.activeConns, hp.maxConns)
	}

	// 创建新连接
	client, err := hp.createNewClient(timeout)
	if err != nil {
		return nil, err
	}

	hp.inUse[client] = true
	hp.activeConns++
	
	// 记录新连接信息
	hp.connInfo[client] = &ConnectionInfo{
		Client:     client,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		UseCount:   1,
		IsHealthy:  true,
	}
	
	slog.Debug("created new connection", "host", getHostFromClient(client), "active", hp.activeConns)

	return client, nil
}

// returnClient 归还客户端到连接池
func (hp *HostPool) returnClient(client *http.Client) {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	if !hp.inUse[client] {
		// 连接不在使用中，直接关闭
		client.CloseIdleConnections()
		delete(hp.connInfo, client)
		return
	}

	delete(hp.inUse, client)
	hp.activeConns--

	// 更新连接最后使用时间
	if info, exists := hp.connInfo[client]; exists {
		info.LastUsedAt = time.Now()
	}

	// 检查连接是否健康
	if !hp.isClientHealthy(client) {
		client.CloseIdleConnections()
		delete(hp.connInfo, client)
		slog.Debug("closing unhealthy connection")
		return
	}

	// 检查连接是否超过最大生命周期
	if info, exists := hp.connInfo[client]; exists {
		if time.Since(info.CreatedAt) > 30*time.Minute { // 默认30分钟生命周期
			client.CloseIdleConnections()
			delete(hp.connInfo, client)
			slog.Debug("closing expired connection", "lifetime", time.Since(info.CreatedAt))
			return
		}
	}

	// 尝试放回空闲队列
	select {
	case hp.idleConns <- client:
		slog.Debug("returned connection to idle pool", "idle_count", len(hp.idleConns))
	default:
		// 空闲队列已满，关闭连接
		client.CloseIdleConnections()
		delete(hp.connInfo, client)
		slog.Debug("idle pool full, closing connection")
	}
}

// isClientHealthy 检查客户端是否健康
func (hp *HostPool) isClientHealthy(client *http.Client) bool {
	// 检查Transport是否有效
	transport := client.Transport
	if transport == nil {
		return false
	}
	
	// 检查HTTP Transport是否有效
	if httpTransport, ok := transport.(*http.Transport); ok {
		// 检查连接是否已关闭
		if httpTransport.DisableKeepAlives {
			return false
		}
		
		// 可以添加更多健康检查逻辑
		// 例如：检查连接池状态、空闲连接数等
	}
	
	return true
}

// createNewClient 创建新的HTTP客户端
func (hp *HostPool) createNewClient(timeout time.Duration) (*http.Client, error) {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: timeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   0, // 使用ResponseHeaderTimeout控制超时
	}, nil
}

// getHostFromClient 从客户端获取主机信息（简化实现）
func getHostFromClient(client *http.Client) string {
	// 这里简化实现，实际中可能需要更复杂的逻辑来获取主机信息
	return "unknown"
}

// startHealthCheck 启动健康检查协程
func (cp *ConnectionPool) startHealthCheck() {
	ticker := time.NewTicker(cp.healthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cp.performHealthCheck()
		case <-cp.stopHealthCheck:
			slog.Info("health check stopped")
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (cp *ConnectionPool) performHealthCheck() {
	cp.mu.RLock()
	hostPools := make(map[string]*HostPool)
	for host, pool := range cp.pools {
		hostPools[host] = pool
	}
	cp.mu.RUnlock()

	totalLeaked := 0
	totalRecycled := 0
	
	for host, hp := range hostPools {
		hp.mu.Lock()
		
		// 检查空闲连接
		idleCount := len(hp.idleConns)
		recycledCount := 0
		for i := 0; i < idleCount; i++ {
			select {
			case client := <-hp.idleConns:
				if hp.isClientHealthy(client) {
					// 检查连接是否超过最大生命周期
					if info, exists := hp.connInfo[client]; exists {
						if time.Since(info.CreatedAt) > cp.maxConnLifetime {
							client.CloseIdleConnections()
							delete(hp.connInfo, client)
							hp.activeConns--
							recycledCount++
							continue
						}
					}
					hp.idleConns <- client
				} else {
					client.CloseIdleConnections()
					delete(hp.connInfo, client)
					hp.activeConns--
					recycledCount++
				}
			default:
				break
			}
		}
		
		// 检查使用中的连接是否泄漏（长时间未归还）
		leakedCount := 0
		for client := range hp.inUse {
			if info, exists := hp.connInfo[client]; exists {
				if time.Since(info.LastUsedAt) > 5*time.Minute { // 5分钟未归还视为泄漏
					leakedCount++
					// 强制关闭泄漏连接
					client.CloseIdleConnections()
					delete(hp.inUse, client)
					delete(hp.connInfo, client)
					hp.activeConns--
				}
			}
		}
		
		if leakedCount > 0 {
			slog.Warn("detected leaked connections", "host", host, "count", leakedCount)
			totalLeaked += leakedCount
		}
		
		if recycledCount > 0 {
			slog.Info("recycled expired connections", "host", host, "count", recycledCount)
			totalRecycled += recycledCount
		}
		
		hp.lastCheck = time.Now()
		hp.mu.Unlock()
	}
	
	// 只在有异常情况时记录日志
	if totalLeaked > 0 || totalRecycled > 0 {
		slog.Info("health check completed",
			"checked_hosts", len(hostPools),
			"recycled_connections", totalRecycled,
			"leaked_connections", totalLeaked)
	}
}

// isConnectionHealthy 检查连接是否健康
func (cp *ConnectionPool) isConnectionHealthy(client *http.Client, host string) bool {
	// 实现更复杂的健康检查逻辑
	// 1. 检查连接是否超时
	// 2. 检查连接是否可重用
	// 3. 检查连接状态
	
	// 简化实现：检查Transport是否有效
	transport := client.Transport
	if transport == nil {
		return false
	}
	
	// 检查HTTP Transport是否有效
	if _, ok := transport.(*http.Transport); !ok {
		return false
	}
	
	return true
}

// checkConnectionLeak 检查连接泄漏
func (cp *ConnectionPool) checkConnectionLeak() int {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	leakedConnections := 0
	
	for _, pool := range cp.pools {
		pool.mu.RLock()
		// 检查是否有长时间未归还的连接
		// 这里可以添加更复杂的泄漏检测逻辑
		if pool.activeConns > 0 {
			// 简单的泄漏检测：活跃连接数大于0但长时间没有变化
			// 实际中应该记录连接获取时间并进行超时检查
			leakedConnections += pool.activeConns
		}
		pool.mu.RUnlock()
	}
	
	return leakedConnections
}

// GetStats 获取连接池统计信息
func (cp *ConnectionPool) GetStats() PoolStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := PoolStats{
		TotalHosts:          len(cp.pools),
		MaxConnsPerHost:     cp.maxConnsPerHost,
		LeakedConnections:   cp.checkConnectionLeak(),
		HealthCheckCount:    int64(len(cp.pools)), // 简化实现
		RecycledConnections: 0,                    // 实际中应该记录回收的连接数
	}

	totalConnections := 0
	for _, pool := range cp.pools {
		pool.mu.RLock()
		stats.TotalActive += pool.activeConns
		stats.TotalIdle += len(pool.idleConns)
		totalConnections += pool.activeConns + len(pool.idleConns)
		
		// 计算连接池运行时间
		if pool.createdAt.After(time.Time{}) {
			if stats.Uptime == 0 || time.Since(pool.createdAt) < stats.Uptime {
				stats.Uptime = time.Since(pool.createdAt)
			}
		}
		pool.mu.RUnlock()
	}
	
	stats.TotalConnections = totalConnections

	return stats
}

// Cleanup 清理连接池，关闭所有连接
func (cp *ConnectionPool) Cleanup() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// 停止健康检查协程
	if cp.stopHealthCheck != nil {
		close(cp.stopHealthCheck)
	}

	for host, pool := range cp.pools {
		pool.cleanup()
		delete(cp.pools, host)
	}

	slog.Info("connection pool cleanup completed")
}

// cleanup 清理主机连接池
func (hp *HostPool) cleanup() {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	// 关闭所有空闲连接
	close(hp.idleConns)
	for client := range hp.idleConns {
		client.CloseIdleConnections()
	}

	// 关闭所有使用中的连接（在实际使用中应该等待连接归还）
	for client := range hp.inUse {
		client.CloseIdleConnections()
	}

	hp.activeConns = 0
	hp.inUse = make(map[*http.Client]bool)
	hp.idleConns = make(chan *http.Client, cap(hp.idleConns))
}

// GlobalConnectionPool 全局连接池实例
var GlobalConnectionPool = NewConnectionPool(
	100,              // maxConnsPerHost
	50,               // maxIdleConns
	5*time.Minute,    // idleTimeout
	30*time.Second,   // dialTimeout
	30*time.Second,   // keepAlive
)

// GetPooledClient 获取带连接池的HTTP客户端
func GetPooledClient(ctx context.Context, host string, timeout time.Duration) (*http.Client, error) {
	return GlobalConnectionPool.GetClient(ctx, host, timeout)
}

// ReturnPooledClient 归还有连接池的HTTP客户端
func ReturnPooledClient(host string, client *http.Client) {
	GlobalConnectionPool.ReturnClient(host, client)
}

// GetPoolStats 获取全局连接池统计信息
func GetPoolStats() PoolStats {
	return GlobalConnectionPool.GetStats()
}

// CleanupPool 清理全局连接池
func CleanupPool() {
	GlobalConnectionPool.Cleanup()
}