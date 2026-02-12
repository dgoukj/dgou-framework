package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	mu       sync.Mutex
	counters map[string]*rateCounter
	limit    int
	window   time.Duration
}

type rateCounter struct {
	count    int
	window   time.Duration
	lastTime time.Time
}

func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	if limit <= 0 {
		limit = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	rl := &rateLimiter{
		counters: make(map[string]*rateCounter),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup()
	return func(c *gin.Context) {
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
		if time.Since(counter.lastTime) > counter.window {
			counter.count = 0
			counter.lastTime = time.Now()
		}
		if counter.count >= rl.limit {
			rl.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"limit":       rl.limit,
				"window":      rl.window.Seconds(),
				"retry_after": counter.window - time.Since(counter.lastTime),
			})
			return
		}
		counter.count++
		rl.mu.Unlock()

		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(rl.limit-counter.count))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(rl.window).Unix(), 10))

		c.Next()
	}
}

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
