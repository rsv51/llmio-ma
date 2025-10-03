package test_performance

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
)

func TestMain(m *testing.M) {
	// 确保db目录存在
	os.MkdirAll("../db", 0755)
	// 初始化数据库连接
	models.Init("../db/benchmark.db")
	
	// 运行测试
	code := m.Run()
	os.Exit(code)
}

// BenchmarkDatabaseQueries 测试数据库查询性能
func BenchmarkDatabaseQueries(b *testing.B) {
	ctx := context.Background()
	
	// 测试GetRequestLogs分页查询性能
	b.Run("GetRequestLogs_Pagination", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var logs []models.ChatLog
			err := models.DB.Model(&models.ChatLog{}).
				Order("created_at DESC").
				Limit(20).
				Find(&logs).Error
			if err != nil {
				b.Fatalf("GetRequestLogs query failed: %v", err)
			}
		}
	})

	// 测试ProvidersBymodelsNameDirect查询性能
	b.Run("ProvidersBymodelsNameDirect", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := service.ProvidersBymodelsNameDirect(ctx, "gpt-4")
			if err != nil {
				// 忽略"not found"错误，只关注性能
				if err.Error() != "not found model gpt-4" {
					b.Fatalf("ProvidersBymodelsNameDirect query failed: %v", err)
				}
			}
		}
	})

	// 测试缓存刷新性能
	b.Run("ConfigCache_Refresh", func(b *testing.B) {
		configCache := service.NewConfigCache(10 * time.Minute)
		for i := 0; i < b.N; i++ {
			// 使用refreshCache方法（小写开头，包内可见）
			// 由于refreshCache是私有方法，我们通过其他公开方法来触发缓存刷新
			_, _ = configCache.GetModel(ctx, "test-model")
		}
	})
}

// TestIndexPerformance 测试索引性能提升
func TestIndexPerformance(t *testing.T) {
	// ctx := context.Background() // 暂时注释掉未使用的变量
	
	// 测试ChatLogs表查询性能
	t.Run("ChatLogs_Query_Performance", func(t *testing.T) {
		start := time.Now()
		
		var logs []models.ChatLog
		err := models.DB.Model(&models.ChatLog{}).
			Where("provider_name = ?", "test-provider").
			Where("status = ?", "success").
			Order("created_at DESC").
			Limit(50).
			Find(&logs).Error
		
		if err != nil {
			t.Fatalf("ChatLogs query failed: %v", err)
		}
		
		duration := time.Since(start)
		t.Logf("ChatLogs query with indexes took: %v", duration)
		
		// 期望查询时间小于100ms
		if duration > 100*time.Millisecond {
			t.Errorf("ChatLogs query too slow: %v", duration)
		}
	})

	// 测试ModelWithProvider表查询性能
	t.Run("ModelWithProvider_Query_Performance", func(t *testing.T) {
		start := time.Now()
		
		var modelProviders []models.ModelWithProvider
		err := models.DB.Model(&models.ModelWithProvider{}).
			Where("model_id = ?", 1).
			Find(&modelProviders).Error
		
		if err != nil {
			t.Fatalf("ModelWithProvider query failed: %v", err)
		}
		
		duration := time.Since(start)
		t.Logf("ModelWithProvider query with indexes took: %v", duration)
		
		// 期望查询时间小于50ms
		if duration > 50*time.Millisecond {
			t.Errorf("ModelWithProvider query too slow: %v", duration)
		}
	})

	// 测试Provider表查询性能
	t.Run("Provider_Query_Performance", func(t *testing.T) {
		start := time.Now()
		
		var providers []models.Provider
		err := models.DB.Model(&models.Provider{}).
			Where("type = ?", "openai").
			Find(&providers).Error
		
		if err != nil {
			t.Fatalf("Provider query failed: %v", err)
		}
		
		duration := time.Since(start)
		t.Logf("Provider query with indexes took: %v", duration)
		
		// 期望查询时间小于30ms
		if duration > 30*time.Millisecond {
			t.Errorf("Provider query too slow: %v", duration)
		}
	})
}

// TestNPlusOneOptimization 测试N+1查询优化效果
func TestNPlusOneOptimization(t *testing.T) {
	ctx := context.Background()
	configCache := service.NewConfigCache(10 * time.Minute)
	
	// 测试缓存刷新性能（优化后的JOIN查询）
	start := time.Now()
	// 通过GetModel方法触发缓存刷新
	_, err := configCache.GetModel(ctx, "test-model")
	if err != nil {
		// 忽略"not found"错误，只关注性能
		t.Logf("GetModel error (expected): %v", err)
	}
	duration := time.Since(start)
	
	t.Logf("ConfigCache query with JOIN optimization took: %v", duration)
	
	// 期望查询时间小于500ms
	if duration > 500*time.Millisecond {
		t.Errorf("ConfigCache query too slow: %v", duration)
	}
}