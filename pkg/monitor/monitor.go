// pkg/monitor/monitor.go
package monitor

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Monitor 监控管理器
type Monitor struct {
	config *config.MonitorConfig
	engine *gin.Engine

	// Prometheus
	promRegistry *prometheus.Registry
	metrics      *Metrics

	// OpenTelemetry
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider

	// Runtime monitoring
	runtimeStats *RuntimeStats

	// Alert manager
	alertManager *AlertManager

	// Health checks
	healthChecks []HealthCheck

	mu sync.RWMutex
}

// Metrics 应用指标
type Metrics struct {
	// HTTP metrics
	httpRequestsTotal     *prometheus.CounterVec
	httpRequestDuration   *prometheus.HistogramVec
	httpRequestSizeBytes  *prometheus.HistogramVec
	httpResponseSizeBytes *prometheus.HistogramVec
	httpInflightRequests  prometheus.Gauge

	// Database metrics
	dbQueriesTotal     *prometheus.CounterVec
	dbQueryDuration    *prometheus.HistogramVec
	dbConnectionsInUse prometheus.Gauge
	dbConnectionsIdle  prometheus.Gauge
	dbConnectionsOpen  prometheus.Gauge

	// Cache metrics
	cacheHitsTotal          *prometheus.CounterVec
	cacheMissesTotal        *prometheus.CounterVec
	cacheOperationsDuration *prometheus.HistogramVec

	// Business metrics
	businessEventsTotal    *prometheus.CounterVec
	businessProcessingTime *prometheus.HistogramVec

	// System metrics
	goRoutines     prometheus.Gauge
	goMemAlloc     prometheus.Gauge
	goMemTotal     prometheus.Gauge
	goGCCycles     prometheus.Counter
	goGCPauseTotal prometheus.Counter

	// Custom metrics registry
	customMetrics sync.Map
}

// RuntimeStats 运行时统计
type RuntimeStats struct {
	startTime      time.Time
	uptime         prometheus.Gauge
	requestsServed prometheus.Counter
	errorsTotal    prometheus.Counter
}

// HealthCheck 健康检查接口
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) HealthStatus
}

// HealthStatus 健康状态
type HealthStatus struct {
	Name     string                 `json:"name"`
	Status   string                 `json:"status"` // "healthy", "unhealthy", "degraded"
	Message  string                 `json:"message,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name        string                 `yaml:"name"`
	Expr        string                 `yaml:"expr"`
	Duration    string                 `yaml:"duration"`
	Labels      map[string]string      `yaml:"labels"`
	Annotations map[string]string      `yaml:"annotations"`
	Severity    string                 `yaml:"severity"` // critical, warning, info
	Enabled     bool                   `yaml:"enabled"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty"`
}

// AlertManager 告警管理器
type AlertManager struct {
	rules     []AlertRule
	handlers  []AlertHandler
	alertChan chan Alert
	mu        sync.RWMutex
}

// Alert 告警
type Alert struct {
	Rule        AlertRule              `json:"rule"`
	Value       float64                `json:"value"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AlertHandler 告警处理器
type AlertHandler interface {
	Handle(ctx context.Context, alert Alert) error
}

// NewMonitor 创建监控管理器
func NewMonitor(cfg *config.MonitorConfig, engine *gin.Engine) (*Monitor, error) {
	if cfg == nil {
		return nil, errors.New(errors.CodeValidationFailed, "monitor config is required")
	}

	m := &Monitor{
		config: cfg,
		engine: engine,
		runtimeStats: &RuntimeStats{
			startTime: time.Now(),
		},
	}

	// 初始化Prometheus
	if err := m.initPrometheus(); err != nil {
		return nil, err
	}

	// 初始化OpenTelemetry
	if cfg.EnableTracing {
		if err := m.initTracing(); err != nil {
			logger.Warn("Failed to initialize tracing", logger.ErrorField(err))
		}
	}

	// 初始化告警管理器
	if cfg.EnableAlerts {
		m.alertManager = NewAlertManager()
	}

	// 注册健康检查
	m.registerDefaultHealthChecks()

	return m, nil
}

// initPrometheus 初始化Prometheus指标
func (m *Monitor) initPrometheus() error {
	// 创建注册表
	m.promRegistry = prometheus.NewRegistry()

	// 注册默认指标
	m.promRegistry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	m.promRegistry.MustRegister(prometheus.NewGoCollector())

	// 初始化自定义指标
	if err := m.initMetrics(); err != nil {
		return err
	}

	// 添加指标路由
	if m.config.EnableMetrics && m.engine != nil {
		m.engine.GET(m.config.MetricsPath, gin.WrapH(promhttp.HandlerFor(
			m.promRegistry,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
				Timeout:           5 * time.Second,
			},
		)))
	}

	return nil
}

// initMetrics 初始化应用指标
func (m *Monitor) initMetrics() error {
	m.metrics = &Metrics{}

	// HTTP指标
	m.metrics.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
			ConstLabels: prometheus.Labels{
				"service": m.config.ServiceName,
			},
		},
		[]string{"method", "path", "status", "handler"},
	)

	m.metrics.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	m.metrics.httpRequestSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)

	m.metrics.httpResponseSizeBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path", "status"},
	)

	m.metrics.httpInflightRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_inflight_requests",
			Help: "Current number of inflight HTTP requests",
		},
	)

	// 数据库指标
	m.metrics.dbQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"type", "table", "operation"},
	)

	m.metrics.dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
		},
		[]string{"type", "table", "operation"},
	)

	m.metrics.dbConnectionsInUse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	m.metrics.dbConnectionsIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	m.metrics.dbConnectionsOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_open",
			Help: "Total number of open database connections",
		},
	)

	// 缓存指标
	m.metrics.cacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"type", "operation"},
	)

	m.metrics.cacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"type", "operation"},
	)

	m.metrics.cacheOperationsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cache_operation_duration_seconds",
			Help:    "Cache operation duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10),
		},
		[]string{"type", "operation"},
	)

	// 业务指标
	m.metrics.businessEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "business_events_total",
			Help: "Total number of business events",
		},
		[]string{"event", "status"},
	)

	m.metrics.businessProcessingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "business_processing_time_seconds",
			Help:    "Business processing time in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"process", "status"},
	)

	// 系统指标
	m.metrics.goRoutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_goroutines",
			Help: "Number of Go goroutines",
		},
	)

	m.metrics.goMemAlloc = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_mem_alloc_bytes",
			Help: "Bytes allocated and still in use",
		},
	)

	m.metrics.goMemTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_mem_total_bytes",
			Help: "Total bytes allocated",
		},
	)

	m.metrics.goGCCycles = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "go_gc_cycles_total",
			Help: "Total number of GC cycles",
		},
	)

	m.metrics.goGCPauseTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "go_gc_pause_total_seconds",
			Help: "Total GC pause time in seconds",
		},
	)

	// 运行时统计
	m.runtimeStats.uptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "service_uptime_seconds",
			Help: "Service uptime in seconds",
		},
	)

	m.runtimeStats.requestsServed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "service_requests_served_total",
			Help: "Total number of requests served",
		},
	)

	m.runtimeStats.errorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "service_errors_total",
			Help: "Total number of errors",
		},
	)

	// 注册所有指标
	metrics := []prometheus.Collector{
		m.metrics.httpRequestsTotal,
		m.metrics.httpRequestDuration,
		m.metrics.httpRequestSizeBytes,
		m.metrics.httpResponseSizeBytes,
		m.metrics.httpInflightRequests,
		m.metrics.dbQueriesTotal,
		m.metrics.dbQueryDuration,
		m.metrics.dbConnectionsInUse,
		m.metrics.dbConnectionsIdle,
		metrics.dbConnectionsOpen,
		m.metrics.cacheHitsTotal,
		m.metrics.cacheMissesTotal,
		m.metrics.cacheOperationsDuration,
		m.metrics.businessEventsTotal,
		m.metrics.businessProcessingTime,
		m.metrics.goRoutines,
		m.metrics.goMemAlloc,
		m.metrics.goMemTotal,
		m.metrics.goGCCycles,
		m.metrics.goGCPauseTotal,
		m.runtimeStats.uptime,
		m.runtimeStats.requestsServed,
		m.runtimeStats.errorsTotal,
	}

	for _, metric := range metrics {
		if err := m.promRegistry.Register(metric); err != nil {
			logger.Warn("Failed to register metric", logger.ErrorField(err))
		}
	}

	// 启动运行时指标收集
	go m.collectRuntimeMetrics()

	return nil
}

// initTracing 初始化分布式追踪
func (m *Monitor) initTracing() error {
	// 创建Jaeger导出器
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint(m.config.JaegerEndpoint),
	))
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to create Jaeger exporter")
	}

	// 创建资源
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(m.config.ServiceName),
		semconv.ServiceVersion(m.config.ServiceVersion),
		attribute.String("environment", m.config.Environment),
	)

	// 创建追踪提供者
	m.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(m.config.TraceSamplingRate)),
	)

	// 设置为全局提供者
	otel.SetTracerProvider(m.tracerProvider)

	// 初始化指标提供者
	if err := m.initMeterProvider(); err != nil {
		return err
	}

	return nil
}

// initMeterProvider 初始化指标提供者
func (m *Monitor) initMeterProvider() error {
	// 创建Prometheus导出器
	exporter, err := prometheus.New()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to create Prometheus exporter")
	}

	// 创建指标提供者
	m.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter.Reader),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(m.config.ServiceName),
		)),
	)

	// 设置为全局提供者
	otel.SetMeterProvider(m.meterProvider)

	return nil
}

// registerDefaultHealthChecks 注册默认健康检查
func (m *Monitor) registerDefaultHealthChecks() {
	m.healthChecks = []HealthCheck{
		&SystemHealthCheck{},
	}
}

// collectRuntimeMetrics 收集运行时指标
func (m *Monitor) collectRuntimeMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	var memStats runtime.MemStats
	var lastNumGC uint32
	var lastPauseTotal uint64

	for {
		select {
		case <-ticker.C:
			// 更新运行时间
			m.runtimeStats.uptime.Set(time.Since(m.runtimeStats.startTime).Seconds())

			// 收集内存统计
			runtime.ReadMemStats(&memStats)

			m.metrics.goRoutines.Set(float64(runtime.NumGoroutine()))
			m.metrics.goMemAlloc.Set(float64(memStats.Alloc))
			m.metrics.goMemTotal.Set(float64(memStats.TotalAlloc))

			// GC统计
			if memStats.NumGC > lastNumGC {
				m.metrics.goGCCycles.Add(float64(memStats.NumGC - lastNumGC))
				lastNumGC = memStats.NumGC
			}

			if memStats.PauseTotalNs > lastPauseTotal {
				pauseSeconds := float64(memStats.PauseTotalNs-lastPauseTotal) / 1e9
				m.metrics.goGCPauseTotal.Add(pauseSeconds)
				lastPauseTotal = memStats.PauseTotalNs
			}
		}
	}
}

// Middleware 监控中间件
func (m *Monitor) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过监控端点
		path := c.Request.URL.Path
		if path == m.config.MetricsPath ||
			path == m.config.HealthPath ||
			path == m.config.ProfilePath {
			c.Next()
			return
		}

		start := time.Now()

		// 增加正在处理的请求数
		m.metrics.httpInflightRequests.Inc()
		defer m.metrics.httpInflightRequests.Dec()

		// 记录请求大小
		reqSize := computeApproximateRequestSize(c.Request)
		m.metrics.httpRequestSizeBytes.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
		).Observe(float64(reqSize))

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())

		m.metrics.httpRequestsTotal.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			status,
			c.HandlerName(),
		).Inc()

		m.metrics.httpRequestDuration.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			status,
		).Observe(duration)

		// 记录响应大小
		respSize := float64(c.Writer.Size())
		if respSize > 0 {
			m.metrics.httpResponseSizeBytes.WithLabelValues(
				c.Request.Method,
				c.FullPath(),
				status,
			).Observe(respSize)
		}

		// 更新请求计数
		m.runtimeStats.requestsServed.Inc()

		// 记录错误
		if c.Writer.Status() >= 400 {
			m.runtimeStats.errorsTotal.Inc()
		}
	}
}

// computeApproximateRequestSize 计算近似请求大小
func computeApproximateRequestSize(r *http.Request) float64 {
	var size int64

	// URL
	size += int64(len(r.URL.String()))

	// 方法
	size += int64(len(r.Method))

	// 协议
	size += int64(len(r.Proto))

	// 头部
	for name, values := range r.Header {
		size += int64(len(name))
		for _, value := range values {
			size += int64(len(value))
		}
	}

	// 内容长度
	size += r.ContentLength

	return float64(size)
}

// RegisterHealthCheck 注册健康检查
func (m *Monitor) RegisterHealthCheck(check HealthCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthChecks = append(m.healthChecks, check)
}

// HealthHandler 健康检查处理器
func (m *Monitor) HealthHandler(c *gin.Context) {
	ctx := c.Request.Context()
	detailed := c.Query("detailed") == "true"

	status := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   m.config.ServiceName,
		"version":   m.config.ServiceVersion,
	}

	if detailed {
		checks := make([]HealthStatus, 0, len(m.healthChecks))
		unhealthyCount := 0

		for _, check := range m.healthChecks {
			healthStatus := check.Check(ctx)
			checks = append(checks, healthStatus)

			if healthStatus.Status != "healthy" {
				unhealthyCount++
			}
		}

		status["checks"] = checks
		if unhealthyCount > 0 {
			status["status"] = "unhealthy"
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}
	}

	c.JSON(http.StatusOK, status)
}

// RegisterProfileRoutes 注册性能分析路由
func (m *Monitor) RegisterProfileRoutes() {
	if !m.config.EnableProfiling {
		return
	}

	group := m.engine.Group(m.config.ProfilePath)
	{
		// Go内置pprof
		group.GET("/", gin.WrapH(http.HandlerFunc(pprof.Index)))
		group.GET("/cmdline", gin.WrapH(http.HandlerFunc(pprof.Cmdline)))
		group.GET("/profile", gin.WrapH(http.HandlerFunc(pprof.Profile)))
		group.GET("/symbol", gin.WrapH(http.HandlerFunc(pprof.Symbol)))
		group.GET("/trace", gin.WrapH(http.HandlerFunc(pprof.Trace)))
		group.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
		group.GET("/block", gin.WrapH(pprof.Handler("block")))
		group.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		group.GET("/heap", gin.WrapH(pprof.Handler("heap")))
		group.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
		group.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))

		// 自定义调试端点
		group.GET("/vars", m.VarsHandler)
		group.GET("/metrics/debug", m.DebugMetricsHandler)
	}
}

// VarsHandler 显示expvar变量
func (m *Monitor) VarsHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(c.Writer, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(c.Writer, ",\n")
		}
		first = false
		fmt.Fprintf(c.Writer, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(c.Writer, "\n}\n")
}

// DebugMetricsHandler 调试指标处理器
func (m *Monitor) DebugMetricsHandler(c *gin.Context) {
	metrics := m.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// GetMetrics 获取所有指标
func (m *Monitor) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})

	// 基本统计
	metrics["uptime"] = time.Since(m.runtimeStats.startTime).String()
	metrics["goroutines"] = runtime.NumGoroutine()

	// 内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	metrics["memory"] = map[string]interface{}{
		"alloc":       memStats.Alloc,
		"total_alloc": memStats.TotalAlloc,
		"sys":         memStats.Sys,
		"heap_alloc":  memStats.HeapAlloc,
		"heap_sys":    memStats.HeapSys,
		"heap_idle":   memStats.HeapIdle,
		"heap_inuse":  memStats.HeapInuse,
		"num_gc":      memStats.NumGC,
		"gc_pause":    memStats.PauseTotalNs,
	}

	return metrics
}

// RecordDBQuery 记录数据库查询指标
func (m *Monitor) RecordDBQuery(queryType, table, operation string, duration time.Duration, success bool) {
	if m.metrics == nil {
		return
	}

	status := "success"
	if !success {
		status = "error"
	}

	m.metrics.dbQueriesTotal.WithLabelValues(
		queryType,
		table,
		operation,
		status,
	).Inc()

	m.metrics.dbQueryDuration.WithLabelValues(
		queryType,
		table,
		operation,
	).Observe(duration.Seconds())
}

// RecordCacheOperation 记录缓存操作指标
func (m *Monitor) RecordCacheOperation(cacheType, operation string, duration time.Duration, hit bool) {
	if m.metrics == nil {
		return
	}

	if hit {
		m.metrics.cacheHitsTotal.WithLabelValues(cacheType, operation).Inc()
	} else {
		m.metrics.cacheMissesTotal.WithLabelValues(cacheType, operation).Inc()
	}

	m.metrics.cacheOperationsDuration.WithLabelValues(
		cacheType,
		operation,
	).Observe(duration.Seconds())
}

// RecordBusinessEvent 记录业务事件指标
func (m *Monitor) RecordBusinessEvent(event, status string) {
	if m.metrics == nil {
		return
	}

	m.metrics.businessEventsTotal.WithLabelValues(event, status).Inc()
}

// RecordBusinessProcessing 记录业务处理时间指标
func (m *Monitor) RecordBusinessProcessing(process, status string, duration time.Duration) {
	if m.metrics == nil {
		return
	}

	m.metrics.businessProcessingTime.WithLabelValues(process, status).Observe(duration.Seconds())
}

// UpdateDBConnectionStats 更新数据库连接统计
func (m *Monitor) UpdateDBConnectionStats(inUse, idle, open int) {
	if m.metrics == nil {
		return
	}

	m.metrics.dbConnectionsInUse.Set(float64(inUse))
	m.metrics.dbConnectionsIdle.Set(float64(idle))
	m.metrics.dbConnectionsOpen.Set(float64(open))
}

// RegisterCustomMetric 注册自定义指标
func (m *Monitor) RegisterCustomMetric(name string, metric prometheus.Collector) error {
	if m.promRegistry == nil {
		return errors.New(errors.CodeInternalError, "prometheus registry not initialized")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.promRegistry.Register(metric); err != nil {
		return err
	}

	m.metrics.customMetrics.Store(name, metric)
	return nil
}

// Start 启动监控
func (m *Monitor) Start() error {
	// 注册健康检查路由
	if m.config.EnableHealth && m.engine != nil {
		m.engine.GET(m.config.HealthPath, m.HealthHandler)
	}

	// 注册性能分析路由
	if m.config.EnableProfiling && m.engine != nil {
		m.RegisterProfileRoutes()
	}

	// 启动告警管理器
	if m.alertManager != nil {
		m.alertManager.Start()
	}

	logger.Info("Monitor started",
		logger.String("service", m.config.ServiceName),
		logger.Bool("metrics_enabled", m.config.EnableMetrics),
		logger.Bool("tracing_enabled", m.config.EnableTracing),
		logger.Bool("health_enabled", m.config.EnableHealth),
	)

	return nil
}

// Stop 停止监控
func (m *Monitor) Stop() error {
	// 关闭追踪提供者
	if m.tracerProvider != nil {
		if err := m.tracerProvider.Shutdown(context.Background()); err != nil {
			logger.Warn("Failed to shutdown tracer provider", logger.ErrorField(err))
		}
	}

	// 关闭指标提供者
	if m.meterProvider != nil {
		if err := m.meterProvider.Shutdown(context.Background()); err != nil {
			logger.Warn("Failed to shutdown meter provider", logger.ErrorField(err))
		}
	}

	// 停止告警管理器
	if m.alertManager != nil {
		m.alertManager.Stop()
	}

	logger.Info("Monitor stopped")
	return nil
}

// SystemHealthCheck 系统健康检查
type SystemHealthCheck struct{}

func (c *SystemHealthCheck) Name() string {
	return "system"
}

func (c *SystemHealthCheck) Check(ctx context.Context) HealthStatus {
	// 检查内存使用
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	memUsage := float64(memStats.Alloc) / float64(memStats.Sys)

	status := "healthy"
	message := "System is healthy"
	metadata := map[string]interface{}{
		"goroutines":   runtime.NumGoroutine(),
		"memory_usage": memUsage,
		"memory_alloc": memStats.Alloc,
		"memory_sys":   memStats.Sys,
	}

	// 内存使用超过80%视为降级
	if memUsage > 0.8 {
		status = "degraded"
		message = "High memory usage detected"
	}

	// 内存使用超过90%视为不健康
	if memUsage > 0.9 {
		status = "unhealthy"
		message = "Critical memory usage detected"
	}

	return HealthStatus{
		Name:     c.Name(),
		Status:   status,
		Message:  message,
		Metadata: metadata,
	}
}

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	return &AlertManager{
		alertChan: make(chan Alert, 100),
		handlers:  make([]AlertHandler, 0),
	}
}

// AddRule 添加告警规则
func (am *AlertManager) AddRule(rule AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rules = append(am.rules, rule)
}

// AddHandler 添加告警处理器
func (am *AlertManager) AddHandler(handler AlertHandler) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.handlers = append(am.handlers, handler)
}

// Start 启动告警管理器
func (am *AlertManager) Start() {
	go am.processAlerts()
}

// Stop 停止告警管理器
func (am *AlertManager) Stop() {
	close(am.alertChan)
}

// TriggerAlert 触发告警
func (am *AlertManager) TriggerAlert(alert Alert) {
	select {
	case am.alertChan <- alert:
		// 成功发送
	default:
		logger.Warn("Alert channel is full, dropping alert",
			logger.String("rule", alert.Rule.Name),
		)
	}
}

// processAlerts 处理告警
func (am *AlertManager) processAlerts() {
	for alert := range am.alertChan {
		am.mu.RLock()
		handlers := am.handlers
		am.mu.RUnlock()

		for _, handler := range handlers {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := handler.Handle(ctx, alert); err != nil {
				logger.Error("Failed to handle alert",
					logger.String("rule", alert.Rule.Name),
					logger.ErrorField(err),
				)
			}
			cancel()
		}
	}
}

// LoggingAlertHandler 日志告警处理器
type LoggingAlertHandler struct{}

func (h *LoggingAlertHandler) Handle(ctx context.Context, alert Alert) error {
	logLevel := logger.Info
	switch alert.Rule.Severity {
	case "critical":
		logLevel = logger.Error
	case "warning":
		logLevel = logger.Warn
	}

	logger.Log(logLevel, "Alert triggered",
		logger.String("rule", alert.Rule.Name),
		logger.String("severity", alert.Rule.Severity),
		logger.Float64("value", alert.Value),
		logger.String("message", alert.Annotations["message"]),
		logger.String("summary", alert.Annotations["summary"]),
	)

	return nil
}
