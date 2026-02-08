// pkg/monitor/middleware.go
package monitor

import (
	"context"
	"dgou/pkg/logger"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware 分布式追踪中间件
func (m *Monitor) TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.EnableTracing || m.tracerProvider == nil {
			c.Next()
			return
		}

		propagator := otel.GetTextMapPropagator()
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		tracer := m.tracerProvider.Tracer("http")
		spanName := c.Request.Method + " " + c.FullPath()

		// 创建属性列表
		attrs := []attribute.KeyValue{
			semconv.HTTPMethodKey.String(c.Request.Method),
			semconv.HTTPURLKey.String(c.Request.URL.String()),
			semconv.HTTPRouteKey.String(c.FullPath()),
			semconv.HTTPRequestContentLengthKey.Int(int(c.Request.ContentLength)),
			semconv.HTTPUserAgentKey.String(c.Request.UserAgent()),
		}

		// 添加客户端IP
		if c.ClientIP() != "" {
			attrs = append(attrs, semconv.HTTPClientIPKey.String(c.ClientIP()))
		}

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attrs...),
		)
		defer span.End()

		// 将span上下文存储到请求中
		c.Request = c.Request.WithContext(ctx)

		// 处理请求
		c.Next()

		// 记录响应信息
		status := c.Writer.Status()
		span.SetAttributes(
			semconv.HTTPStatusCodeKey.Int(status),
		)

		// 如果有响应大小，记录它
		if size := c.Writer.Size(); size > 0 {
			span.SetAttributes(
				semconv.HTTPResponseContentLengthKey.Int(size),
			)
		}

		// 设置span状态
		if status >= 400 {
			span.SetStatus(codes.Error, http.StatusText(status))
		}

		// 记录错误
		if len(c.Errors) > 0 {
			span.RecordError(c.Errors.Last())
		}
	}
}

// MetricsMiddleware 指标收集中间件
func (m *Monitor) MetricsMiddleware() gin.HandlerFunc {
	return m.Middleware()
}

// RecoveryMiddleware 恢复中间件（带监控）
func (m *Monitor) RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录panic指标
				m.recordPanic(c, err)

				// 调用默认恢复
				gin.Recovery()(c)

				// 更新错误计数
				if m.metrics != nil {
					m.runtimeStats.errorsTotal.Inc()
				}
			}
		}()

		c.Next()
	}
}

// recordPanic 记录panic信息
func (m *Monitor) recordPanic(c *gin.Context, err interface{}) {
	logger.Error("Request panic recovered",
		logger.String("method", c.Request.Method),
		logger.String("path", c.FullPath()),
		logger.String("client_ip", c.ClientIP()),
		logger.Any("panic", err),
	)
}

// TimeoutMiddleware 超时中间件（带监控）
func (m *Monitor) TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		done := make(chan struct{})

		go func() {
			defer func() {
				if err := recover(); err != nil {
					m.recordPanic(c, err)
					c.AbortWithStatus(http.StatusInternalServerError)
				}
			}()
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			// 请求正常完成
			return
		case <-ctx.Done():
			// 请求超时
			m.recordTimeout(c, timeout)
			c.AbortWithStatus(http.StatusGatewayTimeout)
		}
	}
}

// recordTimeout 记录超时信息
func (m *Monitor) recordTimeout(c *gin.Context, timeout time.Duration) {
	logger.Warn("Request timeout",
		logger.String("method", c.Request.Method),
		logger.String("path", c.FullPath()),
		logger.Duration("timeout", timeout),
		logger.String("client_ip", c.ClientIP()),
	)

	// 更新指标
	if m.metrics != nil {
		m.runtimeStats.errorsTotal.Inc()
	}
}

// RateLimitMiddleware 限流中间件（带监控）
func (m *Monitor) RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	type requestInfo struct {
		count       int
		windowStart time.Time
	}

	clients := make(map[string]*requestInfo)
	var mu sync.RWMutex

	// 清理过期记录
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			mu.Lock()
			for ip, info := range clients {
				if time.Since(info.windowStart) > window {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		info, exists := clients[ip]
		if !exists {
			info = &requestInfo{
				windowStart: now,
			}
			clients[ip] = info
		}

		// 重置窗口
		if time.Since(info.windowStart) > window {
			info.count = 0
			info.windowStart = now
		}

		// 检查是否超过限制
		if info.count >= limit {
			mu.Unlock()

			// 记录限流
			m.recordRateLimit(c, limit, window)
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		info.count++
		mu.Unlock()

		c.Next()
	}
}

// recordRateLimit 记录限流信息
func (m *Monitor) recordRateLimit(c *gin.Context, limit int, window time.Duration) {
	logger.Warn("Rate limit exceeded",
		logger.String("method", c.Request.Method),
		logger.String("path", c.FullPath()),
		logger.String("client_ip", c.ClientIP()),
		logger.Int("limit", limit),
		logger.Duration("window", window),
	)

	// 更新指标
	if m.metrics != nil {
		m.runtimeStats.errorsTotal.Inc()
	}
}
