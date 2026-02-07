// file: pkg/async/metrics.go
package async

import (
	"sync/atomic"
	"time"
)

// PoolMetrics 协程池指标
type PoolMetrics struct {
	activeWorkers    int64 // 活跃工作协程数
	totalTasks       int64 // 总任务数
	completedTasks   int64 // 完成的任务数
	failedTasks      int64 // 失败的任务数
	queuedTasks      int64 // 排队中的任务数
	rejectedTasks    int64 // 被拒绝的任务数
	cancelledTasks   int64 // 被取消的任务数
	totalProcessTime int64 // 总处理时间（纳秒）
	processCount     int64 // 处理次数
}

// NewPoolMetrics 创建新的指标
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{}
}

// IncActiveWorkers 增加活跃工作协程数
func (m *PoolMetrics) IncActiveWorkers() {
	atomic.AddInt64(&m.activeWorkers, 1)
}

// DecActiveWorkers 减少活跃工作协程数
func (m *PoolMetrics) DecActiveWorkers() {
	atomic.AddInt64(&m.activeWorkers, -1)
}

// GetActiveWorkers 获取活跃工作协程数
func (m *PoolMetrics) GetActiveWorkers() int64 {
	return atomic.LoadInt64(&m.activeWorkers)
}

// IncTotalTasks 增加总任务数
func (m *PoolMetrics) IncTotalTasks() {
	atomic.AddInt64(&m.totalTasks, 1)
}

// GetTotalTasks 获取总任务数
func (m *PoolMetrics) GetTotalTasks() int64 {
	return atomic.LoadInt64(&m.totalTasks)
}

// IncCompletedTasks 增加完成的任务数
func (m *PoolMetrics) IncCompletedTasks() {
	atomic.AddInt64(&m.completedTasks, 1)
}

// GetCompletedTasks 获取完成的任务数
func (m *PoolMetrics) GetCompletedTasks() int64 {
	return atomic.LoadInt64(&m.completedTasks)
}

// IncFailedTasks 增加失败的任务数
func (m *PoolMetrics) IncFailedTasks() {
	atomic.AddInt64(&m.failedTasks, 1)
}

// GetFailedTasks 获取失败的任务数
func (m *PoolMetrics) GetFailedTasks() int64 {
	return atomic.LoadInt64(&m.failedTasks)
}

// IncQueuedTasks 增加排队中的任务数
func (m *PoolMetrics) IncQueuedTasks() {
	atomic.AddInt64(&m.queuedTasks, 1)
}

// GetQueuedTasks 获取排队中的任务数
func (m *PoolMetrics) GetQueuedTasks() int64 {
	return atomic.LoadInt64(&m.queuedTasks)
}

// IncRejectedTasks 增加被拒绝的任务数
func (m *PoolMetrics) IncRejectedTasks() {
	atomic.AddInt64(&m.rejectedTasks, 1)
}

// GetRejectedTasks 获取被拒绝的任务数
func (m *PoolMetrics) GetRejectedTasks() int64 {
	return atomic.LoadInt64(&m.rejectedTasks)
}

// IncCancelledTasks 增加被取消的任务数
func (m *PoolMetrics) IncCancelledTasks() {
	atomic.AddInt64(&m.cancelledTasks, 1)
}

// GetCancelledTasks 获取被取消的任务数
func (m *PoolMetrics) GetCancelledTasks() int64 {
	return atomic.LoadInt64(&m.cancelledTasks)
}

// RecordProcessTime 记录处理时间
func (m *PoolMetrics) RecordProcessTime(duration time.Duration) {
	atomic.AddInt64(&m.totalProcessTime, int64(duration))
	atomic.AddInt64(&m.processCount, 1)
}

// GetAverageProcessTime 获取平均处理时间
func (m *PoolMetrics) GetAverageProcessTime() time.Duration {
	total := atomic.LoadInt64(&m.totalProcessTime)
	count := atomic.LoadInt64(&m.processCount)

	if count == 0 {
		return 0
	}

	return time.Duration(total / count)
}

// Reset 重置指标
func (m *PoolMetrics) Reset() {
	atomic.StoreInt64(&m.activeWorkers, 0)
	atomic.StoreInt64(&m.totalTasks, 0)
	atomic.StoreInt64(&m.completedTasks, 0)
	atomic.StoreInt64(&m.failedTasks, 0)
	atomic.StoreInt64(&m.queuedTasks, 0)
	atomic.StoreInt64(&m.rejectedTasks, 0)
	atomic.StoreInt64(&m.cancelledTasks, 0)
	atomic.StoreInt64(&m.totalProcessTime, 0)
	atomic.StoreInt64(&m.processCount, 0)
}
