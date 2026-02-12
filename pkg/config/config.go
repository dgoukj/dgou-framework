package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	MySQL     MySQLConfigExt  `mapstructure:"mysql"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Upload    UploadConfig    `mapstructure:"upload"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	OSS       OSSConfig       `mapstructure:"oss"`
	Log       LogConfig       `mapstructure:"log"`
	Monitor   MonitorConfig   `mapstructure:"monitor"`
	Security  SecurityConfig  `mapstructure:"security"`
	Async     AsyncConfig     `mapstructure:"async"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

type ServerConfig struct {
	Port            int    `mapstructure:"port"`
	Mode            string `mapstructure:"mode"`
	ReadTimeout     int    `mapstructure:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
	EnableHTTPS     bool   `mapstructure:"enable_https"`
	CertFile        string `mapstructure:"cert_file"`
	KeyFile         string `mapstructure:"key_file"`
	EnableGzip      bool   `mapstructure:"enable_gzip"`
	MaxRequestBody  int64  `mapstructure:"max_request_body"`
	EnablePprof     bool   `mapstructure:"enable_pprof"`
	PprofPort       int    `mapstructure:"pprof_port"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	Charset      string `mapstructure:"charset"`
	ParseTime    bool   `mapstructure:"parse_time"`
	Loc          string `mapstructure:"loc"`
}

type MySQLConfigExt struct {
	Master MySQLConfig   `mapstructure:"master"`
	Slaves []MySQLConfig `mapstructure:"slaves"`
	Pool   struct {
		MaxOpenConns    int `mapstructure:"max_open_conns"`
		MaxIdleConns    int `mapstructure:"max_idle_conns"`
		ConnMaxLifetime int `mapstructure:"conn_max_lifetime"`
		ConnMaxIdleTime int `mapstructure:"conn_max_idle_time"`
	} `mapstructure:"pool"`
	Log struct {
		SlowThreshold int    `mapstructure:"slow_threshold"`
		EnableLogging bool   `mapstructure:"enable_logging"`
		LogLevel      string `mapstructure:"log_level"`
	} `mapstructure:"log"`
	Performance struct {
		PrepareStmt       bool `mapstructure:"prepare_stmt"`
		DisableForeignKey bool `mapstructure:"disable_foreign_key"`
	} `mapstructure:"performance"`
}

type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	MaxRetries   int    `mapstructure:"max_retries"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`
}

// QueueConfig 队列配置（扩展 RabbitMQ 支持）
type QueueConfig struct {
	// 驱动类型：rabbitmq, redis（预留）
	Driver string `mapstructure:"driver"`

	// RabbitMQ 专用配置
	RabbitMQ struct {
		URL               string `mapstructure:"url"`                // AMQP URL，如 amqp://guest:guest@localhost:5672/
		Host              string `mapstructure:"host"`               // 主机（与URL二选一）
		Port              int    `mapstructure:"port"`               // 端口
		Username          string `mapstructure:"username"`           // 用户名
		Password          string `mapstructure:"password"`           // 密码
		Vhost             string `mapstructure:"vhost"`              // 虚拟主机
		ExchangeName      string `mapstructure:"exchange_name"`      // 默认交换机名称
		ExchangeType      string `mapstructure:"exchange_type"`      // 交换机类型：direct, topic, fanout, headers
		QueueName         string `mapstructure:"queue_name"`         // 默认队列名称
		RoutingKey        string `mapstructure:"routing_key"`        // 默认路由键
		Durable           bool   `mapstructure:"durable"`            // 持久化
		AutoDelete        bool   `mapstructure:"auto_delete"`        // 自动删除
		Exclusive         bool   `mapstructure:"exclusive"`          // 排他队列
		NoWait            bool   `mapstructure:"no_wait"`            // 非等待
		PrefetchCount     int    `mapstructure:"prefetch_count"`     // 消费者预取数量
		PrefetchSize      int    `mapstructure:"prefetch_size"`      // 消费者预取大小
		GlobalPrefetch    bool   `mapstructure:"global_prefetch"`    // 全局预取
		Heartbeat         int    `mapstructure:"heartbeat"`          // 心跳间隔（秒）
		ConnectionTimeout int    `mapstructure:"connection_timeout"` // 连接超时（秒）
	} `mapstructure:"rabbitmq"`

	// 原有字段（兼容）
	Broker     string `mapstructure:"broker"`
	BufferSize int    `mapstructure:"buffer_size"`
	WorkerNum  int    `mapstructure:"worker_num"`
	RetryTimes int    `mapstructure:"retry_times"`
}

type UploadConfig struct {
	StorageType       string        `mapstructure:"storage_type"`
	BasePath          string        `mapstructure:"base_path"`
	BaseURL           string        `mapstructure:"base_url"`
	MaxFileSize       int64         `mapstructure:"max_file_size"`
	AllowedTypes      []string      `mapstructure:"allowed_types"`
	AllowedMimeTypes  []string      `mapstructure:"allowed_mime_types"`
	AllowedExtensions []string      `mapstructure:"allowed_extensions"`
	ChunkEnabled      bool          `mapstructure:"chunk_enabled"`
	ChunkSize         int64         `mapstructure:"chunk_size"`
	MaxChunks         int           `mapstructure:"max_chunks"`
	TempDir           string        `mapstructure:"temp_dir"`
	CleanupInterval   time.Duration `mapstructure:"cleanup_interval"`
	MaxTempFileAge    time.Duration `mapstructure:"max_temp_file_age"`
	EnableResumable   bool          `mapstructure:"enable_resumable"`
	EnableVirusScan   bool          `mapstructure:"enable_virus_scan"`
	EnableCDN         bool          `mapstructure:"enable_cdn"`
	CDNURL            string        `mapstructure:"cdn_url"`
	AccessKeyID       string        `mapstructure:"access_key_id"`
	AccessKeySecret   string        `mapstructure:"access_key_secret"`
	Endpoint          string        `mapstructure:"endpoint"`
	Bucket            string        `mapstructure:"bucket"`
	Region            string        `mapstructure:"region"`
	UseHTTPS          bool          `mapstructure:"use_https"`
	MinFileSize       int64         `mapstructure:"min_file_size"`
	MaxFileNameLength int           `mapstructure:"max_file_name_length"`
	ValidateVirus     bool          `mapstructure:"validate_virus"`
	ScanTimeout       int           `mapstructure:"scan_timeout"`
}

type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	Issuer     string `mapstructure:"issuer"`
	Expire     int    `mapstructure:"expire_hours"`
	Refresh    int    `mapstructure:"refresh_hours"`
	Audience   string `mapstructure:"audience"`
	SigningKey string `mapstructure:"signing_key"`
	Algorithm  string `mapstructure:"algorithm"`
}

type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	BucketName      string `mapstructure:"bucket_name"`
	Domain          string `mapstructure:"domain"`
	Region          string `mapstructure:"region"`
	UseHTTPS        bool   `mapstructure:"use_https"`
	EnableCDN       bool   `mapstructure:"enable_cdn"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
	AddCaller  bool   `mapstructure:"add_caller"`
	AddStack   bool   `mapstructure:"add_stack"`
	JSONFormat bool   `mapstructure:"json_format"`
	LocalTime  bool   `mapstructure:"local_time"`
}

type MonitorConfig struct {
	EnableMetrics     bool    `mapstructure:"enable_metrics"`
	MetricsPath       string  `mapstructure:"metrics_path"`
	EnableHealth      bool    `mapstructure:"enable_health"`
	HealthPath        string  `mapstructure:"health_path"`
	EnableProfiling   bool    `mapstructure:"enable_profiling"`
	ProfilePath       string  `mapstructure:"profile_path"`
	ServiceName       string  `mapstructure:"service_name"`
	ServiceVersion    string  `mapstructure:"service_version"`
	Environment       string  `mapstructure:"environment"`
	EnableTracing     bool    `mapstructure:"enable_tracing"`
	JaegerEndpoint    string  `mapstructure:"jaeger_endpoint"`
	TraceSamplingRate float64 `mapstructure:"trace_sampling_rate"`
	EnableAlerts      bool    `mapstructure:"enable_alerts"`
	AlertWebhookURL   string  `mapstructure:"alert_webhook_url"`
}

type SecurityConfig struct {
	EnableRateLimit bool     `mapstructure:"enable_rate_limit"`
	RateLimit       int      `mapstructure:"rate_limit"`
	EnableCORS      bool     `mapstructure:"enable_cors"`
	EnableCSRF      bool     `mapstructure:"enable_csrf"`
	CSRFSecret      string   `mapstructure:"csrf_secret"`
	TrustedProxies  []string `mapstructure:"trusted_proxies"`
	EnableXSSFilter bool     `mapstructure:"enable_xss_filter"`
	EnableHSTS      bool     `mapstructure:"enable_hsts"`
	EnableCSP       bool     `mapstructure:"enable_csp"`
	AllowedOrigins  []string `mapstructure:"allowed_origins"`
	AllowedMethods  []string `mapstructure:"allowed_methods"`
}

type RateLimitConfig struct {
	EnableIPLimit    bool   `mapstructure:"enable_ip_limit"`
	EnableTokenLimit bool   `mapstructure:"enable_token_limit"`
	Limit            int    `mapstructure:"limit"`
	WindowSeconds    int    `mapstructure:"window_seconds"`
	RedisAddr        string `mapstructure:"redis_addr"`
	RedisPassword    string `mapstructure:"redis_password"`
	RedisDB          int    `mapstructure:"redis_db"`
	FailOpen         bool   `mapstructure:"fail_open"`
}

type AsyncConfig struct {
	MaxWorkers     int           `mapstructure:"max_workers"`
	MaxQueueSize   int           `mapstructure:"max_queue_size"`
	WorkerIdleTime time.Duration `mapstructure:"worker_idle_time"`
	EnableMetrics  bool          `mapstructure:"enable_metrics"`
	TaskRetries    int           `mapstructure:"task_retries"`
	TaskTimeout    time.Duration `mapstructure:"task_timeout"`
}

// Load 加载配置，无全局变量。
func Load(configPath ...string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if len(configPath) > 0 && configPath[0] != "" {
		v.SetConfigFile(configPath[0])
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./config")
		v.AddConfigPath(".")
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.read_timeout", 30)
	v.SetDefault("server.write_timeout", 30)
	v.SetDefault("server.shutdown_timeout", 10)
	v.SetDefault("server.enable_gzip", true)
	v.SetDefault("server.max_request_body", 10<<20)

	v.SetDefault("mysql.pool.max_open_conns", 100)
	v.SetDefault("mysql.pool.max_idle_conns", 10)
	v.SetDefault("mysql.pool.conn_max_lifetime", 3600)
	v.SetDefault("mysql.pool.conn_max_idle_time", 1800)
	v.SetDefault("mysql.log.slow_threshold", 200)
	v.SetDefault("mysql.log.enable_logging", true)
	v.SetDefault("mysql.log.log_level", "warn")
	v.SetDefault("mysql.performance.prepare_stmt", true)

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 100)
	v.SetDefault("redis.min_idle_conns", 10)
	v.SetDefault("redis.max_retries", 3)
	v.SetDefault("redis.dial_timeout", 5)
	v.SetDefault("redis.read_timeout", 3)
	v.SetDefault("redis.write_timeout", 3)
	v.SetDefault("redis.idle_timeout", 300)

	v.SetDefault("queue.driver", "rabbitmq")
	v.SetDefault("queue.rabbitmq.url", "amqp://guest:guest@localhost:5672/")
	v.SetDefault("queue.rabbitmq.exchange_name", "dgou.exchange")
	v.SetDefault("queue.rabbitmq.exchange_type", "direct")
	v.SetDefault("queue.rabbitmq.queue_name", "dgou.queue")
	v.SetDefault("queue.rabbitmq.routing_key", "dgou.routing")
	v.SetDefault("queue.rabbitmq.durable", true)
	v.SetDefault("queue.rabbitmq.prefetch_count", 1)
	v.SetDefault("queue.rabbitmq.heartbeat", 30)
	v.SetDefault("queue.rabbitmq.connection_timeout", 10)
	v.SetDefault("queue.rabbitmq.auto_delete", false) // 通常不自动删除
	v.SetDefault("queue.rabbitmq.exclusive", false)

	v.SetDefault("upload.storage_type", "local")
	v.SetDefault("upload.base_path", "./uploads")
	v.SetDefault("upload.base_url", "http://localhost:8080/uploads")
	v.SetDefault("upload.max_file_size", 10<<20)
	v.SetDefault("upload.allowed_types", []string{"image", "document"})
	v.SetDefault("upload.allowed_mime_types", []string{"image/jpeg", "image/png", "application/pdf"})
	v.SetDefault("upload.allowed_extensions", []string{".jpg", ".jpeg", ".png", ".pdf", ".doc", ".docx"})
	v.SetDefault("upload.chunk_enabled", true)
	v.SetDefault("upload.chunk_size", 5<<20)
	v.SetDefault("upload.max_chunks", 1000)
	v.SetDefault("upload.temp_dir", "/tmp/upload_chunks")
	v.SetDefault("upload.cleanup_interval", 30*time.Minute)
	v.SetDefault("upload.max_temp_file_age", 24*time.Hour)
	v.SetDefault("upload.max_file_name_length", 255)
	v.SetDefault("upload.scan_timeout", 30)

	v.SetDefault("jwt.expire_hours", 24)
	v.SetDefault("jwt.refresh_hours", 168)
	v.SetDefault("jwt.issuer", "dgou")
	v.SetDefault("jwt.audience", "dgou-client")
	v.SetDefault("jwt.algorithm", "HS256")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.file", "./logs/app.log")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 3)
	v.SetDefault("log.max_age", 7)
	v.SetDefault("log.compress", true)
	v.SetDefault("log.add_caller", true)
	v.SetDefault("log.add_stack", true)
	v.SetDefault("log.json_format", true)

	v.SetDefault("monitor.enable_metrics", true)
	v.SetDefault("monitor.metrics_path", "/metrics")
	v.SetDefault("monitor.enable_health", true)
	v.SetDefault("monitor.health_path", "/health")
	v.SetDefault("monitor.enable_profiling", false)
	v.SetDefault("monitor.profile_path", "/debug/pprof")
	v.SetDefault("monitor.service_name", "dgou-app")
	v.SetDefault("monitor.service_version", "1.0.0")
	v.SetDefault("monitor.environment", "development")
	v.SetDefault("monitor.trace_sampling_rate", 0.1)

	v.SetDefault("security.enable_rate_limit", true)
	v.SetDefault("security.rate_limit", 100)
	v.SetDefault("security.enable_cors", true)
	v.SetDefault("security.enable_xss_filter", true)
	v.SetDefault("security.enable_hsts", true)
	v.SetDefault("security.allowed_origins", []string{"*"})
	v.SetDefault("security.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})

	v.SetDefault("rate_limit.enable_ip_limit", true)
	v.SetDefault("rate_limit.limit", 100)
	v.SetDefault("rate_limit.window_seconds", 60)
	v.SetDefault("rate_limit.redis_addr", "localhost:6379")
	v.SetDefault("rate_limit.fail_open", true)

	v.SetDefault("async.max_workers", 100)
	v.SetDefault("async.max_queue_size", 10000)
	v.SetDefault("async.worker_idle_time", 30*time.Second)
	v.SetDefault("async.enable_metrics", true)
	v.SetDefault("async.task_retries", 3)
	v.SetDefault("async.task_timeout", 30*time.Second)
}

func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}
	if cfg.JWT.Secret != "" && len(cfg.JWT.Secret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters")
	}
	return nil
}

// WatchConfig 支持热加载，传入当前配置指针，更新其内容。
func WatchConfig(cfg *Config, configPath string) error {
	v := viper.New()
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		if err := v.Unmarshal(cfg); err != nil {
			// log error but cannot propagate
		}
	})
	return nil
}
