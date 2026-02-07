package database

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"gorm.io/gorm"
	"sync"
)

var (
	// globalDB 全局数据库实例
	globalDB *Database
	// once 确保单例初始化
	once sync.Once
)

// InitDB 初始化数据库（单例模式）
func InitDB(cfg *config.Config) (*Database, error) {
	var initErr error

	once.Do(func() {
		db := NewDatabase(cfg)
		if err := db.Init(); err != nil {
			initErr = err
			return
		}
		globalDB = db

		// 注册优雅关闭
		// 这里需要与应用的优雅关闭机制集成
		// 暂时省略，实际使用时需要添加

		logger.Info("Database initialized successfully")
	})

	return globalDB, initErr
}

// GetDB 获取全局数据库实例
func GetDB() *Database {
	if globalDB == nil {
		logger.Error("Database not initialized, please call InitDB first")
		// 在实际项目中，这里应该panic或返回错误
		// 为了向后兼容，尝试初始化默认配置
		cfg := config.GetConfig()
		db, err := InitDB(cfg)
		if err != nil {
			logger.Error("Failed to initialize database", logger.ErrorField(err))
			return nil
		}
		return db
	}
	return globalDB
}

// Master 获取主库实例（快捷方式）
func Master() *gorm.DB {
	db := GetDB()
	if db == nil {
		return nil
	}
	return db.GetMaster()
}

// Slave 获取从库实例（快捷方式）
func Slave() *gorm.DB {
	db := GetDB()
	if db == nil {
		return nil
	}
	return db.GetSlave()
}

// WithContext 创建带上下文的数据库实例（快捷方式）
func WithContext(ctx context.Context) *gorm.DB {
	db := GetDB()
	if db == nil {
		return nil
	}
	return db.WithContext(ctx)
}

// Transaction 事务执行（快捷方式）
func Transaction(fn func(tx *gorm.DB) error) error {
	db := GetDB()
	if db == nil {
		return errors.New(errors.CodeInternalError, "Database not initialized")
	}
	return db.Transaction(fn)
}

// CloseDB 关闭数据库连接（用于优雅关闭）
func CloseDB() error {
	if globalDB == nil {
		return nil
	}
	return globalDB.Close()
}

// GetMigrator 获取迁移管理器
func GetMigrator(migrationsDir string) (*Migrator, error) {
	db := GetDB()
	if db == nil {
		return nil, errors.New(errors.CodeInternalError, "Database not initialized")
	}

	migrator := NewMigrator(db.GetMaster(), migrationsDir)
	if err := migrator.LoadMigrations(); err != nil {
		return nil, err
	}

	return migrator, nil
}
