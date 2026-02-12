// internal/middleware/request_id.go
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDKey 请求ID在上下文中的键
	RequestIDKey = "request_id"
)

// RequestID 请求ID中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// GetRequestID 从上下文中获取请求ID
func GetRequestID(c *gin.Context) string {
	if value, exists := c.Get(RequestIDKey); exists {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

// SetRequestID 设置请求ID到上下文（用于测试或手动设置）
func SetRequestID(c *gin.Context, requestID string) {
	c.Set(RequestIDKey, requestID)
	c.Header("X-Request-ID", requestID)
}
