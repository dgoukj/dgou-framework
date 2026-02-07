package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	// AppConfig 全局配置实例
	AppConfig  *Config
	configOnce sync.Once
	configMu   sync.RWMutex
)

// Config 全局配置结构体
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Queue    QueueConfig    `mapstructure:"queue"`
	Upload   UploadConfig   `mapstructure:"upload"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	OSS      OSSConfig      `mapstructure:"oss"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Log      LogConfig      `mapstructure:"log"`
	Monitor  MonitorConfig  `mapstructure:"monitor"`
	Security SecurityConfig `mapstructure:"security"`
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
}

// MySQLConfig MySQL配置
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

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	MaxRetries   int    `mapstructure:"max_retries"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	Broker     string `mapstructure:"broker"`
	BufferSize int    `mapstructure:"buffer_size"`
}

// UploadConfig 上传配置
type UploadConfig struct {
	Type      UploadType `mapstructure:"type"`
	LocalPath string     `mapstructure:"local_path"`
	MaxSize   int64      `mapstructure:"max_size"`
	AllowExts []string   `mapstructure:"allow_exts"`
}

// UploadType 上传类型
type UploadType string

const (
	Local UploadType = "local"
	OSS   UploadType = "oss"
)

// JWTConfig JWT配置
type JWTConfig struct {
	Secret   string `mapstructure:"secret"`
	Issuer   string `mapstructure:"issuer"`
	Expire   int    `mapstructure:"expire_hours"`
	Refresh  int    `mapstructure:"refresh_hours"`
	Audience string `mapstructure:"audience"`
}

// OSSConfig OSS配置
type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	BucketName      string `mapstructure:"bucket_name"`
	Domain          string `mapstructure:"domain"`
}

// RabbitMQConfig RabbitMQ配置
type RabbitMQConfig struct {
	URL      string `mapstructure:"url"`
	Exchange string `mapstructure:"exchange"`
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
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	EnableMetrics   bool   `mapstructure:"enable_metrics"`
	MetricsPath     string `mapstructure:"metrics_path"`
	EnableHealth    bool   `mapstructure:"enable_health"`
	HealthPath      string `mapstructure:"health_path"`
	EnableProfiling bool   `mapstructure:"enable_profiling"`
	ProfilePath     string `mapstructure:"profile_path"`
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
		},
		MySQL: MySQLConfig{
			Host:         "localhost",
			Port:         3306,
			User:         "root",
			Password:     "",
			DBName:       "test",
			MaxOpenConns: 100,
			MaxIdleConns: 10,
			Charset:      "utf8mb4",
			ParseTime:    true,
			Loc:          "Local",
		},
		Redis: RedisConfig{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           0,
			PoolSize:     100,
			MinIdleConns: 10,
			MaxRetries:   3,
		},
		Queue: QueueConfig{
			Broker:     "redis",
			BufferSize: 1000,
		},
		Upload: UploadConfig{
			Type:      Local,
			LocalPath: "./uploads",
			MaxSize:   10 << 20, // 10MB
			AllowExts: []string{".jpg", ".jpeg", ".png", ".gif", ".pdf"},
		},
		JWT: JWTConfig{
			Secret:   "your-secret-key-at-least-32-chars-long",
			Issuer:   "app",
			Expire:   24,
			Refresh:  168, // 7天
			Audience: "app-client",
		},
		Log: LogConfig{
			Level:      "info",
			File:       "./logs/app.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
			AddCaller:  true,
		},
		Monitor: MonitorConfig{
			EnableMetrics:   true,
			MetricsPath:     "/metrics",
			EnableHealth:    true,
			HealthPath:      "/health",
			EnableProfiling: false,
			ProfilePath:     "/debug/pprof",
		},
		Security: SecurityConfig{
			EnableRateLimit: true,
			RateLimit:       100,
			EnableCORS:      true,
			EnableCSRF:      false,
			EnableXSSFilter: true,
			EnableHSTS:      true,
		},
	}
}

// mergeWithDefaults 合并默认值
func mergeWithDefaults(config, defaults *Config) {
	// 这里可以使用反射或手动合并，为了简单起见，我们只处理关键字段
	// 实际项目中可以使用更完善的合并逻辑
	if config.Server.Port == 0 {
		config.Server.Port = defaults.Server.Port
	}
	if config.JWT.Secret == "" || len(config.JWT.Secret) < 32 {
		config.JWT.Secret = defaults.JWT.Secret
		log.Printf("Warning: JWT secret is too short, using default")
	}
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.MySQL.Host == "" {
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

	return nil
}

// GetEnv 获取环境变量，优先使用环境变量，其次使用配置值
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
