// pkg/monitor/init.go
package monitor

import (
	"dgou/pkg/config"
	"dgou/pkg/logger"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

var (
	globalMonitor *Monitor
	monitorOnce   sync.Once
	monitorErr    error
)

// InitMonitor 初始化监控（单例模式）
func InitMonitor(cfg *config.MonitorConfig, engine interface{}) (*Monitor, error) {
	monitorOnce.Do(func() {
		var ginEngine *gin.Engine
		if e, ok := engine.(*gin.Engine); ok {
			ginEngine = e
		}

		monitor, err := NewMonitor(cfg, ginEngine)
		if err != nil {
			monitorErr = err
			return
		}

		// 启动监控
		if err := monitor.Start(); err != nil {
			monitorErr = err
			return
		}

		globalMonitor = monitor

		// 注册优雅关闭
		// 这里需要与应用优雅关闭机制集成

		logger.Info("Monitor initialized successfully")
	})

	return globalMonitor, monitorErr
}

// GetMonitor 获取全局监控实例
func GetMonitor() *Monitor {
	if globalMonitor == nil {
		logger.Warn("Monitor not initialized, please call InitMonitor first")
	}
	return globalMonitor
}

// MustGetMonitor 获取监控实例（失败时panic）
func MustGetMonitor() *Monitor {
	monitor := GetMonitor()
	if monitor == nil {
		panic("monitor is not initialized")
	}
	return monitor
}

// RecordHTTPRequest 记录HTTP请求（快捷方法）
func RecordHTTPRequest(method, path, status, handler string, duration float64) {
	if monitor := GetMonitor(); monitor != nil && monitor.metrics != nil {
		monitor.metrics.httpRequestsTotal.WithLabelValues(
			method, path, status, handler,
		).Inc()

		monitor.metrics.httpRequestDuration.WithLabelValues(
			method, path, status,
		).Observe(duration)
	}
}

// RecordDBQuery 记录数据库查询（快捷方法）
func RecordDBQuery(queryType, table, operation string, duration time.Duration, success bool) {
	if monitor := GetMonitor(); monitor != nil {
		monitor.RecordDBQuery(queryType, table, operation, duration, success)
	}
}

// RecordBusinessEvent 记录业务事件（快捷方法）
func RecordBusinessEvent(event, status string) {
	if monitor := GetMonitor(); monitor != nil {
		monitor.RecordBusinessEvent(event, status)
	}
}

// CloseMonitor 关闭监控连接
func CloseMonitor() error {
	if globalMonitor == nil {
		return nil
	}
	return globalMonitor.Stop()
}
