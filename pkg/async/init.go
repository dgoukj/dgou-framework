// file: pkg/async/init.go
package async

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"sync"
	"time"
)

var (
	// globalTaskManager 全局任务管理器实例
	globalTaskManager *TaskManager
	// taskManagerMutex 用于保护全局任务管理器实例
	taskManagerMutex sync.RWMutex
	// isInitialized 标记异步组件是否已初始化
	isInitialized bool
	// initMutex 用于保护初始化过程
	initMutex sync.Mutex
)

// Init 初始化异步组件（使用默认配置）
func Init() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if isInitialized {
		logger.Info("Async component already initialized")
		return nil
	}

	// 获取应用配置
	cfg := config.GetConfig()
	if cfg == nil {
		logger.Warn("Config not found, using default configuration")
		cfg = &config.Config{
			Async: config.AsyncConfig{
				MaxWorkers:     100,
				MaxQueueSize:   10000,
				WorkerIdleTime: 30 * time.Second,
				EnableMetrics:  true,
				TaskRetries:    3,
				TaskTimeout:    30 * time.Second,
			},
		}
	}

	return InitWithConfig(cfg)
}

// InitWithConfig 使用自定义配置初始化异步组件
func InitWithConfig(cfg *config.Config) error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if isInitialized {
		logger.Info("Async component already initialized, reinitializing")
		// 先停止已有的管理器
		if globalTaskManager != nil {
			globalTaskManager.StopAll()
		}
	}

	// 初始化任务管理器
	manager, err := InitTaskManager(cfg)
	if err != nil {
		logger.Error("Failed to initialize async component",
			logger.ErrorField(err),
		)
		return err
	}

	// 设置全局实例
	SetGlobalTaskManager(manager)
	isInitialized = true

	logger.Info("Async component initialized successfully",
		logger.Int("max_workers", cfg.Async.MaxWorkers),
		logger.Int("max_queue_size", cfg.Async.MaxQueueSize),
		logger.Bool("metrics_enabled", cfg.Async.EnableMetrics),
	)

	return nil
}

// SetGlobalTaskManager 设置全局任务管理器
func SetGlobalTaskManager(manager *TaskManager) {
	taskManagerMutex.Lock()
	defer taskManagerMutex.Unlock()
	globalTaskManager = manager
}

// GetGlobalTaskManager 获取全局任务管理器
func GetGlobalTaskManager() *TaskManager {
	taskManagerMutex.RLock()
	defer taskManagerMutex.RUnlock()
	return globalTaskManager
}

// IsInitialized 检查异步组件是否已初始化
func IsInitialized() bool {
	taskManagerMutex.RLock()
	defer taskManagerMutex.RUnlock()
	return globalTaskManager != nil && isInitialized
}

// NewTask 创建新任务（使用默认管理器）
func NewTask(name string, handler TaskHandler, params interface{}) *Task {
	return NewTask(name, handler, params)
}

// Submit 提交任务（使用默认管理器）
func Submit(task *Task) (string, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return "", NewError("Task manager not initialized, please call async.Init() first")
	}
	return manager.Submit(task)
}

// SubmitToPool 提交任务到指定协程池（使用默认管理器）
func SubmitToPool(poolName string, task *Task) (string, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return "", NewError("Task manager not initialized, please call async.Init() first")
	}
	return manager.SubmitToPool(poolName, task)
}

// SubmitAndWait 提交任务并等待完成（使用默认管理器）
func SubmitAndWait(task *Task, timeout time.Duration) (*TaskResult, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return nil, NewError("Task manager not initialized, please call async.Init() first")
	}

	// 获取默认协程池
	defaultPool, err := manager.GetPool("")
	if err != nil {
		return nil, err
	}

	return defaultPool.SubmitAndWait(task, timeout)
}

// GetTask 获取任务信息（使用默认管理器）
func GetTask(taskID string) (*Task, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return nil, NewError("Task manager not initialized, please call async.Init() first")
	}
	return manager.GetTask(taskID)
}

// CancelTask 取消任务（使用默认管理器）
func CancelTask(taskID string) error {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return NewError("Task manager not initialized, please call async.Init() first")
	}
	return manager.CancelTask(taskID)
}

// CreatePool 创建新的协程池（使用默认管理器）
func CreatePool(name string, config *PoolConfig) (*WorkerPool, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return nil, NewError("Task manager not initialized, please call async.Init() first")
	}
	return manager.CreatePool(name, config)
}

// GetPool 获取协程池（使用默认管理器）
func GetPool(name string) (*WorkerPool, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return nil, NewError("Task manager not initialized, please call async.Init() first")
	}
	return manager.GetPool(name)
}

// GetStats 获取统计信息（使用默认管理器）
func GetStats() (map[string]interface{}, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return nil, NewError("Task manager not initialized, please call async.Init() first")
	}
	return manager.GetStats(), nil
}

// ExecuteWithRetry 执行带重试的操作
func ExecuteWithRetry(ctx context.Context, operation func() error, maxRetries int, delay time.Duration) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// 如果是最后一次尝试，直接返回
		if i == maxRetries {
			logger.Warn("Max retries reached",
				logger.Int("max_retries", maxRetries),
				logger.ErrorField(err),
			)
			break
		}

		// 记录重试
		logger.Warn("Operation failed, retrying...",
			logger.Int("attempt", i+1),
			logger.Int("max_retries", maxRetries),
			logger.Duration("delay", delay),
			logger.ErrorField(err),
		)

		// 等待重试延迟
		select {
		case <-time.After(delay):
			// 增加延迟（指数退避）
			delay = time.Duration(float64(delay) * 1.5)
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errors.Wrap(lastErr, errors.CodeInternalError,
		"Operation failed after retries")
}

// Schedule 定时执行任务
func Schedule(ctx context.Context, interval time.Duration, task *Task) (string, error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return "", NewError("Task manager not initialized, please call async.Init() first")
	}

	// 创建定时任务包装器
	wrappedTask := &Task{
		ID:       generateTaskID(),
		Name:     "scheduled-" + task.Name,
		Handler:  createScheduledHandler(task, interval),
		Params:   nil,
		Priority: task.Priority,
		Timeout:  task.Timeout,
	}

	return manager.Submit(wrappedTask)
}

// createScheduledHandler 创建定时任务处理函数
func createScheduledHandler(task *Task, interval time.Duration) TaskHandler {
	return func(ctx context.Context, params interface{}) (interface{}, error) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ticker.C:
				// 执行任务
				result, err := task.Handler(ctx, task.Params)
				if err != nil {
					logger.Error("Scheduled task execution failed",
						logger.String("task_name", task.Name),
						logger.ErrorField(err),
					)
				} else {
					logger.Debug("Scheduled task executed",
						logger.String("task_name", task.Name),
						logger.Duration("interval", interval),
					)
				}
				return result, err
			}
		}
	}
}

// BatchSubmit 批量提交任务
func BatchSubmit(tasks []*Task) ([]string, []error) {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return nil, []error{NewError("Task manager not initialized, please call async.Init() first")}
	}

	var taskIDs []string
	var errors []error

	for _, task := range tasks {
		taskID, err := manager.Submit(task)
		if err != nil {
			errors = append(errors, fmt.Errorf("task %s: %v", task.Name, err))
		} else {
			taskIDs = append(taskIDs, taskID)
		}
	}

	logger.Info("Batch submit completed",
		logger.Int("total", len(tasks)),
		logger.Int("success", len(taskIDs)),
		logger.Int("failed", len(errors)),
	)

	return taskIDs, errors
}

// WaitAll 等待所有任务完成
func WaitAll(ctx context.Context, taskIDs []string, timeout time.Duration) error {
	manager := GetGlobalTaskManager()
	if manager == nil {
		return NewError("Task manager not initialized, please call async.Init() first")
	}

	deadline := time.Now().Add(timeout)

	for _, taskID := range taskIDs {
		remaining := deadline.Sub(time.Now())
		if remaining <= 0 {
			return errors.New(errors.CodeInternalError, "Wait timeout")
		}

		task, err := manager.GetTask(taskID)
		if err != nil {
			return err
		}

		if !task.Wait(remaining) {
			return errors.New(errors.CodeInternalError, "Task wait timeout")
		}
	}

	return nil
}

// Stop 停止异步组件
func Stop() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if !isInitialized {
		logger.Info("Async component not initialized")
		return nil
	}

	manager := GetGlobalTaskManager()
	if manager == nil {
		isInitialized = false
		return nil
	}

	// 停止所有协程池
	err := manager.StopAll()
	if err != nil {
		logger.Error("Failed to stop async component",
			logger.ErrorField(err),
		)
		return err
	}

	// 清理状态
	taskManagerMutex.Lock()
	globalTaskManager = nil
	isInitialized = false
	taskManagerMutex.Unlock()

	logger.Info("Async component stopped successfully")
	return nil
}

// GetDefaultPoolConfig 获取默认协程池配置
func GetDefaultPoolConfig() *PoolConfig {
	return DefaultPoolConfig()
}

// NewError 创建异步组件错误
func NewError(message string) error {
	return &AsyncError{
		Message: message,
	}
}

// AsyncError 异步组件错误
type AsyncError struct {
	Message string
}

// Error 实现error接口
func (e *AsyncError) Error() string {
	return e.Message
}

// 初始化检查
func init() {
	// 注册配置验证器
	config.RegisterValidator(func(cfg *config.Config) error {
		// 验证异步配置
		if cfg.Async.MaxWorkers <= 0 {
			logger.Warn("Async max workers not configured or invalid, using default")
		}
		if cfg.Async.MaxQueueSize <= 0 {
			logger.Warn("Async max queue size not configured or invalid, using default")
		}
		return nil
	})

	// 注册优雅关闭处理器
	config.RegisterShutdownHandler(func() error {
		return Stop()
	})

	logger.Info("Async component package initialized")
}
