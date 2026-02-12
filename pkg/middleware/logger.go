package middleware

import (
	"dgou/pkg/logger"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.Int("body_size", c.Writer.Size()),
		}
		if requestID, exists := c.Get("request_id"); exists {
			fields = append(fields, zap.String("request_id", requestID.(string)))
		}
		log.Info("HTTP Request", fields...)
	}
}
