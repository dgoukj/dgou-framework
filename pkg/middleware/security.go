package middleware

import (
	"github.com/gin-gonic/gin"
)

// Security 安全中间件
func Security() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置安全相关的HTTP头
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")

		// 如果是HTTPS，添加HSTS头
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
