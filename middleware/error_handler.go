package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/gin-gonic/gin"
)

// ErrorCode 定义标准错误码
type ErrorCode int

const (
	// 成功
	Success ErrorCode = 0

	// 通用错误 (1000-1999)
	ErrorInternalServer     ErrorCode = 1000
	ErrorBadRequest        ErrorCode = 1001
	ErrorUnauthorized      ErrorCode = 1002
	ErrorForbidden         ErrorCode = 1003
	ErrorNotFound          ErrorCode = 1004
	ErrorTimeout           ErrorCode = 1005
	ErrorValidation        ErrorCode = 1006

	// 业务错误 (2000-2999)
	ErrorProviderUnavailable ErrorCode = 2000
	ErrorModelNotFound      ErrorCode = 2001
	ErrorRateLimit          ErrorCode = 2002
	ErrorQuotaExceeded      ErrorCode = 2003
	ErrorInvalidRequest     ErrorCode = 2004

	// 系统错误 (3000-3999)
	ErrorDatabase          ErrorCode = 3000
	ErrorCache             ErrorCode = 3001
	ErrorExternalAPI       ErrorCode = 3002
	ErrorConfiguration     ErrorCode = 3003
)

// ErrorResponse 统一错误响应结构
type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
	Path      string `json:"path,omitempty"`
}

// ErrorMessage 错误码对应的消息映射
var ErrorMessage = map[ErrorCode]string{
	Success:                "success",
	ErrorInternalServer:    "Internal server error",
	ErrorBadRequest:       "Bad request",
	ErrorUnauthorized:     "Unauthorized",
	ErrorForbidden:        "Forbidden",
	ErrorNotFound:         "Resource not found",
	ErrorTimeout:          "Request timeout",
	ErrorValidation:       "Validation failed",
	ErrorProviderUnavailable: "Provider unavailable",
	ErrorModelNotFound:    "Model not found",
	ErrorRateLimit:        "Rate limit exceeded",
	ErrorQuotaExceeded:    "Quota exceeded",
	ErrorInvalidRequest:   "Invalid request",
	ErrorDatabase:         "Database error",
	ErrorCache:            "Cache error",
	ErrorExternalAPI:      "External API error",
	ErrorConfiguration:    "Configuration error",
}

// GetErrorMessage 获取错误消息
func GetErrorMessage(code ErrorCode) string {
	if msg, ok := ErrorMessage[code]; ok {
		return msg
	}
	return "Unknown error"
}

// ErrorHandler 统一错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			
			// 获取请求ID
			requestID := GetRequestID(c)
			
			// 根据错误类型返回相应的响应
			if isBadRequestError(err) {
				sendErrorResponse(c, 400, "请求参数错误", err.Error(), requestID)
			} else if isUnauthorizedError(err) {
				sendErrorResponse(c, 401, "未授权访问", err.Error(), requestID)
			} else if isForbiddenError(err) {
				sendErrorResponse(c, 403, "禁止访问", err.Error(), requestID)
			} else if isNotFoundError(err) {
				sendErrorResponse(c, 404, "资源未找到", err.Error(), requestID)
			} else if isValidationError(err) {
				sendErrorResponse(c, 422, "数据验证失败", err.Error(), requestID)
			} else {
				// 默认内部服务器错误
				sendErrorResponse(c, 500, "内部服务器错误", err.Error(), requestID)
			}
		}
	}
}

// Recovery 恢复中间件，处理panic
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录panic信息
				err := fmt.Errorf("panic recovered: %v", r)
				stack := string(debug.Stack())
				
				// 记录详细错误日志
				slog.Error("Panic recovered",
					"error", err,
					"stack", stack,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"ip", c.ClientIP(),
				)

				// 返回标准错误响应
				common.ErrorWithHttpStatus(c, http.StatusInternalServerError, int(ErrorInternalServer), "Internal server error")
				c.Abort()
			}
		}()

		c.Next()
	}
}

// handleError 处理错误
func handleError(c *gin.Context, err *gin.Error, start time.Time) {
	// 计算请求耗时
	duration := time.Since(start)
	
	// 获取请求ID
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = "unknown"
	}

	// 根据错误类型确定错误码和HTTP状态码
	var errorCode ErrorCode
	var httpStatus int
	var errorMsg string

	switch {
	case isBadRequestError(err.Err):
		errorCode = ErrorBadRequest
		httpStatus = http.StatusBadRequest
		errorMsg = GetErrorMessage(ErrorBadRequest)
	case isUnauthorizedError(err.Err):
		errorCode = ErrorUnauthorized
		httpStatus = http.StatusUnauthorized
		errorMsg = GetErrorMessage(ErrorUnauthorized)
	case isForbiddenError(err.Err):
		errorCode = ErrorForbidden
		httpStatus = http.StatusForbidden
		errorMsg = GetErrorMessage(ErrorForbidden)
	case isNotFoundError(err.Err):
		errorCode = ErrorNotFound
		httpStatus = http.StatusNotFound
		errorMsg = GetErrorMessage(ErrorNotFound)
	case isTimeoutError(err.Err):
		errorCode = ErrorTimeout
		httpStatus = http.StatusRequestTimeout
		errorMsg = GetErrorMessage(ErrorTimeout)
	default:
		errorCode = ErrorInternalServer
		httpStatus = http.StatusInternalServerError
		errorMsg = GetErrorMessage(ErrorInternalServer)
	}

	// 记录错误日志
	logError(c, err, errorCode, duration, requestID)

	// 返回标准错误响应
	response := ErrorResponse{
		Code:      int(errorCode),
		Message:   errorMsg,
		Error:     err.Error(),
		Timestamp: time.Now().Format(time.RFC3339),
		RequestID: requestID,
		Path:      c.Request.URL.Path,
	}

	c.JSON(httpStatus, response)
	c.Abort()
}

// logError 记录错误日志
func logError(c *gin.Context, err *gin.Error, code ErrorCode, duration time.Duration, requestID string) {
	logLevel := slog.LevelError
	
	// 根据错误码调整日志级别
	if code >= 2000 && code < 3000 {
		logLevel = slog.LevelWarn // 业务错误使用警告级别
	}

	slog.LogAttrs(c.Request.Context(), logLevel, "Request error",
		slog.String("request_id", requestID),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("ip", c.ClientIP()),
		slog.Int("error_code", int(code)),
		slog.String("error_message", err.Error()),
		slog.Duration("duration", duration),
	)
}

// 错误类型判断函数
func isBadRequestError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "bad request") || 
		strings.Contains(errStr, "invalid request") ||
		strings.Contains(errStr, "validation") ||
		strings.Contains(errStr, "Invalid input") ||
		GetErrorCode(err) == ErrorBadRequest ||
		GetErrorCode(err) == ErrorValidation
}

func isUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "unauthorized") || 
		strings.Contains(errStr, "invalid token") ||
		strings.Contains(errStr, "authorization") ||
		GetErrorCode(err) == ErrorUnauthorized
}

func isForbiddenError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "forbidden") ||
		GetErrorCode(err) == ErrorForbidden
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "not found") || 
		strings.Contains(errStr, "Resource not found") ||
		GetErrorCode(err) == ErrorNotFound
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "Request timeout") ||
		GetErrorCode(err) == ErrorTimeout
}

func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "validation") ||
		strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "required") ||
		GetErrorCode(err) == ErrorValidation
}

// Error 创建标准错误
func Error(code ErrorCode, message ...string) error {
	msg := GetErrorMessage(code)
	if len(message) > 0 {
		msg = message[0]
	}
	return fmt.Errorf("%d: %s", code, msg)
}

// WrapError 包装错误并添加错误码
func WrapError(code ErrorCode, err error, message ...string) error {
	msg := GetErrorMessage(code)
	if len(message) > 0 {
		msg = message[0]
	}
	return fmt.Errorf("%d: %s: %w", code, msg, err)
}

// IsErrorCode 检查错误是否包含特定错误码
func IsErrorCode(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}
	
	var errorCode int
	_, scanErr := fmt.Sscanf(err.Error(), "%d:", &errorCode)
	return scanErr == nil && ErrorCode(errorCode) == code
}

// GetErrorCode 从错误中提取错误码
func GetErrorCode(err error) ErrorCode {
	if err == nil {
		return Success
	}
	
	var errorCode int
	_, scanErr := fmt.Sscanf(err.Error(), "%d:", &errorCode)
	if scanErr != nil {
		return ErrorInternalServer
	}
	
	return ErrorCode(errorCode)
}

// sendErrorResponse 发送标准错误响应
func sendErrorResponse(c *gin.Context, httpStatus int, message string, errorDetail string, requestID string) {
	response := ErrorResponse{
		Code:      httpStatus,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now().Format(time.RFC3339),
		RequestID: requestID,
		Path:      c.Request.URL.Path,
	}
	
	// 记录错误日志
	slog.Error("Error response",
		"request_id", requestID,
		"status", httpStatus,
		"message", message,
		"error", errorDetail,
		"path", c.Request.URL.Path,
	)
	
	c.JSON(httpStatus, response)
	c.Abort()
}