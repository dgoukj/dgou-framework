package app

import (
	"context"
	"dgou/pkg/cache"
	"dgou/pkg/database"
	"dgou/pkg/logger"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	checks []HealthCheck
	mu     sync.RWMutex
}

// HealthCheck 健康检查接口
type HealthCheck interface {
	Check() HealthStatus
	Name() string
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status  string `json:"status"`            // 状态：healthy, unhealthy, degraded
	Message string `json:"message,omitempty"` // 状态消息
	Latency int64  `json:"latency_ms"`        // 延迟（毫秒）
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker() *HealthChecker {
	hc := &HealthChecker{
		checks: make([]HealthCheck, 0),
	}

	// 注册默认检查项
	hc.Register(&DatabaseCheck{})
	hc.Register(&CacheCheck{})

	return hc
}

// Register 注册健康检查
func (hc *HealthChecker) Register(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks = append(hc.checks, check)
}

// Handler 健康检查处理器
func (hc *HealthChecker) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 执行所有健康检查
		results := hc.checkAll()

		// 确定总体状态
		overallStatus := "healthy"
		httpStatus := http.StatusOK

		for _, result := range results {
			if result.Status == "unhealthy" {
				overallStatus = "unhealthy"
				httpStatus = http.StatusServiceUnavailable
				break
			} else if result.Status == "degraded" && overallStatus == "healthy" {
				overallStatus = "degraded"
				httpStatus = http.StatusOK // 仍返回200，但状态为degraded
			}
		}

		// 构建响应
		response := gin.H{
			"status":    overallStatus,
			"timestamp": time.Now().Unix(),
			"checks":    results,
		}

		c.JSON(httpStatus, response)
	}
}

// ReadyHandler 就绪检查处理器
func (hc *HealthChecker) ReadyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查核心依赖是否就绪
		checks := []HealthCheck{
			&DatabaseCheck{},
			&CacheCheck{},
		}

		ready := true
		for _, check := range checks {
			status := check.Check()
			if status.Status != "healthy" {
				ready = false
				break
			}
		}

		if ready {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
		}
	}
}

// LiveHandler 存活检查处理器
func (hc *HealthChecker) LiveHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 存活检查只需要返回成功即可
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	}
}

// checkAll 执行所有检查
func (hc *HealthChecker) checkAll() map[string]HealthStatus {
	hc.mu.RLock()
	checks := hc.checks
	hc.mu.RUnlock()

	results := make(map[string]HealthStatus)

	// 并行执行检查
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, check := range checks {
		wg.Add(1)
		go func(check HealthCheck) {
			defer wg.Done()

			start := time.Now()
			status := check.Check()
			elapsed := time.Since(start)

			status.Latency = elapsed.Milliseconds()

			mu.Lock()
			results[check.Name()] = status
			mu.Unlock()
		}(check)
	}

	wg.Wait()
	return results
}

// ==================== 具体检查实现 ====================

// DatabaseCheck 数据库检查
type DatabaseCheck struct{}

func (d *DatabaseCheck) Name() string {
	return "database"
}

func (d *DatabaseCheck) Check() HealthStatus {
	db := database.GetDB()
	if db == nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: "Database not initialized",
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return HealthStatus{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}

	if err := sqlDB.Ping(); err != nil {
		logger.Error("Database health check failed", logger.ErrorField(err))
		return HealthStatus{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}

	// 检查连接数
	stats := sqlDB.Stats()
	if stats.OpenConnections > int(float64(stats.MaxOpenConnections)*0.9) {
		return HealthStatus{
			Status:  "degraded",
			Message: "Database connections are running high",
		}
	}

	return HealthStatus{
		Status: "healthy",
	}
}

// CacheCheck 缓存检查
type CacheCheck struct{}

func (c *CacheCheck) Name() string {
	return "cache"
}

func (c *CacheCheck) Check() HealthStatus {
	client := cache.GetClient()
	if client == nil {
		return HealthStatus{
			Status:  "degraded",
			Message: "Cache not available, using memory cache",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("Cache health check failed", logger.ErrorField(err))
		return HealthStatus{
			Status:  "degraded",
			Message: err.Error(),
		}
	}

	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		return HealthStatus{
			Status:  "degraded",
			Message: "Cache response is slow",
		}
	}

	return HealthStatus{
		Status: "healthy",
	}
}

// CustomHealthCheck 自定义健康检查
type CustomHealthCheck struct {
	name    string
	checkFn func() HealthStatus
}

func NewCustomHealthCheck(name string, checkFn func() HealthStatus) *CustomHealthCheck {
	return &CustomHealthCheck{
		name:    name,
		checkFn: checkFn,
	}
}

func (c *CustomHealthCheck) Name() string {
	return c.name
}

func (c *CustomHealthCheck) Check() HealthStatus {
	return c.checkFn()
}
