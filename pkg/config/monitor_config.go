// pkg/config/monitor.go
package config

// MonitorConfig 监控配置
type MonitorConfig struct {
	EnableMetrics     bool    `mapstructure:"enable_metrics"`      // 是否启用指标监控
	MetricsPath       string  `mapstructure:"metrics_path"`        // 指标路径
	EnableHealth      bool    `mapstructure:"enable_health"`       // 是否启用健康检查
	HealthPath        string  `mapstructure:"health_path"`         // 健康检查路径
	EnableProfiling   bool    `mapstructure:"enable_profiling"`    // 是否启用性能分析
	ProfilePath       string  `mapstructure:"profile_path"`        // 性能分析路径
	ServiceName       string  `mapstructure:"service_name"`        // 服务名称
	ServiceVersion    string  `mapstructure:"service_version"`     // 服务版本
	Environment       string  `mapstructure:"environment"`         // 环境
	EnableTracing     bool    `mapstructure:"enable_tracing"`      // 是否启用分布式追踪
	JaegerEndpoint    string  `mapstructure:"jaeger_endpoint"`     // Jaeger端点
	TraceSamplingRate float64 `mapstructure:"trace_sampling_rate"` // 追踪采样率
	EnableAlerts      bool    `mapstructure:"enable_alerts"`       // 是否启用告警
	AlertWebhookURL   string  `mapstructure:"alert_webhook_url"`   // 告警Webhook URL
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		EnableMetrics:     true,
		MetricsPath:       "/metrics",
		EnableHealth:      true,
		HealthPath:        "/health",
		EnableProfiling:   false,
		ProfilePath:       "/debug/pprof",
		ServiceName:       "dgou-app",
		ServiceVersion:    "1.0.0",
		Environment:       "development",
		EnableTracing:     false,
		JaegerEndpoint:    "http://localhost:14268/api/traces",
		TraceSamplingRate: 0.1,
		EnableAlerts:      false,
	}
}
