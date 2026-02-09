// file: pkg/async/pool.go
package async

import (
	"container/heap"
	"context"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"sync"
	"time"
)

// PoolConfig 协程池配置
type PoolConfig struct {
	MaxWorkers     int           `mapstructure:"max_workers"`      // 最大工作协程数
	MaxQueueSize   int           `mapstructure:"max_queue_size"`   // 最大队列大小
	WorkerIdleTime time.Duration `mapstructure:"worker_idle_time"` // 工作协程空闲时间
	EnableMetrics  bool          `mapstructure:"enable_metrics"`   // 是否启用指标
}

// DefaultPoolConfig 默认协程池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxWorkers:     100,
		MaxQueueSize:   10000,
		WorkerIdleTime: 30 * time.Second,
		EnableMetrics:  true,
	}
}

// WorkerPool 协程池
type WorkerPool struct {
	config          *PoolConfig        // 配置
	taskQueue       *PriorityQueue     // 任务队列
	workers         []*worker          // 工作协程
	taskStore       *TaskStore         // 任务存储
	metrics         *PoolMetrics       // 指标
	ctx             context.Context    // 上下文
	cancel          context.CancelFunc // 取消函数
	wg              sync.WaitGroup     // 等待组
	mu              sync.RWMutex       // 读写锁
	isRunning       bool               // 是否运行中
	shutdownCh      chan struct{}      // 关闭通道
	taskQueuedCh    chan *Task         // 任务入队通道
	taskProcessedCh chan *Task         // 任务处理完成通道
}

// worker 工作协程
type worker struct {
	id        int           // 协程ID
	pool      *WorkerPool   // 所属协程池
	taskCh    chan *Task    // 任务通道
	quitCh    chan struct{} // 退出通道
	lastWork  time.Time     // 最后工作时间
	isWorking bool          // 是否工作中
}

// NewWorkerPool 创建新的协程池
func NewWorkerPool(config *PoolConfig) *WorkerPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		config:          config,
		taskQueue:       NewPriorityQueue(),
		taskStore:       NewTaskStore(),
		metrics:         NewPoolMetrics(),
		ctx:             ctx,
		cancel:          cancel,
		shutdownCh:      make(chan struct{}),
		taskQueuedCh:    make(chan *Task, 1000),
		taskProcessedCh: make(chan *Task, 1000),
	}

	return pool
}

// Start 启动协程池
func (p *WorkerPool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return errors.New(errors.CodeInternalError, "Worker pool is already running")
	}

	logger.Info("Starting worker pool",
		logger.Int("max_workers", p.config.MaxWorkers),
		logger.Int("max_queue_size", p.config.MaxQueueSize),
	)

	// 初始化工作协程
	p.workers = make([]*worker, p.config.MaxWorkers)
	for i := 0; i < p.config.MaxWorkers; i++ {
		worker := &worker{
			id:     i,
			pool:   p,
			taskCh: make(chan *Task, 1),
			quitCh: make(chan struct{}),
		}
		p.workers[i] = worker

		// 启动工作协程
		p.wg.Add(1)
		go worker.run()
	}

	// 启动调度器
	p.wg.Add(1)
	go p.scheduler()

	// 启动指标收集器
	if p.config.EnableMetrics {
		p.wg.Add(1)
		go p.metricsCollector()
	}

	p.isRunning = true
	logger.Info("Worker pool started successfully")

	return nil
}

// Stop 停止协程池
func (p *WorkerPool) Stop() error {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	logger.Info("Stopping worker pool...")

	// 发送关闭信号
	close(p.shutdownCh)

	// 停止所有工作协程
	for _, worker := range p.workers {
		close(worker.quitCh)
	}

	// 取消上下文
	p.cancel()

	// 等待所有协程退出
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	// 等待超时
	select {
	case <-done:
		logger.Info("Worker pool stopped successfully")
		return nil
	case <-time.After(10 * time.Second):
		logger.Error("Worker pool stop timeout")
		return errors.New(errors.CodeInternalError, "Worker pool stop timeout")
	}
}

// Submit 提交任务
func (p *WorkerPool) Submit(task *Task) (string, error) {
	p.mu.RLock()
	if !p.isRunning {
		p.mu.RUnlock()
		return "", errors.New(errors.CodeInternalError, "Worker pool is not running")
	}
	p.mu.RUnlock()

	// 检查队列是否已满
	p.mu.Lock()
	if p.taskQueue.Len() >= p.config.MaxQueueSize {
		p.mu.Unlock()
		p.metrics.IncRejectedTasks()
		return "", errors.New(errors.CodeTooManyRequests, "Task queue is full")
	}

	// 将任务加入存储
	p.taskStore.Add(task)

	// 将任务加入优先队列
	heap.Push(p.taskQueue, &PriorityItem{
		Task:     task,
		Priority: int(task.Priority),
		Index:    p.taskQueue.Len(),
	})
	p.mu.Unlock()

	// 更新指标
	p.metrics.IncQueuedTasks()

	// 发送任务入队通知
	select {
	case p.taskQueuedCh <- task:
	default:
		// 通道已满，忽略
	}

	logger.Debug("Task submitted",
		logger.String("task_id", task.ID),
		logger.String("task_name", task.Name),
		logger.Any("priority", task.Priority),
	)

	return task.ID, nil
}

// SubmitAndWait 提交任务并等待完成
func (p *WorkerPool) SubmitAndWait(task *Task, timeout time.Duration) (*TaskResult, error) {
	_, err := p.Submit(task)
	if err != nil {
		return nil, err
	}

	// 等待任务完成
	if !task.Wait(timeout) {
		return nil, errors.New(errors.CodeInternalError, "Task wait timeout")
	}

	// 获取任务结果
	return task.GetResult()
}

// GetTask 获取任务信息
func (p *WorkerPool) GetTask(taskID string) (*Task, error) {
	return p.taskStore.Get(taskID)
}

// CancelTask 取消任务
func (p *WorkerPool) CancelTask(taskID string) error {
	task, err := p.taskStore.Get(taskID)
	if err != nil {
		return err
	}

	if task.Cancel() {
		p.metrics.IncCancelledTasks()
		return nil
	}

	return errors.New(errors.CodeValidationFailed, "Task cannot be cancelled")
}

// GetMetrics 获取协程池指标
func (p *WorkerPool) GetMetrics() *PoolMetrics {
	return p.metrics
}

// GetStats 获取协程池统计信息
func (p *WorkerPool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	activeWorkers := 0
	for _, w := range p.workers {
		if w.isWorking {
			activeWorkers++
		}
	}

	return map[string]interface{}{
		"is_running":       p.isRunning,
		"max_workers":      p.config.MaxWorkers,
		"active_workers":   activeWorkers,
		"idle_workers":     p.config.MaxWorkers - activeWorkers,
		"queue_size":       p.taskQueue.Len(),
		"max_queue_size":   p.config.MaxQueueSize,
		"total_tasks":      p.metrics.GetTotalTasks(),     // 使用 Get 方法
		"completed_tasks":  p.metrics.GetCompletedTasks(), // 使用 Get 方法
		"failed_tasks":     p.metrics.GetFailedTasks(),    // 使用 Get 方法
		"queued_tasks":     p.metrics.GetQueuedTasks(),    // 使用 Get 方法
		"rejected_tasks":   p.metrics.GetRejectedTasks(),  // 使用 Get 方法
		"avg_process_time": p.metrics.GetAverageProcessTime(),
		"task_states":      p.taskStore.GetStats(),
	}
}

// scheduler 任务调度器
func (p *WorkerPool) scheduler() {
	defer p.wg.Done()

	logger.Info("Task scheduler started")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			logger.Info("Task scheduler stopped")
			return

		case <-p.shutdownCh:
			logger.Info("Task scheduler stopping...")
			return

		case <-ticker.C:
			p.dispatchTasks()

		case task := <-p.taskProcessedCh:
			p.handleTaskProcessed(task)
		}
	}
}

// dispatchTasks 分发任务
func (p *WorkerPool) dispatchTasks() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果没有任务，直接返回
	if p.taskQueue.Len() == 0 {
		return
	}

	// 查找空闲的工作协程
	for _, worker := range p.workers {
		if !worker.isWorking {
			// 从优先队列中获取最高优先级的任务
			if p.taskQueue.Len() > 0 {
				item := heap.Pop(p.taskQueue).(*PriorityItem)
				task := item.Task

				// 将任务分配给工作协程
				select {
				case worker.taskCh <- task:
					worker.isWorking = true
					worker.lastWork = time.Now()
					p.metrics.IncActiveWorkers()

					logger.Debug("Task dispatched to worker",
						logger.String("task_id", task.ID),
						logger.String("task_name", task.Name),
						logger.Int("worker_id", worker.id),
					)
				default:
					// 工作协程忙，将任务重新放回队列
					heap.Push(p.taskQueue, item)
				}
			}
		} else {
			// 检查工作协程是否空闲超时
			if time.Since(worker.lastWork) > p.config.WorkerIdleTime*2 {
				logger.Warn("Worker seems stuck",
					logger.Int("worker_id", worker.id),
					logger.Duration("idle_time", time.Since(worker.lastWork)),
				)
			}
		}
	}
}

// handleTaskProcessed 处理任务完成
func (p *WorkerPool) handleTaskProcessed(task *Task) {
	// 更新指标
	p.metrics.DecActiveWorkers()
	p.metrics.IncCompletedTasks()

	state := task.GetState()
	if state == TaskStateFailed || state == TaskStateTimeout {
		p.metrics.IncFailedTasks()
	}

	// 记录任务处理完成
	logger.Debug("Task processed",
		logger.String("task_id", task.ID),
		logger.String("task_name", task.Name),
		logger.String("state", state.String()),
		logger.Duration("duration", time.Since(task.CreatedAt)),
	)

	// 清理过期的任务
	p.taskStore.Cleanup(24 * time.Hour)
}

// metricsCollector 指标收集器
func (p *WorkerPool) metricsCollector() {
	defer p.wg.Done()

	logger.Info("Metrics collector started")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			logger.Info("Metrics collector stopped")
			return

		case <-p.shutdownCh:
			logger.Info("Metrics collector stopping...")
			return

		case <-ticker.C:
			stats := p.GetStats()
			logger.Debug("Worker pool stats",
				logger.Any("stats", stats),
			)
		}
	}
}

// run 工作协程主循环
func (w *worker) run() {
	defer w.pool.wg.Done()

	logger.Debug("Worker started",
		logger.Int("worker_id", w.id),
	)

	for {
		select {
		case <-w.pool.ctx.Done():
			logger.Debug("Worker stopped by context",
				logger.Int("worker_id", w.id),
			)
			return

		case <-w.quitCh:
			logger.Debug("Worker stopped",
				logger.Int("worker_id", w.id),
			)
			return

		case task := <-w.taskCh:
			w.isWorking = true
			w.lastWork = time.Now()

			// 执行任务
			startTime := time.Now()
			err := task.Execute(w.pool.ctx)
			processTime := time.Since(startTime)

			w.isWorking = false
			w.pool.metrics.RecordProcessTime(processTime)

			// 发送任务处理完成通知
			select {
			case w.pool.taskProcessedCh <- task:
			default:
				// 通道已满，忽略
			}

			// 记录执行结果
			if err != nil {
				logger.Error("Task execution failed",
					logger.String("task_id", task.ID),
					logger.String("task_name", task.Name),
					logger.Int("worker_id", w.id),
					logger.ErrorField(err),
					logger.Duration("process_time", processTime),
				)
			}
		}
	}
}
