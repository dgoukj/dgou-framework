// pkg/monitor/performance.go
package monitor

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"dgou/pkg/logger"
)

// Profiler 性能分析器
type Profiler struct {
	monitor *Monitor
}

// NewProfiler 创建性能分析器
func NewProfiler(monitor *Monitor) *Profiler {
	return &Profiler{
		monitor: monitor,
	}
}

// StartCPUProfile 开始CPU性能分析
func (p *Profiler) StartCPUProfile(duration time.Duration) (string, error) {
	filename := fmt.Sprintf("cpu_profile_%s.pprof", time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}

	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return "", err
	}

	logger.Info("CPU profiling started", logger.String("file", filename))

	// 定时停止
	time.AfterFunc(duration, func() {
		pprof.StopCPUProfile()
		file.Close()
		logger.Info("CPU profiling stopped", logger.String("file", filename))
	})

	return filename, nil
}

// CaptureHeapProfile 捕获堆内存分析
func (p *Profiler) CaptureHeapProfile() (string, error) {
	filename := fmt.Sprintf("heap_profile_%s.pprof", time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	runtime.GC() // 触发GC以获得准确的内存使用情况
	if err := pprof.WriteHeapProfile(file); err != nil {
		return "", err
	}

	logger.Info("Heap profile captured", logger.String("file", filename))
	return filename, nil
}

// CaptureGoroutineProfile 捕获协程分析
func (p *Profiler) CaptureGoroutineProfile() (string, error) {
	filename := fmt.Sprintf("goroutine_profile_%s.pprof", time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	profile := pprof.Lookup("goroutine")
	if err := profile.WriteTo(file, 2); err != nil {
		return "", err
	}

	logger.Info("Goroutine profile captured", logger.String("file", filename))
	return filename, nil
}

// CaptureBlockProfile 捕获阻塞分析
func (p *Profiler) CaptureBlockProfile() (string, error) {
	filename := fmt.Sprintf("block_profile_%s.pprof", time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	profile := pprof.Lookup("block")
	if err := profile.WriteTo(file, 1); err != nil {
		return "", err
	}

	logger.Info("Block profile captured", logger.String("file", filename))
	return filename, nil
}

// AnalyzeMemory 分析内存使用情况
func (p *Profiler) AnalyzeMemory() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"alloc":           formatBytes(memStats.Alloc),
		"total_alloc":     formatBytes(memStats.TotalAlloc),
		"sys":             formatBytes(memStats.Sys),
		"heap_alloc":      formatBytes(memStats.HeapAlloc),
		"heap_sys":        formatBytes(memStats.HeapSys),
		"heap_idle":       formatBytes(memStats.HeapIdle),
		"heap_inuse":      formatBytes(memStats.HeapInuse),
		"heap_released":   formatBytes(memStats.HeapReleased),
		"heap_objects":    memStats.HeapObjects,
		"stack_inuse":     formatBytes(memStats.StackInuse),
		"stack_sys":       formatBytes(memStats.StackSys),
		"num_gc":          memStats.NumGC,
		"gc_cpu_fraction": memStats.GCCPUFraction,
		"next_gc":         formatBytes(memStats.NextGC),
	}
}

// formatBytes 格式化字节大小
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// PerformanceReport 性能报告
func (p *Profiler) PerformanceReport() map[string]interface{} {
	report := make(map[string]interface{})

	// 运行时信息
	report["runtime"] = map[string]interface{}{
		"goroutines": runtime.NumGoroutine(),
		"cpus":       runtime.NumCPU(),
		"gomaxprocs": runtime.GOMAXPROCS(0),
		"version":    runtime.Version(),
	}

	// 内存信息
	report["memory"] = p.AnalyzeMemory()

	// 监控指标
	if p.monitor != nil && p.monitor.metrics != nil {
		report["metrics"] = p.monitor.GetMetrics()
	}

	return report
}
