package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter 限流器
type rateLimiter struct {
	mu       sync.Mutex
	counters map[string]*rateCounter
	limit    int
	window   time.Duration
}

// rateCounter 计数器
type rateCounter struct {
	count    int
	window   time.Duration
	lastTime time.Time
}

// RateLimiter 限流中间件 - 简化版本，接受limit参数
func RateLimiter(limit int) gin.HandlerFunc {
	if limit <= 0 {
		limit = 100 // 默认限制
	}

	rl := &rateLimiter{
		counters: make(map[string]*rateCounter),
		limit:    limit,
		window:   time.Minute, // 1分钟窗口
	}

	// 定期清理过期计数器
	go rl.cleanup()

	return func(c *gin.Context) {
		// 获取客户端IP作为限流key
		key := c.ClientIP()

		rl.mu.Lock()
		counter, exists := rl.counters[key]
		if !exists {
			counter = &rateCounter{
				count:    0,
				window:   rl.window,
				lastTime: time.Now(),
			}
			rl.counters[key] = counter
		}

		// 检查是否需要重置计数器
		if time.Since(counter.lastTime) > counter.window {
			counter.count = 0
			counter.lastTime = time.Now()
		}

		// 检查是否超过限制
		if counter.count >= rl.limit {
			rl.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":  "Rate limit exceeded",
				"limit":  rl.limit,
				"window": rl.window.Seconds(),
			})
			return
		}

		// 增加计数
		counter.count++
		rl.mu.Unlock()

		// 设置响应头
		c.Header("X-RateLimit-Limit", string(rune(rl.limit)))
		c.Header("X-RateLimit-Remaining", string(rune(rl.limit-counter.count)))
		c.Header("X-RateLimit-Reset", string(rune(time.Now().Add(rl.window).Unix())))

		c.Next()
	}
}

// cleanup 定期清理过期计数器
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, counter := range rl.counters {
			if now.Sub(counter.lastTime) > 2*counter.window {
				delete(rl.counters, key)
			}
		}
		rl.mu.Unlock()
	}
}
