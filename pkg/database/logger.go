package database

import (
	"context"
	"dgou/pkg/logger"
	"fmt"
	"time"

	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// GormLogger GORM自定义日志器
type GormLogger struct {
	SlowThreshold time.Duration       // 慢查询阈值
	LogLevel      gormLogger.LogLevel // 日志级别
}

// NewGormLogger 创建新的GORM日志器
func NewGormLogger(slowThresholdMs int, logLevel string) gormLogger.Interface {
	level := logger.Warn
	switch logLevel {
	case "silent":
		level = logger.Silent
	case "error":
		level = logger.Error
	case "warn":
		level = logger.Warn
	case "info":
		level = logger.Info
	}

	return &GormLogger{
		SlowThreshold: time.Duration(slowThresholdMs) * time.Millisecond,
		LogLevel:      level,
	}
}

// LogMode 设置日志模式
func (l *GormLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 打印信息
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		logger.CtxInfo(ctx, fmt.Sprintf("[GORM] "+msg, data...))
	}
}

// Warn 打印警告
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		logger.CtxWarn(ctx, fmt.Sprintf("[GORM] "+msg, data...))
	}
}

// Error 打印错误
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		logger.CtxError(ctx, fmt.Sprintf("[GORM] "+msg, data...))
	}
}

// Trace 跟踪SQL执行
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	logFields := []logger.Field{
		logger.String("sql", sql),
		logger.Duration("elapsed", elapsed),
		logger.Int64("rows", rows),
	}

	// 获取请求上下文信息
	if requestID, ok := ctx.Value(logger.RequestIDKey).(string); ok && requestID != "" {
		logFields = append(logFields, logger.String("request_id", requestID))
	}

	// 检查是否为慢查询
	isSlow := elapsed > l.SlowThreshold && l.SlowThreshold > 0

	switch {
	case err != nil && l.LogLevel >= logger.Error:
		// SQL执行错误
		logFields = append(logFields, logger.ErrorField(err))
		logger.CtxError(ctx, "SQL execution failed", logFields...)

	case isSlow && l.LogLevel >= logger.Warn:
		// 慢查询
		logger.CtxWarn(ctx, "Slow SQL query detected", logFields...)

	case l.LogLevel >= logger.Info:
		// 普通SQL日志
		logger.CtxDebug(ctx, "SQL executed", logFields...)
	}
}
