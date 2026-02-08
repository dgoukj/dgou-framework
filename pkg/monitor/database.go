// pkg/monitor/database.go
package monitor

import (
	"context"
	"dgou/pkg/logger"
	"time"

	"gorm.io/gorm"
)

// GormMonitor GORM监控包装器
type GormMonitor struct {
	monitor *Monitor
}

// NewGormMonitor 创建GORM监控器
func NewGormMonitor(monitor *Monitor) *GormMonitor {
	return &GormMonitor{
		monitor: monitor,
	}
}

// WrapDB 包装GORM数据库实例
func (gm *GormMonitor) WrapDB(db *gorm.DB) *gorm.DB {
	// 定义回调函数
	registerCallbacks := func(db *gorm.DB) error {
		// 注册各个操作的Before回调（记录开始时间）
		err := db.Callback().Create().Before("gorm:create").Register("monitor:before_create", gm.beforeOperation)
		if err != nil {
			return err
		}

		err = db.Callback().Query().Before("gorm:query").Register("monitor:before_query", gm.beforeOperation)
		if err != nil {
			return err
		}

		err = db.Callback().Update().Before("gorm:update").Register("monitor:before_update", gm.beforeOperation)
		if err != nil {
			return err
		}

		err = db.Callback().Delete().Before("gorm:delete").Register("monitor:before_delete", gm.beforeOperation)
		if err != nil {
			return err
		}

		err = db.Callback().Row().Before("gorm:row").Register("monitor:before_row", gm.beforeOperation)
		if err != nil {
			return err
		}

		err = db.Callback().Raw().Before("gorm:raw").Register("monitor:before_raw", gm.beforeOperation)
		if err != nil {
			return err
		}

		// 注册各个操作的After回调（记录耗时）
		err = db.Callback().Create().After("gorm:create").Register("monitor:after_create", gm.afterCreate)
		if err != nil {
			return err
		}

		err = db.Callback().Query().After("gorm:query").Register("monitor:after_query", gm.afterQuery)
		if err != nil {
			return err
		}

		err = db.Callback().Update().After("gorm:update").Register("monitor:after_update", gm.afterUpdate)
		if err != nil {
			return err
		}

		err = db.Callback().Delete().After("gorm:delete").Register("monitor:after_delete", gm.afterDelete)
		if err != nil {
			return err
		}

		err = db.Callback().Row().After("gorm:row").Register("monitor:after_row", gm.afterRow)
		if err != nil {
			return err
		}

		err = db.Callback().Raw().After("gorm:raw").Register("monitor:after_raw", gm.afterRaw)
		if err != nil {
			return err
		}

		return nil
	}

	// 应用回调
	if err := registerCallbacks(db); err != nil {
		logger.Error("Failed to register GORM callbacks", logger.ErrorField(err))
	}

	return db
}

// contextKey 用于在上下文中存储数据的键类型
type contextKey string

const (
	startTimeKey contextKey = "startTime"
)

// beforeOperation 操作前的回调，记录开始时间
func (gm *GormMonitor) beforeOperation(db *gorm.DB) {
	if db.Statement == nil {
		return
	}

	// 创建新的上下文，存储开始时间
	ctx := context.WithValue(db.Statement.Context, startTimeKey, time.Now())
	db.Statement.Context = ctx
}

// getStartTime 从上下文中获取开始时间
func (gm *GormMonitor) getStartTime(db *gorm.DB) (time.Time, bool) {
	if db.Statement == nil || db.Statement.Context == nil {
		return time.Time{}, false
	}

	startTime, ok := db.Statement.Context.Value(startTimeKey).(time.Time)
	return startTime, ok
}

// afterCreate 创建操作后的回调
func (gm *GormMonitor) afterCreate(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "" {
		return
	}

	startTime, ok := gm.getStartTime(db)
	if !ok {
		return
	}

	duration := time.Since(startTime)
	gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "create", duration, db.Error == nil)
}

// afterQuery 查询操作后的回调
func (gm *GormMonitor) afterQuery(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "" {
		return
	}

	startTime, ok := gm.getStartTime(db)
	if !ok {
		return
	}

	duration := time.Since(startTime)
	gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "query", duration, db.Error == nil)
}

// afterUpdate 更新操作后的回调
func (gm *GormMonitor) afterUpdate(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "" {
		return
	}

	startTime, ok := gm.getStartTime(db)
	if !ok {
		return
	}

	duration := time.Since(startTime)
	gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "update", duration, db.Error == nil)
}

// afterDelete 删除操作后的回调
func (gm *GormMonitor) afterDelete(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "" {
		return
	}

	startTime, ok := gm.getStartTime(db)
	if !ok {
		return
	}

	duration := time.Since(startTime)
	gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "delete", duration, db.Error == nil)
}

// afterRow Row查询后的回调
func (gm *GormMonitor) afterRow(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "" {
		return
	}

	startTime, ok := gm.getStartTime(db)
	if !ok {
		return
	}

	duration := time.Since(startTime)
	gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "row", duration, db.Error == nil)
}

// afterRaw Raw查询后的回调
func (gm *GormMonitor) afterRaw(db *gorm.DB) {
	if db.Error != nil || db.Statement == nil || db.Statement.Table == "" {
		return
	}

	startTime, ok := gm.getStartTime(db)
	if !ok {
		return
	}

	duration := time.Since(startTime)
	gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "raw", duration, db.Error == nil)
}

// UpdateDBStats 更新数据库连接池统计
func (gm *GormMonitor) UpdateDBStats(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		stats := sqlDB.Stats()
		gm.monitor.UpdateDBConnectionStats(
			stats.InUse,
			stats.Idle,
			stats.OpenConnections,
		)
	}
}
