package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/atopos31/llmio/handler"
	"github.com/atopos31/llmio/middleware"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/crypto/x509roots/fallback"
)

func init() {
	models.Init("./db/llmio.db")
	slog.Info("TZ", "time.Local", time.Local.String())
}

func main() {
	// 启动健康检查服务
	healthCheckService := service.NewHealthCheckService(models.DB)
	if err := healthCheckService.Start(); err != nil {
		slog.Error("Failed to start health check service", "error", err)
	}
	
	// 设置优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("Shutting down health check service...")
		healthCheckService.Stop()
		os.Exit(0)
	}()
	
	router := gin.Default()
	
	// 添加统一错误处理中间件
	router.Use(middleware.RequestID()) // 请求ID中间件
	router.Use(middleware.Recovery())   // 恢复中间件
	router.Use(middleware.ErrorHandler()) // 错误处理中间件
	
	setwebui(router, "./webui/dist")

	authOpenAi := middleware.Auth(os.Getenv("TOKEN"))
	authAnthropic := middleware.AuthAnthropic(os.Getenv("TOKEN"))

	v1 := router.Group("/v1")
	v1.GET("/models", authOpenAi, handler.ModelsHandler)

	v1.POST("/chat/completions", authOpenAi, handler.ChatCompletionsHandler)
	v1.POST("/messages", authAnthropic, handler.Messages)

	api := router.Group("/api")
	api.Use(middleware.Auth(os.Getenv("TOKEN")))
	api.GET("/metrics/use/:days", handler.Metrics)
	api.GET("/metrics/counts", handler.Counts)
	// Provider management
	api.GET("/providers/template", handler.GetProviderTemplates)
	api.GET("/providers", handler.GetProviders)
	api.GET("/providers/models/:id", handler.GetProviderModels)
	api.POST("/providers", handler.CreateProvider)
	api.PUT("/providers/:id", handler.UpdateProvider)
	api.DELETE("/providers/:id", handler.DeleteProvider)

	// Model management
	api.GET("/models", handler.GetModels)
	api.POST("/models", handler.CreateModel)
	api.PUT("/models/:id", handler.UpdateModel)
	api.DELETE("/models/:id", handler.DeleteModel)

	// Model-provider association management
	api.GET("/model-providers", handler.GetModelProviders)
	api.GET("/model-providers/status", handler.GetModelProviderStatus)
	api.POST("/model-providers", handler.CreateModelProvider)
	api.PUT("/model-providers/:id", handler.UpdateModelProvider)
	api.DELETE("/model-providers/:id", handler.DeleteModelProvider)

	// System status and monitoring
	api.GET("/logs", handler.GetRequestLogs)
	api.GET("/logs/export", handler.ExportLogs)
	
	// Dashboard and statistics
	api.GET("/dashboard/stats", handler.GetDashboardStats)
	api.GET("/dashboard/realtime", handler.GetRealtimeStats)
	
	// Provider health checks
	api.GET("/providers/health", handler.GetAllProvidersHealth)
	api.GET("/providers/health/:id", handler.GetProviderHealth)
	
	// Batch operations
	api.POST("/providers/batch-delete", handler.BatchDeleteProviders)
	api.POST("/models/batch-delete", handler.BatchDeleteModels)
	
	// Configuration validation, import and export
	api.POST("/providers/validate", handler.ValidateProviderConfig)
	api.GET("/config/export", handler.ExportConfig)
	api.POST("/config/import", handler.ImportConfig)
	
	// Log management
	api.DELETE("/logs/clear", handler.ClearLogs)

	// System configuration
	api.GET("/config", handler.GetSystemConfig)
	api.PUT("/config", handler.UpdateSystemConfig)
	
	// Health check configuration
	api.GET("/health-check/config", handler.GetHealthCheckConfig)
	api.PUT("/health-check/config", handler.UpdateHealthCheckConfig)
	api.POST("/health-check/force/:id", handler.ForceHealthCheck)

	// Provider connectivity test
	api.GET("/test/:id", handler.ProviderTestHandler)
	api.GET("/test/react/:id", handler.TestReactHandler)

	router.Run(":7070")
}

func setwebui(r *gin.Engine, path string) {
	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/v1/"})))
	r.Use(static.Serve("/", static.LocalFile(path, false)))

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && !strings.HasPrefix(c.Request.URL.Path, "/api/") && !strings.HasPrefix(c.Request.URL.Path, "/v1/") {
			data, err := os.ReadFile(path + "/index.html")
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err) //nolint:errcheck
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		} else {
			c.Status(http.StatusNotFound)
		}
	})
}
