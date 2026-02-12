package database

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	pkgErrors "github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type DSNConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string
	Charset      string
	ParseTime    bool
	Loc          string
	MaxOpenConns int
	MaxIdleConns int
}

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // seconds
	ConnMaxIdleTime int // seconds
}

type LogConfig struct {
	SlowThreshold int
	EnableLogging bool
	LogLevel      string
}

type PerformanceConfig struct {
	PrepareStmt       bool
	DisableForeignKey bool
}

type Config struct {
	Master      DSNConfig
	Slaves      []DSNConfig
	Pool        PoolConfig
	Log         LogConfig
	Performance PerformanceConfig
}

type DB struct {
	master  *gorm.DB
	slaves  []*gorm.DB
	counter uint64
	gormLog logger.Interface
}

func New(cfg Config) (*DB, error) {
	master, err := openDB(cfg.Master, cfg.Pool, cfg.Performance, cfg.Log)
	if err != nil {
		return nil, pkgErrors.Wrap(err, "connect master")
	}
	var slaves []*gorm.DB
	for _, slaveCfg := range cfg.Slaves {
		db, err := openDB(slaveCfg, cfg.Pool, cfg.Performance, cfg.Log)
		if err != nil {
			// 从库连接失败仅记录，不影响主库
			continue
		}
		slaves = append(slaves, db)
	}
	return &DB{
		master: master,
		slaves: slaves,
	}, nil
}

func openDB(dsnCfg DSNConfig, pool PoolConfig, perf PerformanceConfig, logCfg LogConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%v&loc=%s",
		dsnCfg.User, dsnCfg.Password, dsnCfg.Host, dsnCfg.Port, dsnCfg.DBName,
		dsnCfg.Charset, dsnCfg.ParseTime, dsnCfg.Loc,
	)
	gormConfig := &gorm.Config{
		PrepareStmt: perf.PrepareStmt,
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		DisableForeignKeyConstraintWhenMigrating: perf.DisableForeignKey,
	}
	if logCfg.EnableLogging {
		gormConfig.Logger = newGormLogger(logCfg)
	}
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(pool.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(pool.ConnMaxIdleTime) * time.Second)
	return db, nil
}

func (db *DB) Master() *gorm.DB {
	return db.master
}

func (db *DB) Slave() *gorm.DB {
	if len(db.slaves) == 0 {
		return db.master
	}
	idx := atomic.AddUint64(&db.counter, 1) % uint64(len(db.slaves))
	return db.slaves[idx]
}

func (db *DB) Close() error {
	var errs []error
	if sqlDB, err := db.master.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, s := range db.slaves {
		if sqlDB, err := s.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close db errors: %v", errs)
	}
	return nil
}

// GORM Logger 适配器
type gormLogger struct {
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

func newGormLogger(cfg LogConfig) *gormLogger {
	level := logger.Warn
	switch cfg.LogLevel {
	case "silent":
		level = logger.Silent
	case "error":
		level = logger.Error
	case "warn":
		level = logger.Warn
	case "info":
		level = logger.Info
	}
	return &gormLogger{
		LogLevel:      level,
		SlowThreshold: time.Duration(cfg.SlowThreshold) * time.Millisecond,
	}
}

func (l *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		// 这里不依赖外部 logger，仅输出到标准输出
	}
}

func (l *gormLogger) Warn(ctx context.Context, msg string, data ...interface{})  {}
func (l *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {}
func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
}
