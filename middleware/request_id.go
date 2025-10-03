package middleware

import (
	"crypto/rand"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
)

// RequestID 请求ID中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头中获取请求ID，如果没有则生成新的
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// 设置请求ID到上下文
		c.Set("request_id", requestID)

		// 设置请求ID到响应头
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// generateRequestID 生成唯一的请求ID
func generateRequestID() string {
	b := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		// 如果随机数生成失败，使用时间戳作为备选方案
		return fmt.Sprintf("%x", b)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// GetRequestID 从上下文中获取请求ID
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}