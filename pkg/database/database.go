package database

import (
	"context"
	"database/sql"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	// dbInstances 数据库实例缓存
	dbInstances = make(map[string]*DBInstance)
	// dbMutex 读写锁保护dbInstances
	dbMutex sync.RWMutex
	// defaultDBName 默认数据库名称
	defaultDBName = "default"
)

// DBConfig 数据库配置扩展（支持读写分离）
type DBConfig struct {
	Name            string `mapstructure:"name"`               // 实例名称
	DSN             string `mapstructure:"dsn"`                // 完整的DSN连接字符串
	Driver          string `mapstructure:"driver"`             // 数据库驱动，默认mysql
	Host            string `mapstructure:"host"`               // 主机地址
	Port            int    `mapstructure:"port"`               // 端口
	User            string `mapstructure:"user"`               // 用户名
	Password        string `mapstructure:"password"`           // 密码
	DBName          string `mapstructure:"dbname"`             // 数据库名
	Charset         string `mapstructure:"charset"`            // 字符集
	ParseTime       bool   `mapstructure:"parse_time"`         // 是否解析时间
	Loc             string `mapstructure:"loc"`                // 时区
	Role            string `mapstructure:"role"`               // 角色：master, slave
	MaxOpenConns    int    `mapstructure:"max_open_conns"`     // 最大打开连接数
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`     // 最大空闲连接数
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`  // 连接最大生命周期(秒)
	ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time"` // 连接最大空闲时间(秒)
	SlowThreshold   int    `mapstructure:"slow_threshold"`     // 慢查询阈值(毫秒)
	EnableLogging   bool   `mapstructure:"enable_logging"`     // 是否启用SQL日志
	LogLevel        string `mapstructure:"log_level"`          // 日志级别：silent, error, warn, info
}

// DBInstance 数据库实例
type DBInstance struct {
	Name        string    // 实例名称
	Config      *DBConfig // 配置
	DB          *gorm.DB  // GORM实例
	Role        string    // 角色：master, slave
	IsConnected bool      // 是否已连接
	lastPing    time.Time // 最后ping时间
}

// Database 数据库管理器
type Database struct {
	config       *config.Config         // 应用配置
	instances    map[string]*DBInstance // 数据库实例
	master       *DBInstance            // 主库实例
	slaves       []*DBInstance          // 从库实例
	currentSlave int                    // 当前使用的从库索引
	mu           sync.RWMutex           // 读写锁
}

// NewDatabase 创建新的数据库管理器
func NewDatabase(cfg *config.Config) *Database {
	return &Database{
		config:    cfg,
		instances: make(map[string]*DBInstance),
		slaves:    make([]*DBInstance, 0),
	}
}

// Init 初始化数据库连接
func (db *Database) Init() error {
	logger.Info("Initializing database connections...")

	// 检查配置
	if db.config.MySQL.Host == "" {
		return errors.New(errors.CodeInternalError, "MySQL host is not configured")
	}

	// 创建主库配置
	masterConfig := &DBConfig{
		Name:          "master",
		Host:          db.config.MySQL.Host,
		Port:          db.config.MySQL.Port,
		User:          db.config.MySQL.User,
		Password:      db.config.MySQL.Password,
		DBName:        db.config.MySQL.DBName,
		Charset:       db.config.MySQL.Charset,
		ParseTime:     db.config.MySQL.ParseTime,
		Loc:           db.config.MySQL.Loc,
		Role:          "master",
		MaxOpenConns:  db.config.MySQL.MaxOpenConns,
		MaxIdleConns:  db.config.MySQL.MaxIdleConns,
		SlowThreshold: 200, // 默认200ms
		EnableLogging: true,
		LogLevel:      "warn",
	}

	// 初始化主库
	masterInstance, err := db.createDBInstance(masterConfig)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to initialize master database")
	}

	db.master = masterInstance
	db.instances["master"] = masterInstance
	logger.Info("Master database connected successfully",
		logger.String("host", masterConfig.Host),
		logger.Int("port", masterConfig.Port),
		logger.String("database", masterConfig.DBName),
	)

	// 如果有从库配置，初始化从库
	// 这里可以从配置中读取从库列表，暂时简化处理
	// 实际项目中可以从配置文件中读取多个从库配置

	// 启动健康检查
	go db.startHealthCheck()

	logger.Info("Database initialization completed")
	return nil
}

// createDBInstance 创建数据库实例
func (db *Database) createDBInstance(cfg *DBConfig) (*DBInstance, error) {
	// 构建DSN
	dsn := db.buildDSN(cfg)

	// GORM配置
	gormConfig := &gorm.Config{
		PrepareStmt: true, // 开启预编译语句
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",  // 表前缀
			SingularTable: true,  // 使用单数表名
			NoLowerCase:   false, // 关闭小写转换
		},
		NowFunc: func() time.Time {
			return time.Now().UTC() // 使用UTC时间
		},
		DisableForeignKeyConstraintWhenMigrating: true, // 迁移时禁用外键约束
	}

	// 创建数据库连接
	gormDB, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to connect to database: %s", cfg.Name))
	}

	// 获取底层数据库连接
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to get database instance: %s", cfg.Name))
	}

	// 配置连接池
	db.configureConnectionPool(sqlDB, cfg)

	// 设置自定义日志器
	if cfg.EnableLogging {
		gormDB.Logger = NewGormLogger(cfg.SlowThreshold, cfg.LogLevel)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Database ping failed: %s", cfg.Name))
	}

	instance := &DBInstance{
		Name:        cfg.Name,
		Config:      cfg,
		DB:          gormDB,
		Role:        cfg.Role,
		IsConnected: true,
		lastPing:    time.Now(),
	}

	return instance, nil
}

// buildDSN 构建MySQL DSN
func (db *Database) buildDSN(cfg *DBConfig) string {
	if cfg.DSN != "" {
		return cfg.DSN
	}

	// 默认值
	if cfg.Charset == "" {
		cfg.Charset = "utf8mb4"
	}
	if cfg.Loc == "" {
		cfg.Loc = "Local"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%v&loc=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.Charset,
		cfg.ParseTime,
		cfg.Loc,
	)

	// MySQL8 特定参数
	dsn += "&allowNativePasswords=true&checkConnLiveness=true&timeout=5s"

	return dsn
}

// configureConnectionPool 配置连接池
func (db *Database) configureConnectionPool(sqlDB *sql.DB, cfg *DBConfig) {
	// 设置最大打开连接数
	maxOpenConns := 100
	if cfg.MaxOpenConns > 0 {
		maxOpenConns = cfg.MaxOpenConns
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)

	// 设置最大空闲连接数
	maxIdleConns := 20
	if cfg.MaxIdleConns > 0 {
		maxIdleConns = cfg.MaxIdleConns
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)

	// 设置连接最大生命周期
	connMaxLifetime := 30 * time.Minute
	if cfg.ConnMaxLifetime > 0 {
		connMaxLifetime = time.Duration(cfg.ConnMaxLifetime) * time.Second
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	// 设置连接最大空闲时间
	connMaxIdleTime := 10 * time.Minute
	if cfg.ConnMaxIdleTime > 0 {
		connMaxIdleTime = time.Duration(cfg.ConnMaxIdleTime) * time.Second
	}
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
}

// GetMaster 获取主库实例
func (db *Database) GetMaster() *gorm.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.master != nil {
		return db.master.DB
	}

	// 如果主库不存在，返回nil或panic
	logger.Error("Master database is not initialized")
	return nil
}

// GetSlave 获取从库实例（负载均衡）
func (db *Database) GetSlave() *gorm.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if len(db.slaves) == 0 {
		// 如果没有从库，使用主库
		return db.GetMaster()
	}

	// 轮询选择从库
	db.currentSlave = (db.currentSlave + 1) % len(db.slaves)
	return db.slaves[db.currentSlave].DB
}

// GetDB 根据读写类型获取数据库实例
func (db *Database) GetDB(opType string) *gorm.DB {
	switch opType {
	case "read":
		return db.GetSlave()
	case "write":
		return db.GetMaster()
	default:
		// 默认返回主库
		return db.GetMaster()
	}
}

// AddSlave 添加从库实例
func (db *Database) AddSlave(cfg *DBConfig) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	cfg.Role = "slave"
	if cfg.Name == "" {
		cfg.Name = fmt.Sprintf("slave_%d", len(db.slaves)+1)
	}

	instance, err := db.createDBInstance(cfg)
	if err != nil {
		return err
	}

	db.slaves = append(db.slaves, instance)
	db.instances[cfg.Name] = instance

	logger.Info("Slave database added successfully",
		logger.String("name", cfg.Name),
		logger.String("host", cfg.Host),
	)

	return nil
}

// Transaction 事务执行
func (db *Database) Transaction(fn func(tx *gorm.DB) error) error {
	return db.GetMaster().Transaction(fn)
}

// TransactionWithContext 带上下文的执行
func (db *Database) TransactionWithContext(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return db.GetMaster().WithContext(ctx).Transaction(fn)
}

// AutoMigrate 自动迁移
func (db *Database) AutoMigrate(models ...interface{}) error {
	return db.GetMaster().AutoMigrate(models...)
}

// Close 关闭所有数据库连接
func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	var errs []error

	for _, instance := range db.instances {
		if instance.DB != nil {
			sqlDB, err := instance.DB.DB()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if err := sqlDB.Close(); err != nil {
				errs = append(errs, err)
			} else {
				instance.IsConnected = false
				logger.Info("Database connection closed",
					logger.String("name", instance.Name),
				)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing database connections: %v", errs)
	}

	logger.Info("All database connections closed successfully")
	return nil
}

// startHealthCheck 启动健康检查
func (db *Database) startHealthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.checkConnections()
		}
	}
}

// checkConnections 检查连接状态
func (db *Database) checkConnections() {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, instance := range db.instances {
		if instance.DB != nil {
			sqlDB, err := instance.DB.DB()
			if err != nil {
				logger.Error("Failed to get database connection",
					logger.String("name", instance.Name),
					logger.ErrorField(err),
				)
				instance.IsConnected = false
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = sqlDB.PingContext(ctx)
			cancel()

			if err != nil {
				logger.Error("Database connection check failed",
					logger.String("name", instance.Name),
					logger.ErrorField(err),
				)
				instance.IsConnected = false
			} else {
				instance.IsConnected = true
				instance.lastPing = time.Now()
			}
		}
	}
}

// GetStats 获取连接池统计信息
func (db *Database) GetStats() map[string]interface{} {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := make(map[string]interface{})

	for name, instance := range db.instances {
		if instance.DB != nil {
			sqlDB, err := instance.DB.DB()
			if err == nil {
				dbStats := sqlDB.Stats()
				stats[name] = map[string]interface{}{
					"role":             instance.Role,
					"is_connected":     instance.IsConnected,
					"last_ping":        instance.lastPing.Format(time.RFC3339),
					"open_connections": dbStats.OpenConnections,
					"in_use":           dbStats.InUse,
					"idle":             dbStats.Idle,
					"wait_count":       dbStats.WaitCount,
					"wait_duration":    dbStats.WaitDuration.String(),
					"max_open_conns":   dbStats.MaxOpenConnections,
				}
			}
		}
	}

	return stats
}

// WithContext 创建带上下文的数据库实例
func (db *Database) WithContext(ctx context.Context) *gorm.DB {
	return db.GetMaster().WithContext(ctx)
}

// GetInstance 获取指定名称的数据库实例
func (db *Database) GetInstance(name string) (*gorm.DB, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	instance, exists := db.instances[name]
	if !exists {
		return nil, errors.New(errors.CodeResourceNotFound,
			fmt.Sprintf("Database instance '%s' not found", name))
	}

	return instance.DB, nil
}

// IsConnected 检查数据库是否连接
func (db *Database) IsConnected() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.master == nil {
		return false
	}
	return db.master.IsConnected
}
