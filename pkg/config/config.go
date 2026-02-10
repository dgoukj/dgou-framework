package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	// AppConfig 全局配置实例
	AppConfig  *Config
	configOnce sync.Once
	configMu   sync.RWMutex

	// 配置验证器和关闭处理器
	configValidators   []func(*Config) error
	shutdownHandlers   []func() error
	validatorsMu       sync.RWMutex
	shutdownHandlersMu sync.RWMutex
)

// Config 全局配置结构体
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	MySQL     MySQLConfigExt  `mapstructure:"mysql"` // 使用扩展的MySQL配置
	Redis     RedisConfig     `mapstructure:"redis"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Upload    UploadConfig    `mapstructure:"upload"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	OSS       OSSConfig       `mapstructure:"oss"`
	RabbitMQ  RabbitMQConfig  `mapstructure:"rabbitmq"`
	Log       LogConfig       `mapstructure:"log"`
	Monitor   MonitorConfig   `mapstructure:"monitor"` // 使用新的MonitorConfig
	Security  SecurityConfig  `mapstructure:"security"`
	Async     AsyncConfig     `mapstructure:"async"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"` // 限流配置
}

// ServerConfig 服务配置
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
	EnablePprof     bool   `mapstructure:"enable_pprof"` // 是否启用性能分析
	PprofPort       int    `mapstructure:"pprof_port"`   // 性能分析端口
}

// MySQLConfig 基础MySQL配置（供扩展配置使用）
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

// MySQLConfigExt 扩展的MySQL配置（支持读写分离）
type MySQLConfigExt struct {
	// 主库配置
	Master MySQLConfig `mapstructure:"master"`

	// 从库配置列表
	Slaves []MySQLConfig `mapstructure:"slaves"`

	// 连接池配置
	Pool struct {
		MaxOpenConns    int `mapstructure:"max_open_conns"`     // 最大打开连接数
		MaxIdleConns    int `mapstructure:"max_idle_conns"`     // 最大空闲连接数
		ConnMaxLifetime int `mapstructure:"conn_max_lifetime"`  // 连接最大生命周期(秒)
		ConnMaxIdleTime int `mapstructure:"conn_max_idle_time"` // 连接最大空闲时间(秒)
	} `mapstructure:"pool"`

	// 日志配置
	Log struct {
		SlowThreshold int    `mapstructure:"slow_threshold"` // 慢查询阈值(毫秒)
		EnableLogging bool   `mapstructure:"enable_logging"` // 是否启用SQL日志
		LogLevel      string `mapstructure:"log_level"`      // 日志级别：silent, error, warn, info
	} `mapstructure:"log"`

	// 性能配置
	Performance struct {
		PrepareStmt       bool `mapstructure:"prepare_stmt"`        // 是否启用预编译语句
		DisableForeignKey bool `mapstructure:"disable_foreign_key"` // 是否禁用外键约束
	} `mapstructure:"performance"`
}

// GetMasterConfig 获取主库配置
func (c *MySQLConfigExt) GetMasterConfig() *MySQLConfig {
	return &c.Master
}

// GetSlaveConfigs 获取从库配置列表
func (c *MySQLConfigExt) GetSlaveConfigs() []MySQLConfig {
	return c.Slaves
}

// HasSlaves 检查是否有从库配置
func (c *MySQLConfigExt) HasSlaves() bool {
	return len(c.Slaves) > 0
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	MaxRetries   int    `mapstructure:"max_retries"`
	DialTimeout  int    `mapstructure:"dial_timeout"`  // 连接超时(秒)
	ReadTimeout  int    `mapstructure:"read_timeout"`  // 读取超时(秒)
	WriteTimeout int    `mapstructure:"write_timeout"` // 写入超时(秒)
	IdleTimeout  int    `mapstructure:"idle_timeout"`  // 空闲连接超时(秒)
}

// QueueConfig 队列配置
type QueueConfig struct {
	Broker     string `mapstructure:"broker"`
	BufferSize int    `mapstructure:"buffer_size"`
	WorkerNum  int    `mapstructure:"worker_num"`  // 工作协程数
	RetryTimes int    `mapstructure:"retry_times"` // 重试次数
}

// UploadConfig 上传配置
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

// JWTConfig JWT配置
type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	Issuer     string `mapstructure:"issuer"`
	Expire     int    `mapstructure:"expire_hours"`  // 过期时间(小时)
	Refresh    int    `mapstructure:"refresh_hours"` // 刷新时间(小时)
	Audience   string `mapstructure:"audience"`      // 受众
	SigningKey string `mapstructure:"signing_key"`   // 签名密钥
	Algorithm  string `mapstructure:"algorithm"`     // 签名算法
}

// OSSConfig OSS配置
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

// RabbitMQConfig RabbitMQ配置
type RabbitMQConfig struct {
	URL        string                 `mapstructure:"url"`
	Exchange   string                 `mapstructure:"exchange"`
	QueueName  string                 `mapstructure:"queue_name"`
	RoutingKey string                 `mapstructure:"routing_key"`
	Durable    bool                   `mapstructure:"durable"`
	AutoDelete bool                   `mapstructure:"auto_delete"`
	Exclusive  bool                   `mapstructure:"exclusive"`
	NoWait     bool                   `mapstructure:"no_wait"`
	Arguments  map[string]interface{} `mapstructure:"arguments"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
	AddCaller  bool   `mapstructure:"add_caller"`
	AddStack   bool   `mapstructure:"add_stack"`   // 是否添加堆栈信息
	JSONFormat bool   `mapstructure:"json_format"` // 是否使用JSON格式
	LocalTime  bool   `mapstructure:"local_time"`  // 是否使用本地时间
}

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

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableRateLimit bool     `mapstructure:"enable_rate_limit"`
	RateLimit       int      `mapstructure:"rate_limit"`
	EnableCORS      bool     `mapstructure:"enable_cors"`
	EnableCSRF      bool     `mapstructure:"enable_csrf"`
	CSRFSecret      string   `mapstructure:"csrf_secret"`
	TrustedProxies  []string `mapstructure:"trusted_proxies"`
	EnableXSSFilter bool     `mapstructure:"enable_xss_filter"`
	EnableHSTS      bool     `mapstructure:"enable_hsts"`
	EnableCSP       bool     `mapstructure:"enable_csp"`      // 内容安全策略
	AllowedOrigins  []string `mapstructure:"allowed_origins"` // 允许的源
	AllowedMethods  []string `mapstructure:"allowed_methods"` // 允许的方法
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	EnableIPLimit    bool   `mapstructure:"enable_ip_limit"`
	EnableTokenLimit bool   `mapstructure:"enable_token_limit"`
	Limit            int    `mapstructure:"limit"`
	WindowSeconds    int    `mapstructure:"window_seconds"`
	RedisAddr        string `mapstructure:"redis_addr"`
	RedisPassword    string `mapstructure:"redis_password"`
	RedisDB          int    `mapstructure:"redis_db"`
	FailOpen         bool   `mapstructure:"fail_open"` // Redis故障时是否允许请求通过
}

// AsyncConfig 异步配置
type AsyncConfig struct {
	MaxWorkers     int           `mapstructure:"max_workers"`
	MaxQueueSize   int           `mapstructure:"max_queue_size"`
	WorkerIdleTime time.Duration `mapstructure:"worker_idle_time"`
	EnableMetrics  bool          `mapstructure:"enable_metrics"`
	TaskRetries    int           `mapstructure:"task_retries"`
	TaskTimeout    time.Duration `mapstructure:"task_timeout"`
}

// LoadConfig 加载配置（线程安全单例模式）
func LoadConfig(configPath ...string) *Config {
	configOnce.Do(func() {
		// 创建默认配置
		defaultConfig := createDefaultConfig()

		// 初始化viper
		v := viper.New()

		// 设置配置文件路径
		if len(configPath) > 0 && configPath[0] != "" {
			v.SetConfigFile(configPath[0])
		} else {
			// 默认查找 config/config.yaml
			v.SetConfigName("config")
			v.SetConfigType("yaml")
			v.AddConfigPath("./config")
			v.AddConfigPath(".")
			v.AddConfigPath("/etc/app/")
		}

		// 读取环境变量
		v.AutomaticEnv()
		v.SetEnvPrefix("APP")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

		// 尝试读取配置文件
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				log.Printf("Config file not found, using defaults and environment variables")
			} else {
				log.Printf("Error reading config file: %v", err)
			}
		} else {
			log.Printf("Using config file: %s", v.ConfigFileUsed())
		}

		// 合并配置到结构体
		config := &Config{}
		if err := v.Unmarshal(config); err != nil {
			log.Fatalf("Unable to decode config: %v", err)
		}

		// 合并默认值（viper不会为未设置的字段设置默认值）
		mergeWithDefaults(config, defaultConfig)

		// 验证配置
		if err := validateConfig(config); err != nil {
			log.Fatalf("Config validation failed: %v", err)
		}

		// 执行注册的验证器
		if err := ValidateConfig(config); err != nil {
			log.Fatalf("Registered config validators failed: %v", err)
		}

		// 设置全局配置
		configMu.Lock()
		AppConfig = config
		configMu.Unlock()

		// 监听配置文件变化（热重载）
		v.WatchConfig()
		v.OnConfigChange(func(e fsnotify.Event) {
			log.Printf("Config file changed: %s", e.Name)

			newConfig := &Config{}
			if err := v.Unmarshal(newConfig); err != nil {
				log.Printf("Error reloading config: %v", err)
				return
			}

			// 验证新配置
			if err := validateConfig(newConfig); err != nil {
				log.Printf("New config validation failed: %v", err)
				return
			}

			// 执行注册的验证器
			if err := ValidateConfig(newConfig); err != nil {
				log.Printf("Registered config validators failed for new config: %v", err)
				return
			}

			// 更新全局配置（线程安全）
			configMu.Lock()
			AppConfig = newConfig
			configMu.Unlock()

			log.Printf("Config reloaded successfully")
		})

		log.Printf("Config loaded successfully")
	})

	return AppConfig
}

// GetConfig 获取当前配置（线程安全）
func GetConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return AppConfig
}

// createDefaultConfig 创建默认配置
func createDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8080,
			Mode:            "release",
			ReadTimeout:     30,
			WriteTimeout:    30,
			ShutdownTimeout: 10,
			EnableHTTPS:     false,
			EnableGzip:      true,
			MaxRequestBody:  10 << 20, // 10MB
			EnablePprof:     false,
			PprofPort:       6060,
		},
		MySQL: MySQLConfigExt{
			Master: MySQLConfig{
				Host:         "localhost",
				Port:         3306,
				User:         "root",
				Password:     "",
				DBName:       "dgou",
				MaxOpenConns: 100,
				MaxIdleConns: 10,
				Charset:      "utf8mb4",
				ParseTime:    true,
				Loc:          "Local",
			},
			Pool: struct {
				MaxOpenConns    int `mapstructure:"max_open_conns"`
				MaxIdleConns    int `mapstructure:"max_idle_conns"`
				ConnMaxLifetime int `mapstructure:"conn_max_lifetime"`
				ConnMaxIdleTime int `mapstructure:"conn_max_idle_time"`
			}{
				MaxOpenConns:    100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600, // 1小时
				ConnMaxIdleTime: 1800, // 30分钟
			},
			Log: struct {
				SlowThreshold int    `mapstructure:"slow_threshold"`
				EnableLogging bool   `mapstructure:"enable_logging"`
				LogLevel      string `mapstructure:"log_level"`
			}{
				SlowThreshold: 200, // 200毫秒
				EnableLogging: true,
				LogLevel:      "warn",
			},
			Performance: struct {
				PrepareStmt       bool `mapstructure:"prepare_stmt"`
				DisableForeignKey bool `mapstructure:"disable_foreign_key"`
			}{
				PrepareStmt:       true,
				DisableForeignKey: false,
			},
		},
		Redis: RedisConfig{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           0,
			PoolSize:     100,
			MinIdleConns: 10,
			MaxRetries:   3,
			DialTimeout:  5,
			ReadTimeout:  3,
			WriteTimeout: 3,
			IdleTimeout:  300,
		},
		Queue: QueueConfig{
			Broker:     "redis",
			BufferSize: 1000,
			WorkerNum:  10,
			RetryTimes: 3,
		},
		Upload: UploadConfig{
			StorageType:       "local",
			BasePath:          "./uploads",
			BaseURL:           "http://localhost:8080/uploads",
			MaxFileSize:       10 << 20, // 10MB
			AllowedTypes:      []string{"image", "document"},
			AllowedMimeTypes:  []string{"image/jpeg", "image/png", "application/pdf"},
			AllowedExtensions: []string{".jpg", ".jpeg", ".png", ".pdf", ".doc", ".docx"},
			ChunkEnabled:      true,
			ChunkSize:         5 << 20, // 5MB
			MaxChunks:         1000,
			TempDir:           "/tmp/upload_chunks",
			CleanupInterval:   30 * time.Minute,
			MaxTempFileAge:    24 * time.Hour,
			EnableResumable:   true,
			EnableVirusScan:   false,
			EnableCDN:         false,
			CDNURL:            "",
			AccessKeyID:       "",
			AccessKeySecret:   "",
			Endpoint:          "",
			Bucket:            "",
			Region:            "",
			UseHTTPS:          false,
			MinFileSize:       0,
			MaxFileNameLength: 255,
			ValidateVirus:     false,
			ScanTimeout:       30,
		},
		JWT: JWTConfig{
			Secret:     "your-secret-key-at-least-32-chars-long",
			Issuer:     "dgou",
			Expire:     24,  // 24小时
			Refresh:    168, // 7天
			Audience:   "dgou-client",
			SigningKey: "your-signing-key",
			Algorithm:  "HS256",
		},
		OSS: OSSConfig{
			Endpoint:        "",
			AccessKeyID:     "",
			AccessKeySecret: "",
			BucketName:      "",
			Domain:          "",
			Region:          "oss-cn-hangzhou",
			UseHTTPS:        true,
			EnableCDN:       false,
		},
		RabbitMQ: RabbitMQConfig{
			URL:        "amqp://guest:guest@localhost:5672/",
			Exchange:   "dgou_exchange",
			QueueName:  "dgou_queue",
			RoutingKey: "dgou_routing_key",
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Arguments:  nil,
		},
		Log: LogConfig{
			Level:      "info",
			File:       "./logs/app.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
			AddCaller:  true,
			AddStack:   true,
			JSONFormat: true,
			LocalTime:  false,
		},
		Monitor: MonitorConfig{
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
			AlertWebhookURL:   "",
		},
		Security: SecurityConfig{
			EnableRateLimit: true,
			RateLimit:       100,
			EnableCORS:      true,
			EnableCSRF:      false,
			EnableXSSFilter: true,
			EnableHSTS:      true,
			EnableCSP:       false,
			AllowedOrigins:  []string{"*"},
			AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		},
		RateLimit: RateLimitConfig{
			EnableIPLimit:    true,
			EnableTokenLimit: false,
			Limit:            100,
			WindowSeconds:    60,
			RedisAddr:        "localhost:6379",
			RedisPassword:    "",
			RedisDB:          0,
			FailOpen:         true,
		},
		Async: AsyncConfig{
			MaxWorkers:     100,
			MaxQueueSize:   10000,
			WorkerIdleTime: 30 * time.Second,
			EnableMetrics:  true,
			TaskRetries:    3,
			TaskTimeout:    30 * time.Second,
		},
	}
}

// mergeWithDefaults 合并默认值
func mergeWithDefaults(config, defaults *Config) {
	// 如果端口为0，使用默认端口
	if config.Server.Port == 0 {
		config.Server.Port = defaults.Server.Port
	}

	if config.Server.Mode == "" {
		config.Server.Mode = defaults.Server.Mode
	}

	// JWT密钥长度检查
	if config.JWT.Secret == "" || len(config.JWT.Secret) < 32 {
		config.JWT.Secret = defaults.JWT.Secret
		log.Printf("Warning: JWT secret is too short, using default")
	}

	// MySQL配置合并
	if config.MySQL.Master.Host == "" {
		config.MySQL.Master = defaults.MySQL.Master
	}

	if config.MySQL.Pool.MaxOpenConns == 0 {
		config.MySQL.Pool = defaults.MySQL.Pool
	}

	if config.MySQL.Log.SlowThreshold == 0 {
		config.MySQL.Log = defaults.MySQL.Log
	}

	// 上传配置合并
	if config.Upload.StorageType == "" {
		config.Upload.StorageType = defaults.Upload.StorageType
	}
	if config.Upload.BasePath == "" {
		config.Upload.BasePath = defaults.Upload.BasePath
	}
	if config.Upload.BaseURL == "" {
		config.Upload.BaseURL = defaults.Upload.BaseURL
	}
	if config.Upload.MaxFileSize == 0 {
		config.Upload.MaxFileSize = defaults.Upload.MaxFileSize
	}
	if len(config.Upload.AllowedTypes) == 0 {
		config.Upload.AllowedTypes = defaults.Upload.AllowedTypes
	}
	if len(config.Upload.AllowedMimeTypes) == 0 {
		config.Upload.AllowedMimeTypes = defaults.Upload.AllowedMimeTypes
	}
	if len(config.Upload.AllowedExtensions) == 0 {
		config.Upload.AllowedExtensions = defaults.Upload.AllowedExtensions
	}
	if config.Upload.ChunkSize == 0 {
		config.Upload.ChunkSize = defaults.Upload.ChunkSize
	}
	if config.Upload.TempDir == "" {
		config.Upload.TempDir = defaults.Upload.TempDir
	}
	if config.Upload.CleanupInterval == 0 {
		config.Upload.CleanupInterval = defaults.Upload.CleanupInterval
	}
	if config.Upload.MaxTempFileAge == 0 {
		config.Upload.MaxTempFileAge = defaults.Upload.MaxTempFileAge
	}

	// 监控配置合并
	if config.Monitor.ServiceName == "" {
		config.Monitor = defaults.Monitor
	}
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.MySQL.Master.Host == "" {
		return fmt.Errorf("mysql host is required")
	}

	if config.Redis.Addr == "" {
		return fmt.Errorf("redis address is required")
	}

	if len(config.JWT.Secret) < 32 {
		return fmt.Errorf("jwt secret must be at least 32 characters")
	}

	if config.Server.EnableHTTPS {
		if config.Server.CertFile == "" || config.Server.KeyFile == "" {
			return fmt.Errorf("cert_file and key_file are required when HTTPS is enabled")
		}

		// 检查证书文件是否存在
		if _, err := os.Stat(config.Server.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("certificate file not found: %s", config.Server.CertFile)
		}
		if _, err := os.Stat(config.Server.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("key file not found: %s", config.Server.KeyFile)
		}
	}

	// 验证上传配置
	if config.Upload.StorageType == "" {
		return fmt.Errorf("upload storage type is required")
	}

	if config.Upload.StorageType == "local" && config.Upload.BasePath == "" {
		return fmt.Errorf("base_path is required for local storage")
	}

	if config.Upload.StorageType == "oss" {
		if config.Upload.AccessKeyID == "" || config.Upload.AccessKeySecret == "" {
			return fmt.Errorf("access_key_id and access_key_secret are required for OSS storage")
		}
		if config.Upload.Endpoint == "" {
			return fmt.Errorf("endpoint is required for OSS storage")
		}
		if config.Upload.Bucket == "" {
			return fmt.Errorf("bucket is required for OSS storage")
		}
	}

	if config.Upload.MaxFileSize <= 0 {
		return fmt.Errorf("max_file_size must be greater than 0")
	}

	// 验证监控配置
	if config.Monitor.EnableMetrics && config.Monitor.MetricsPath == "" {
		return fmt.Errorf("metrics_path is required when enable_metrics is true")
	}

	if config.Monitor.EnableHealth && config.Monitor.HealthPath == "" {
		return fmt.Errorf("health_path is required when enable_health is true")
	}

	return nil
}

// GetEnv 获取环境变量，优先使用环境变量，其次使用配置值
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ==================== 配置验证器和关闭处理器 ====================

// RegisterValidator 注册配置验证器（公共函数）
func RegisterValidator(fn func(*Config) error) {
	validatorsMu.Lock()
	defer validatorsMu.Unlock()
	configValidators = append(configValidators, fn)
}

// RegisterShutdownHandler 注册优雅关闭处理器（公共函数）
func RegisterShutdownHandler(fn func() error) {
	shutdownHandlersMu.Lock()
	defer shutdownHandlersMu.Unlock()
	shutdownHandlers = append(shutdownHandlers, fn)
}

// ValidateConfig 执行所有注册的配置验证器
func ValidateConfig(cfg *Config) error {
	validatorsMu.RLock()
	validators := make([]func(*Config) error, len(configValidators))
	copy(validators, configValidators)
	validatorsMu.RUnlock()

	var errs []error
	for _, validator := range validators {
		if err := validator(cfg); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors: %v", errs)
	}

	return nil
}

// ExecuteShutdownHandlers 执行所有关闭处理器
func ExecuteShutdownHandlers() error {
	shutdownHandlersMu.RLock()
	handlers := make([]func() error, len(shutdownHandlers))
	copy(handlers, shutdownHandlers)
	shutdownHandlersMu.RUnlock()

	var errs []error
	for _, handler := range handlers {
		if err := handler(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// Shutdown 执行优雅关闭
func Shutdown() error {
	return ExecuteShutdownHandlers()
}
