package middleware

import (
	"dgou/pkg/logger"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算耗时
		end := time.Now()
		latency := end.Sub(start)

		// 获取状态码
		status := c.Writer.Status()

		// 记录日志
		logger.Info("HTTP Request",
			logger.String("method", c.Request.Method),
			logger.String("path", path),
			logger.String("query", query),
			logger.String("ip", c.ClientIP()),
			logger.String("user-agent", c.Request.UserAgent()),
			logger.Int("status", status),
			logger.Duration("latency", latency),
			logger.Int("body_size", c.Writer.Size()),
		)
	}
}
