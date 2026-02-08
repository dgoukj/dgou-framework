# Dgou Framework - 生产级 Go Gin 脚手架
## 9. 监控组件 (pkg/monitor)

### 特性
- ✅ **Prometheus指标收集**：完整的HTTP、数据库、缓存、业务指标
- ✅ **分布式追踪**：OpenTelemetry集成，支持Jaeger导出
- ✅ **告警规则管理**：YAML配置，支持Prometheus告警规则
- ✅ **性能分析工具**：pprof集成，内存、CPU、协程分析
- ✅ **健康检查系统**：多维度健康检查，支持自定义检查器
- ✅ **运行时监控**：自动收集Go运行时指标
- ✅ **中间件支持**：监控中间件，追踪中间件，恢复中间件
- ✅ **高性能**：零锁设计，异步处理，内存高效
- ✅ **生产就绪**：完善的错误处理，优雅关闭，安全防护

### 快速开始

#### 1. 基本配置

```yaml
# config/config.yaml
monitor:
  service_name: "myapp"
  service_version: "1.0.0"
  environment: "production"

  # Prometheus配置
  enable_metrics: true
  metrics_path: /metrics

  # 健康检查配置
  enable_health: true
  health_path: /health

  # 性能分析配置
  enable_profiling: false  # 生产环境建议关闭
  profile_path: /debug/pprof

  # 分布式追踪配置
  enable_tracing: true
  jaeger_endpoint: "http://jaeger:14268/api/traces"
  trace_sampling_rate: 0.1

  # 告警配置
  enable_alerts: true
  alert_rules: "./config/alert_rules.yaml"

  # 性能阈值配置
  performance_thresholds:
    max_request_duration: 2.0
    max_memory_usage: 0.8
    max_db_query_duration: 1.0
    max_goroutines: 1000
    error_rate_threshold: 0.05
```

#### 2. 初始化监控

```go
import (
    "dgou/pkg/app"
    "dgou/pkg/config"
    "dgou/pkg/monitor"
)

func main() {
    // 加载配置
    cfg := config.LoadConfig()
    
    // 创建应用
    app := app.NewApp(cfg)
    
    // 初始化监控
    monitor, err := monitor.InitMonitor(cfg.Monitor, app.GetEngine())
    if err != nil {
        log.Fatal(err)
    }
    defer monitor.Stop()
    
    // 使用监控中间件
    app.GetEngine().Use(monitor.MetricsMiddleware())
    app.GetEngine().Use(monitor.TracingMiddleware())
    app.GetEngine().Use(monitor.RecoveryMiddleware())
    
    // 启动应用
    app.Run()
}
```

#### 3. 指标收集示例

```go
// HTTP请求自动收集（通过中间件）
// 无需额外代码

// 数据库查询指标
func getUserByID(id int) (*User, error) {
    start := time.Now()
    
    var user User
    err := database.Master().First(&user, id).Error
    
    duration := time.Since(start)
    monitor.RecordDBQuery("mysql", "users", "select", duration, err == nil)
    
    return &user, err
}
    
// 业务事件指标
func processOrder(order *Order) error {
    start := time.Now()
    
    // 处理订单逻辑
    err := doProcessOrder(order)
    
    duration := time.Since(start)
    status := "success"
    if err != nil {
        status = "error"
    }

    monitor.RecordBusinessEvent("order_processing", status)
    monitor.RecordBusinessProcessing("order_process", status, duration)
    
    return err
}
    
// 自定义指标
func registerCustomMetrics(monitor *monitor.Monitor) error {
    // 创建自定义计数器
    customCounter := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "custom_events_total",
        Help: "Total custom events",
    })
    return monitor.RegisterCustomMetric("custom_events", customCounter)
}
```

#### 4. 分布式追踪

```go
import (
    "context"
    "go.opentelemetry.io/otel"
)

func processWithTracing(ctx context.Context) error {
    tracer := otel.Tracer("business")

    ctx, span := tracer.Start(ctx, "process_business")
    defer span.End()

    // 执行业务逻辑
    result, err := doBusiness(ctx)

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }

    return err
}
```

#### 5. 健康检查

```go
// 自定义健康检查
type DatabaseHealthCheck struct {
    db *gorm.DB
}

func (c *DatabaseHealthCheck) Name() string {
    return "database"
}

func (c *DatabaseHealthCheck) Check(ctx context.Context) monitor.HealthStatus {
    var result int
    err := c.db.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error

    status := "healthy"
    message := "Database is healthy"

    if err != nil {
        status = "unhealthy"
        message = fmt.Sprintf("Database connection failed: %v", err)
    }

    return monitor.HealthStatus{
        Name:    c.Name(),
        Status:  status,
        Message: message,
    }
}

// 注册健康检查 
monitor.RegisterHealthCheck(&DatabaseHealthCheck{db: database.Master()})
```

#### 6. 性能分析

```go
// 手动触发性能分析 
func triggerProfiling() {
    profiler := monitor.NewProfiler(monitor.GetMonitor())

    // 开始CPU分析（30秒）
    cpuFile, _ := profiler.StartCPUProfile(30 * time.Second)

    // 捕获堆内存快照
    heapFile, _ := profiler.CaptureHeapProfile()

    // 获取性能报告
    report := profiler.PerformanceReport()

    logger.Info("Profiling completed",
        logger.String("cpu_profile", cpuFile),
        logger.String("heap_profile", heapFile),
        logger.Any("report", report),
    )
}

// 通过HTTP端点访问
// GET /debug/pprof/          # pprof首页
// GET /debug/pprof/heap      # 堆内存分析
// GET /debug/pprof/profile   # CPU分析（30秒）
// GET /debug/pprof/goroutine # 协程分析
// GET /debug/pprof/trace     # 追踪分析（5秒）
```

#### 7. 告警管理

```yaml
# config/alert_rules.yaml
groups:
  - name: application_alerts
    rules:
      - alert: HighHttpErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High HTTP error rate detected"
          description: "HTTP error rate is {{ printf \"%.2f\" $value }}%"

      - alert: HighRequestLatency
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High request latency detected"
          description: "95th percentile request latency is {{ printf \"%.2f\" $value }}s"
```

#### 8. 监控端点

```bash
# Prometheus指标
GET /metrics

# 健康检查
GET /health
GET /health?detailed=true

# 性能分析（需开启）
GET /debug/pprof/
GET /debug/pprof/heap
GET /debug/pprof/profile?seconds=30

# 调试信息
GET /debug/vars
GET /debug/metrics
```

## 高级特性

### 1. 数据库监控集成

```go
// 包装GORM数据库
db := database.Master()
monitor := monitor.GetMonitor()

gormMonitor := monitor.NewGormMonitor(monitor)
monitoredDB := gormMonitor.WrapDB(db)

// 自动收集所有查询指标
// 更新连接池统计
gormMonitor.UpdateDBStats(db)
```

### 2. 缓存监控集成

```go
monitor := monitor.GetMonitor()
cacheMonitor := monitor.NewCacheMonitor(monitor)

// 在缓存操作前后记录指标
start := time.Now()
value, err := cache.Get(ctx, "key")
duration := time.Since(start)

cacheMonitor.RecordGet(ctx, "redis", duration, err == nil && value != nil)
```

### 3. 自定义告警处理器

```go
type SlackAlertHandler struct {
    webhookURL string
}

func (h *SlackAlertHandler) Handle(ctx context.Context, alert monitor.Alert) error {
    message := fmt.Sprintf("Alert: %s (Severity: %s)\nValue: %f\nMessage: %s",
        alert.Rule.Name,
        alert.Rule.Severity,
        alert.Value,
        alert.Annotations["message"],
    )

    // 发送到Slack
    return sendSlackMessage(h.webhookURL, message)
}

// 注册告警处理器
alertManager := monitor.GetMonitor().AlertManager()
alertManager.AddHandler(&SlackAlertHandler{
    webhookURL: "https://hooks.slack.com/services/...",
})
```

### 4. 性能阈值检查

```go
func checkPerformanceThresholds(monitor *monitor.Monitor) {
    metrics := monitor.GetMetrics()
    
    // 检查内存使用率
    if memUsage := metrics["memory"].(map[string]interface{})["usage"].(float64);
    memUsage > 0.8 {
        logger.Warn("High memory usage detected",
            logger.Float64("usage", memUsage),
        )
    }
    
    // 检查错误率
    if errorRate := metrics["error_rate"].(float64);
    errorRate > 0.05 {
        logger.Error("High error rate detected",
            logger.Float64("rate", errorRate),
        )
    }
}
```

## 最佳实践

### 1. 生产环境配置建议

```yaml
monitor:
  enable_metrics: true      # 始终启用指标
  enable_health: true       # 始终启用健康检查
  enable_profiling: false   # 生产环境关闭性能分析
  enable_tracing: true      # 根据需求开启追踪
  enable_alerts: true       # 始终启用告警

  # 安全建议
  profile_path: /internal/debug/pprof  # 使用非标准路径
  metrics_path: /internal/metrics      # 使用非标准路径
```

### 2. 指标标签设计

```go
// 好的标签设计
labels := []string{
    "method",      // HTTP方法
    "path",        // 请求路径
    "status",      // HTTP状态码
    "handler",     // 处理函数
    "service",     // 服务名称
    "version",     // 服务版本
    "environment", // 环境
}

// 避免标签爆炸
// 不要使用用户ID、会话ID等高基数标签
```

### 3. 告警策略

```yaml
# 告警级别定义
# critical - 需要立即处理，服务不可用
# warning  - 需要关注，可能影响性能
# info     - 信息性告警，无需立即处理

# 告警抑制规则
# 避免重复告警，设置合适的for持续时间
# 使用标签分组，相关告警一起通知
```

### 4. 性能优化

```go
// 使用异步处理
go func() {
    monitor.RecordBusinessEvent("async_event", "processed")
}()

// 批量记录指标
func batchRecordMetrics(events []BusinessEvent) {
    for _, event := range events {
        monitor.RecordBusinessEvent(event.Type, event.Status)
    }
}

// 避免在热点路径中创建大量标签
```

## 故障排除

### 1. 指标不显示

```bash
# 检查Prometheus配置
scrape_configs:
  - job_name: 'myapp'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'

# 检查服务端点
curl http://localhost:8080/metrics
curl http://localhost:8080/health
```

### 2. 内存泄漏检测

```go
// 定期检查内存使用
func checkMemoryLeaks() {
    profiler := monitor.NewProfiler(monitor.GetMonitor())
    
    // 定期捕获堆内存
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        heapFile, _ := profiler.CaptureHeapProfile()
        logger.Info("Heap profile captured",
            logger.String("file", heapFile),
        )
    }
}
```

### 3. 高延迟问题

```bash
# 使用pprof分析
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# 火焰图生成
go tool pprof -http=:8081 profile.pprof
```

### 4. 追踪数据缺失

```yaml
# 检查Jaeger配置
jaeger:
  endpoint: "http://jaeger:14268/api/traces"
  sampler_type: "probabilistic"
  sampler_param: 0.1  # 10%采样率

# 检查网络连接
telnet jaeger 14268
```

这个监控组件提供了完整的生产级监控解决方案，支持指标收集、分布式追踪、告警管理和性能分析。组件设计为模块化，可以按需启用各个功能，并且与现有组件完美集成。