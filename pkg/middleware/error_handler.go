package middleware

import (
	"dgou/pkg/logger"
	"dgou/pkg/response"

	"github.com/gin-gonic/gin"
)

// ErrorHandler 错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 恢复panic
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered",
					logger.Any("panic", err),
					logger.String("path", c.Request.URL.Path),
					logger.String("method", c.Request.Method),
				)

				response.InternalServerError(c, "Internal server error")
				c.Abort()
			}
		}()

		c.Next()

		// 如果有错误，处理错误
		if len(c.Errors) > 0 {
			for _, ginErr := range c.Errors {
				logger.Error("Request error",
					logger.ErrorField(ginErr),
					logger.String("path", c.Request.URL.Path),
				)
			}
		}
	}
}
