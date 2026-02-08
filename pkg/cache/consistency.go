package cache

import (
	"context"
	"dgou/pkg/logger"
	"fmt"
	"sync"
	"time"
)

// CacheConsistency 缓存一致性管理器
type CacheConsistency struct {
	cache        Cache                // 缓存实例
	mu           sync.RWMutex         // 读写锁
	consistency  *ConsistencyConfig   // 一致性配置
	invalidation *InvalidationManager // 失效管理器
}

// ConsistencyConfig 一致性配置
type ConsistencyConfig struct {
	EnableWriteThrough bool          `mapstructure:"enable_write_through"` // 是否启用写穿透
	EnableWriteBehind  bool          `mapstructure:"enable_write_behind"`  // 是否启用写回
	EnableReadThrough  bool          `mapstructure:"enable_read_through"`  // 是否启用读穿透
	InvalidationDelay  time.Duration `mapstructure:"invalidation_delay"`   // 失效延迟
	DoubleDeleteDelay  time.Duration `mapstructure:"double_delete_delay"`  // 双删延迟
}

// InvalidationManager 失效管理器
type InvalidationManager struct {
	queue    chan InvalidationTask // 失效队列
	workers  int                   // 工作协程数量
	stopChan chan bool             // 停止信号
	wg       sync.WaitGroup        // 等待组
}

// InvalidationTask 失效任务
type InvalidationTask struct {
	Key       string      `json:"key"`       // 缓存键
	Operation string      `json:"operation"` // 操作：delete, update
	Timestamp time.Time   `json:"timestamp"` // 时间戳
	Data      interface{} `json:"data"`      // 数据（可选）
}

// NewCacheConsistency 创建缓存一致性管理器
func NewCacheConsistency(cache Cache, config *ConsistencyConfig) *CacheConsistency {
	cc := &CacheConsistency{
		cache:       cache,
		consistency: config,
	}

	// 初始化失效管理器
	if config != nil {
		cc.invalidation = NewInvalidationManager(10, 1000)
	}

	return cc
}

// WriteThrough 写穿透：先写数据库，再更新缓存
func (cc *CacheConsistency) WriteThrough(ctx context.Context, key string, value interface{},
	dbWriter func() error, ttl time.Duration) error {

	// 1. 先写数据库
	if err := dbWriter(); err != nil {
		return fmt.Errorf("database write failed: %w", err)
	}

	// 2. 更新缓存
	if err := cc.cache.Set(ctx, key, value, ttl); err != nil {
		logger.Warn("Cache update failed after database write",
			logger.String("key", key),
			logger.ErrorField(err),
		)
		// 缓存更新失败不应影响整体操作
	}

	return nil
}

// WriteBehind 写回：先更新缓存，异步写数据库
func (cc *CacheConsistency) WriteBehind(ctx context.Context, key string, value interface{},
	dbWriter func() error, ttl time.Duration) error {

	// 1. 先更新缓存
	if err := cc.cache.Set(ctx, key, value, ttl); err != nil {
		return fmt.Errorf("cache update failed: %w", err)
	}

	// 2. 异步写数据库
	go func() {
		if err := dbWriter(); err != nil {
			logger.Error("Async database write failed",
				logger.String("key", key),
				logger.ErrorField(err),
			)
			// 数据库写入失败，需要处理（如重试、记录日志等）
		}
	}()

	return nil
}

// ReadThrough 读穿透：缓存未命中时从数据库加载
func (cc *CacheConsistency) ReadThrough(ctx context.Context, key string,
	dbLoader func() (interface{}, error), ttl time.Duration) (string, error) {

	// 1. 先尝试从缓存读取
	value, err := cc.cache.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	// 2. 缓存未命中，从数据库加载
	data, err := dbLoader()
	if err != nil {
		return "", fmt.Errorf("database load failed: %w", err)
	}

	// 3. 写入缓存
	if err := cc.cache.Set(ctx, key, data, ttl); err != nil {
		logger.Warn("Cache update failed after database load",
			logger.String("key", key),
			logger.ErrorField(err),
		)
	}

	return fmt.Sprintf("%v", data), nil
}

// DoubleDelete 双删策略：先删缓存，再更新数据库，延迟后再删一次缓存
func (cc *CacheConsistency) DoubleDelete(ctx context.Context, key string,
	dbUpdater func() error, delay time.Duration) error {

	// 1. 第一次删除缓存
	if err := cc.cache.Delete(ctx, key); err != nil {
		logger.Warn("First cache delete failed",
			logger.String("key", key),
			logger.ErrorField(err),
		)
	}

	// 2. 更新数据库
	if err := dbUpdater(); err != nil {
		return fmt.Errorf("database update failed: %w", err)
	}

	// 3. 延迟后第二次删除缓存
	go func() {
		if delay <= 0 {
			delay = cc.consistency.DoubleDeleteDelay
		}
		if delay <= 0 {
			delay = 1 * time.Second // 默认1秒
		}

		time.Sleep(delay)

		ctx2, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := cc.cache.Delete(ctx2, key); err != nil {
			logger.Warn("Second cache delete failed",
				logger.String("key", key),
				logger.ErrorField(err),
			)
		}
	}()

	return nil
}

// CacheAside 缓存旁路策略（经典模式）
func (cc *CacheConsistency) CacheAside(ctx context.Context, key string,
	dbLoader func() (interface{}, error), dbUpdater func() error, ttl time.Duration) (string, error) {

	// 读取：先读缓存，未命中读数据库，然后写入缓存
	value, err := cc.cache.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	data, err := dbLoader()
	if err != nil {
		return "", err
	}

	if err := cc.cache.Set(ctx, key, data, ttl); err != nil {
		logger.Warn("Cache update failed", logger.ErrorField(err))
	}

	return fmt.Sprintf("%v", data), nil
}

// PublishInvalidation 发布失效消息
func (cc *CacheConsistency) PublishInvalidation(ctx context.Context, channel string, key string) error {
	task := InvalidationTask{
		Key:       key,
		Operation: "invalidate",
		Timestamp: time.Now(),
	}

	// 发送到失效队列
	if cc.invalidation != nil {
		cc.invalidation.Enqueue(task)
	}

	// 发布消息
	if err := cc.cache.Publish(ctx, channel, task); err != nil {
		return fmt.Errorf("failed to publish invalidation: %w", err)
	}

	return nil
}

// SubscribeInvalidation 订阅失效消息
func (cc *CacheConsistency) SubscribeInvalidation(ctx context.Context, channel string,
	handler func(key string)) error {

	return cc.cache.Subscribe(ctx, channel, func(message string) {
		var task InvalidationTask
		// 解析消息
		// 这里简化处理，实际应该解析JSON
		logger.Info("Received invalidation message",
			logger.String("channel", channel),
			logger.String("message", message),
		)

		// 执行处理函数
		handler(task.Key)
	})
}

// NewInvalidationManager 创建失效管理器
func NewInvalidationManager(workers, queueSize int) *InvalidationManager {
	im := &InvalidationManager{
		queue:    make(chan InvalidationTask, queueSize),
		workers:  workers,
		stopChan: make(chan bool),
	}

	// 启动工作协程
	for i := 0; i < workers; i++ {
		im.wg.Add(1)
		go im.worker(i)
	}

	return im
}

// Enqueue 入队失效任务
func (im *InvalidationManager) Enqueue(task InvalidationTask) {
	select {
	case im.queue <- task:
		// 入队成功
	default:
		logger.Warn("Invalidation queue is full, dropping task",
			logger.String("key", task.Key),
		)
	}
}

// worker 工作协程
func (im *InvalidationManager) worker(id int) {
	defer im.wg.Done()

	logger.Debug("Invalidation worker started",
		logger.Int("worker_id", id),
	)

	for {
		select {
		case task := <-im.queue:
			im.processTask(task, id)
		case <-im.stopChan:
			logger.Debug("Invalidation worker stopped",
				logger.Int("worker_id", id),
			)
			return
		}
	}
}

// processTask 处理失效任务
func (im *InvalidationManager) processTask(task InvalidationTask, workerID int) {
	logger.Debug("Processing invalidation task",
		logger.Int("worker_id", workerID),
		logger.String("key", task.Key),
		logger.String("operation", task.Operation),
	)

	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 根据操作类型处理
	switch task.Operation {
	case "delete":
		// 删除缓存
		// 这里应该调用实际的缓存删除逻辑
		logger.Info("Invalidating cache key",
			logger.String("key", task.Key),
		)
	case "update":
		// 更新缓存
		logger.Info("Updating cache key",
			logger.String("key", task.Key),
		)
	}
}

// Stop 停止失效管理器
func (im *InvalidationManager) Stop() {
	close(im.stopChan)
	im.wg.Wait()
	close(im.queue)
}

// CacheCircuitBreaker 缓存熔断器
type CacheCircuitBreaker struct {
	failureThreshold int           // 失败阈值
	resetTimeout     time.Duration // 重置超时
	state            string        // 状态：closed, open, half-open
	failureCount     int           // 失败计数
	lastFailureTime  time.Time     // 最后失败时间
	mu               sync.RWMutex  // 读写锁
}

// NewCacheCircuitBreaker 创建缓存熔断器
func NewCacheCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CacheCircuitBreaker {
	return &CacheCircuitBreaker{
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		state:            "closed",
	}
}

// Execute 执行缓存操作
func (cb *CacheCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.Allow() {
		return fmt.Errorf("circuit breaker is open")
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	return err
}

// Allow 检查是否允许执行
func (cb *CacheCircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == "closed" {
		return true
	}

	if cb.state == "open" {
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = "half-open"
			return true
		}
		return false
	}

	// half-open状态允许尝试
	return true
}

// RecordFailure 记录失败
func (cb *CacheCircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.failureThreshold {
		cb.state = "open"
	}
}

// RecordSuccess 记录成功
func (cb *CacheCircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0
	cb.state = "closed"
}
