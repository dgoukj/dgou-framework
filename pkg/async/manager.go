// file: pkg/async/manager.go
package async

import (
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"sync"
)

// TaskManager 任务管理器
type TaskManager struct {
	pools       map[string]*WorkerPool // 协程池映射
	defaultPool *WorkerPool            // 默认协程池
	mu          sync.RWMutex           // 读写锁
}

var (
	globalManager *TaskManager
	once          sync.Once
)

// InitTaskManager 初始化任务管理器
func InitTaskManager(cfg *config.Config) (*TaskManager, error) {
	var initErr error

	once.Do(func() {
		// 从配置加载协程池配置
		poolConfig := &PoolConfig{
			MaxWorkers:     cfg.Async.MaxWorkers,
			MaxQueueSize:   cfg.Async.MaxQueueSize,
			WorkerIdleTime: cfg.Async.WorkerIdleTime,
			EnableMetrics:  cfg.Async.EnableMetrics,
		}

		// 创建默认协程池
		defaultPool := NewWorkerPool(poolConfig)
		if err := defaultPool.Start(); err != nil {
			initErr = err
			return
		}

		globalManager = &TaskManager{
			pools:       make(map[string]*WorkerPool),
			defaultPool: defaultPool,
		}

		// 注册优雅关闭
		// 这里需要与应用的优雅关闭机制集成
		// 暂时省略，实际使用时需要添加

		logger.Info("Task manager initialized successfully")
	})

	return globalManager, initErr
}

// GetTaskManager 获取任务管理器
func GetTaskManager() *TaskManager {
	if globalManager == nil {
		logger.Error("Task manager not initialized, please call InitTaskManager first")
		// 在实际项目中，这里应该panic或返回错误
		// 为了向后兼容，尝试初始化默认配置
		cfg := config.GetConfig()
		manager, err := InitTaskManager(cfg)
		if err != nil {
			logger.Error("Failed to initialize task manager", logger.ErrorField(err))
			// 创建默认管理器
			globalManager = &TaskManager{
				pools:       make(map[string]*WorkerPool),
				defaultPool: NewWorkerPool(DefaultPoolConfig()),
			}
		} else {
			return manager
		}
	}
	return globalManager
}

// CreatePool 创建新的协程池
func (m *TaskManager) CreatePool(name string, config *PoolConfig) (*WorkerPool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pools[name]; exists {
		return nil, errors.New(errors.CodeValidationFailed,
			"Worker pool already exists: "+name)
	}

	pool := NewWorkerPool(config)
	if err := pool.Start(); err != nil {
		return nil, err
	}

	m.pools[name] = pool
	logger.Info("Worker pool created",
		logger.String("name", name),
		logger.Int("max_workers", config.MaxWorkers),
	)

	return pool, nil
}

// GetPool 获取协程池
func (m *TaskManager) GetPool(name string) (*WorkerPool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if name == "" || name == "default" {
		return m.defaultPool, nil
	}

	pool, exists := m.pools[name]
	if !exists {
		return nil, errors.New(errors.CodeResourceNotFound,
			"Worker pool not found: "+name)
	}

	return pool, nil
}

// Submit 提交任务到默认协程池
func (m *TaskManager) Submit(task *Task) (string, error) {
	return m.defaultPool.Submit(task)
}

// SubmitToPool 提交任务到指定协程池
func (m *TaskManager) SubmitToPool(poolName string, task *Task) (string, error) {
	pool, err := m.GetPool(poolName)
	if err != nil {
		return "", err
	}

	return pool.Submit(task)
}

// GetTask 获取任务信息
func (m *TaskManager) GetTask(taskID string) (*Task, error) {
	// 在所有协程池中查找任务
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 在默认协程池中查找
	if task, err := m.defaultPool.GetTask(taskID); err == nil {
		return task, nil
	}

	// 在其他协程池中查找
	for _, pool := range m.pools {
		if task, err := pool.GetTask(taskID); err == nil {
			return task, nil
		}
	}

	return nil, errors.New(errors.CodeResourceNotFound, "Task not found: "+taskID)
}

// StopAll 停止所有协程池
func (m *TaskManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	// 停止默认协程池
	if err := m.defaultPool.Stop(); err != nil {
		errors = append(errors, err)
	}

	// 停止其他协程池
	for name, pool := range m.pools {
		if err := pool.Stop(); err != nil {
			errors = append(errors, err)
			logger.Error("Failed to stop worker pool",
				logger.String("name", name),
				logger.ErrorField(err),
			)
		} else {
			logger.Info("Worker pool stopped",
				logger.String("name", name),
			)
		}
	}

	// 清空协程池映射
	m.pools = make(map[string]*WorkerPool)

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping worker pools: %v", errors)
	}

	logger.Info("All worker pools stopped successfully")
	return nil
}

// GetStats 获取所有协程池的统计信息
func (m *TaskManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})

	// 默认协程池统计
	stats["default"] = m.defaultPool.GetStats()

	// 其他协程池统计
	for name, pool := range m.pools {
		stats[name] = pool.GetStats()
	}

	return stats
}

// 快捷方法

// SubmitTask 提交任务（快捷方式）
func SubmitTask(task *Task) (string, error) {
	return GetTaskManager().Submit(task)
}

// SubmitTaskToPool 提交任务到指定协程池（快捷方式）
func SubmitTaskToPool(poolName string, task *Task) (string, error) {
	return GetTaskManager().SubmitToPool(poolName, task)
}

// GetTaskByID 根据ID获取任务（快捷方式）
func GetTaskByID(taskID string) (*Task, error) {
	return GetTaskManager().GetTask(taskID)
}

// CancelTask 取消任务（TaskManager 的方法）
func (m *TaskManager) CancelTask(taskID string) error {
	// 在默认协程池中尝试取消
	if err := m.defaultPool.CancelTask(taskID); err == nil {
		return nil
	}

	// 在其他协程池中尝试取消
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pool := range m.pools {
		if err := pool.CancelTask(taskID); err == nil {
			return nil
		}
	}

	return errors.New(errors.CodeResourceNotFound, "Task not found: "+taskID)
}

// CancelTaskInternal 取消任务（内部快捷方式，重命名避免冲突）
func CancelTaskInternal(taskID string) error {
	// 在所有协程池中查找并取消任务
	manager := GetTaskManager()

	// 在默认协程池中尝试取消
	if err := manager.defaultPool.CancelTask(taskID); err == nil {
		return nil
	}

	// 在其他协程池中尝试取消
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	for _, pool := range manager.pools {
		if err := pool.CancelTask(taskID); err == nil {
			return nil
		}
	}

	return errors.New(errors.CodeResourceNotFound, "Task not found: "+taskID)
}
