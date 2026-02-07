// internal/middleware/rate_limiter.go
package middleware

import (
	"dgou/pkg/config"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// RateLimiter 限流中间件
func RateLimiter() gin.HandlerFunc {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.AppConfig.Redis.Addr,
		Password: config.AppConfig.Redis.Password,
		DB:       config.AppConfig.Redis.DB,
	})

	return func(c *gin.Context) {
		key := "rate_limit:" + c.ClientIP()
		limit := 100 // 每分钟限制请求数
		duration := time.Minute

		// 获取当前计数
		count, err := rdb.Incr(c, key).Result()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// 如果是第一次请求，设置过期时间
		if count == 1 {
			rdb.Expire(c, key, duration)
		}

		// 检查是否超过限制
		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			return
		}

		c.Next()
	}
}
