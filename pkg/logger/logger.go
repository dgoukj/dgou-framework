package logger

import (
	"context"
	"dgou/pkg/config"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// Logger 全局日志实例
	Logger *zap.Logger
	sugar  *zap.SugaredLogger
	once   sync.Once
	mu     sync.RWMutex
)

// ContextKey 上下文键类型
type ContextKey string

const (
	// RequestIDKey 请求ID在上下文中的键
	RequestIDKey ContextKey = "request_id"
	// UserIDKey 用户ID在上下文中的键
	UserIDKey ContextKey = "user_id"
	// TraceIDKey 追踪ID在上下文中的键
	TraceIDKey ContextKey = "trace_id"
)

// InitLogger 初始化日志组件
func InitLogger(cfg *config.LogConfig) {
	once.Do(func() {
		// 设置日志级别
		level := zap.NewAtomicLevel()
		if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
			level.SetLevel(zap.InfoLevel)
		}

		// 编码器配置（生产环境优化）
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		// 创建输出
		var cores []zapcore.Core

		// 控制台输出（开发环境）
		if config.GetConfig().Server.Mode == "debug" {
			consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
			consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
			cores = append(cores, consoleCore)
		}

		// 文件输出
		if cfg.File != "" {
			// 确保日志目录存在
			if err := os.MkdirAll(getLogDir(cfg.File), 0755); err != nil {
				fmt.Printf("Failed to create log directory: %v\n", err)
			}

			// 使用lumberjack进行日志轮转
			lumberjackLogger := &lumberjack.Logger{
				Filename:   cfg.File,
				MaxSize:    cfg.MaxSize,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAge,
				Compress:   cfg.Compress,
			}

			fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
			fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(lumberjackLogger), level)
			cores = append(cores, fileCore)
		}

		// 如果没有配置任何输出，使用标准输出
		if len(cores) == 0 {
			consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
			consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
			cores = append(cores, consoleCore)
		}

		// 创建核心
		core := zapcore.NewTee(cores...)

		// 构建Logger
		options := []zap.Option{
			zap.AddCaller(),
			zap.AddCallerSkip(1),
			zap.AddStacktrace(zap.ErrorLevel),
		}

		if cfg.AddCaller {
			options = append(options, zap.AddCaller())
		}

		Logger = zap.New(core, options...)
		sugar = Logger.Sugar()

		// 替换全局logger
		zap.ReplaceGlobals(Logger)

		Logger.Info("Logger initialized successfully",
			zap.String("level", cfg.Level),
			zap.String("file", cfg.File),
		)
	})
}

// getLogDir 获取日志目录
func getLogDir(filepath string) string {
	if idx := strings.LastIndex(filepath, "/"); idx != -1 {
		return filepath[:idx]
	}
	if idx := strings.LastIndex(filepath, "\\"); idx != -1 {
		return filepath[:idx]
	}
	return "."
}

// Sync 同步日志缓冲区
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// ==================== 上下文日志方法 ====================

// WithContext 从上下文中提取信息并创建带上下文的Logger
func WithContext(ctx context.Context) *zap.Logger {
	logger := Logger
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	// 添加请求ID
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		logger = logger.With(zap.String("request_id", requestID))
	}

	// 添加用户ID
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		logger = logger.With(zap.String("user_id", userID))
	}

	// 添加追踪ID
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		logger = logger.With(zap.String("trace_id", traceID))
	}

	return logger
}

// CtxInfo 带上下文的Info级别日志
func CtxInfo(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Info(msg, fields...)
}

// CtxError 带上下文的Error级别日志
func CtxError(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Error(msg, fields...)
}

// CtxWarn 带上下文的Warn级别日志
func CtxWarn(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Warn(msg, fields...)
}

// CtxDebug 带上下文的Debug级别日志
func CtxDebug(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Debug(msg, fields...)
}

// ==================== 常规日志方法 ====================

// Info 记录Info级别日志
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Error 记录Error级别日志
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// Warn 记录Warn级别日志
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Debug 记录Debug级别日志
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Panic 记录Panic级别日志
func Panic(msg string, fields ...zap.Field) {
	Logger.Panic(msg, fields...)
}

// Fatal 记录Fatal级别日志
func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// ==================== 结构化字段创建 ====================

// String 创建字符串字段
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int 创建整数字段
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Int64 创建64位整数字段
func Int64(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

// Float64 创建浮点数字段
func Float64(key string, value float64) zap.Field {
	return zap.Float64(key, value)
}

// Bool 创建布尔字段
func Bool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

// Any 创建任意类型字段
func Any(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

// ErrorField 创建错误字段
func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

// Duration 创建时间间隔字段
func Duration(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

// Time 创建时间字段
func Time(key string, value time.Time) zap.Field {
	return zap.Time(key, value)
}

// ==================== 性能日志方法 ====================

// TimeTrack 跟踪函数执行时间
func TimeTrack(ctx context.Context, start time.Time, name string) {
	elapsed := time.Since(start)
	WithContext(ctx).Info("Function execution time",
		zap.String("function", name),
		zap.Duration("elapsed", elapsed),
	)
}

// MemoryUsage 记录内存使用情况
func MemoryUsage(ctx context.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	WithContext(ctx).Debug("Memory usage",
		zap.Uint64("alloc", m.Alloc),
		zap.Uint64("total_alloc", m.TotalAlloc),
		zap.Uint64("sys", m.Sys),
		zap.Uint32("num_gc", m.NumGC),
	)
}

// ==================== Sugar Logger 方法 ====================

// Sugar 获取SugaredLogger
func Sugar() *zap.SugaredLogger {
	mu.RLock()
	if sugar == nil {
		mu.RUnlock()
		mu.Lock()
		if sugar == nil {
			sugar = Logger.Sugar()
		}
		mu.Unlock()
		return sugar
	}
	mu.RUnlock()
	return sugar
}

// Infof 格式化Info日志
func Infof(template string, args ...interface{}) {
	Sugar().Infof(template, args...)
}

// Errorf 格式化Error日志
func Errorf(template string, args ...interface{}) {
	Sugar().Errorf(template, args...)
}

// Warnf 格式化Warn日志
func Warnf(template string, args ...interface{}) {
	Sugar().Warnf(template, args...)
}

// Debugf 格式化Debug日志
func Debugf(template string, args ...interface{}) {
	Sugar().Debugf(template, args...)
}
