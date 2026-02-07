// pkg/monitor/database.go
package monitor

import (
	"context"
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
	return db.Callback().Create().After("gorm:create").Register("monitor:create", gm.afterCreate)
	db = db.Callback().Query().After("gorm:query").Register("monitor:query", gm.afterQuery)
	db = db.Callback().Update().After("gorm:update").Register("monitor:update", gm.afterUpdate)
	db = db.Callback().Delete().After("gorm:delete").Register("monitor:delete", gm.afterDelete)
	db = db.Callback().Row().After("gorm:row").Register("monitor:row", gm.afterRow)
	db = db.Callback().Raw().After("gorm:raw").Register("monitor:raw", gm.afterRaw)

	return db
}

func (gm *GormMonitor) afterCreate(db *gorm.DB) {
	if db.Error == nil && db.Statement != nil && db.Statement.Table != "" {
		duration := time.Since(db.Statement.StartTime)
		gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "create", duration, db.Error == nil)
	}
}

func (gm *GormMonitor) afterQuery(db *gorm.DB) {
	if db.Error == nil && db.Statement != nil && db.Statement.Table != "" {
		duration := time.Since(db.Statement.StartTime)
		gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "query", duration, db.Error == nil)
	}
}

func (gm *GormMonitor) afterUpdate(db *gorm.DB) {
	if db.Error == nil && db.Statement != nil && db.Statement.Table != "" {
		duration := time.Since(db.Statement.StartTime)
		gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "update", duration, db.Error == nil)
	}
}

func (gm *GormMonitor) afterDelete(db *gorm.DB) {
	if db.Error == nil && db.Statement != nil && db.Statement.Table != "" {
		duration := time.Since(db.Statement.StartTime)
		gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "delete", duration, db.Error == nil)
	}
}

func (gm *GormMonitor) afterRow(db *gorm.DB) {
	if db.Error == nil && db.Statement != nil && db.Statement.Table != "" {
		duration := time.Since(db.Statement.StartTime)
		gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "row", duration, db.Error == nil)
	}
}

func (gm *GormMonitor) afterRaw(db *gorm.DB) {
	if db.Error == nil && db.Statement != nil && db.Statement.Table != "" {
		duration := time.Since(db.Statement.StartTime)
		gm.monitor.RecordDBQuery("gorm", db.Statement.Table, "raw", duration, db.Error == nil)
	}
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
