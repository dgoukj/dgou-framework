// pkg/config/monitor.go
package config

// MonitorConfig 监控配置
type MonitorConfig struct {
	// 基础配置
	ServiceName    string `mapstructure:"service_name" yaml:"service_name"`
	ServiceVersion string `mapstructure:"service_version" yaml:"service_version"`
	Environment    string `mapstructure:"environment" yaml:"environment"`

	// Prometheus配置
	EnableMetrics bool   `mapstructure:"enable_metrics" yaml:"enable_metrics"`
	MetricsPath   string `mapstructure:"metrics_path" yaml:"metrics_path"`

	// 健康检查配置
	EnableHealth bool   `mapstructure:"enable_health" yaml:"enable_health"`
	HealthPath   string `mapstructure:"health_path" yaml:"health_path"`

	// 性能分析配置
	EnableProfiling bool   `mapstructure:"enable_profiling" yaml:"enable_profiling"`
	ProfilePath     string `mapstructure:"profile_path" yaml:"profile_path"`

	// 分布式追踪配置
	EnableTracing     bool    `mapstructure:"enable_tracing" yaml:"enable_tracing"`
	JaegerEndpoint    string  `mapstructure:"jaeger_endpoint" yaml:"jaeger_endpoint"`
	TraceSamplingRate float64 `mapstructure:"trace_sampling_rate" yaml:"trace_sampling_rate"`

	// 告警配置
	EnableAlerts bool   `mapstructure:"enable_alerts" yaml:"enable_alerts"`
	AlertRules   string `mapstructure:"alert_rules" yaml:"alert_rules"`

	// 性能阈值配置
	PerformanceThresholds struct {
		MaxRequestDuration float64 `mapstructure:"max_request_duration" yaml:"max_request_duration"`
		MaxMemoryUsage     float64 `mapstructure:"max_memory_usage" yaml:"max_memory_usage"`
		MaxDBQueryDuration float64 `mapstructure:"max_db_query_duration" yaml:"max_db_query_duration"`
		MaxGoroutines      int     `mapstructure:"max_goroutines" yaml:"max_goroutines"`
		ErrorRateThreshold float64 `mapstructure:"error_rate_threshold" yaml:"error_rate_threshold"`
	} `mapstructure:"performance_thresholds" yaml:"performance_thresholds"`
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		ServiceName:       "dgou-app",
		ServiceVersion:    "1.0.0",
		Environment:       "production",
		EnableMetrics:     true,
		MetricsPath:       "/metrics",
		EnableHealth:      true,
		HealthPath:        "/health",
		EnableProfiling:   false,
		ProfilePath:       "/debug/pprof",
		EnableTracing:     false,
		JaegerEndpoint:    "http://localhost:14268/api/traces",
		TraceSamplingRate: 0.1,
		EnableAlerts:      true,
		AlertRules:        "./config/alert_rules.yaml",
	}
}
