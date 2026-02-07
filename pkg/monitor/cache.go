// pkg/monitor/cache.go
package monitor

import (
	"context"
	"time"
)

// CacheMonitor 缓存监控包装器
type CacheMonitor struct {
	monitor *Monitor
}

// NewCacheMonitor 创建缓存监控器
func NewCacheMonitor(monitor *Monitor) *CacheMonitor {
	return &CacheMonitor{
		monitor: monitor,
	}
}

// RecordGet 记录缓存读取操作
func (cm *CacheMonitor) RecordGet(ctx context.Context, cacheType string, duration time.Duration, hit bool) {
	cm.monitor.RecordCacheOperation(cacheType, "get", duration, hit)
}

// RecordSet 记录缓存设置操作
func (cm *CacheMonitor) RecordSet(ctx context.Context, cacheType string, duration time.Duration) {
	cm.monitor.RecordCacheOperation(cacheType, "set", duration, false)
}

// RecordDelete 记录缓存删除操作
func (cm *CacheMonitor) RecordDelete(ctx context.Context, cacheType string, duration time.Duration) {
	cm.monitor.RecordCacheOperation(cacheType, "delete", duration, false)
}

// RecordIncr 记录缓存递增操作
func (cm *CacheMonitor) RecordIncr(ctx context.Context, cacheType string, duration time.Duration) {
	cm.monitor.RecordCacheOperation(cacheType, "incr", duration, false)
}
