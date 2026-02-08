# Dgou Framework - 生产级 Go Gin 脚手架
## 9. 异步任务组件 (pkg/async)

### 特性
- ✅ **协程池管理**：可配置的协程池，支持动态调整工作协程数
- ✅ **任务优先级**：4级优先级（低、正常、高、关键），支持优先队列
- ✅ **失败重试机制**：可配置的重试次数和重试延迟，支持指数退避
- ✅ **任务状态追踪**：完整的任务生命周期管理，实时状态监控
- ✅ **任务结果查询**：支持任务结果持久化和查询
- ✅ **任务超时控制**：每个任务可配置独立超时时间
- ✅ **优雅关闭**：支持优雅关闭，确保任务完成不丢失
- ✅ **详细指标监控**：全面的性能指标和统计信息
- ✅ **任务取消支持**：可随时取消正在执行的任务
- ✅ **防内存泄漏**：自动清理过期任务，防止内存泄漏

### 快速开始

#### 基本配置

```yaml
  # config/config.yaml
async:
    max_workers: 100           # 最大工作协程数
    max_queue_size: 10000      # 最大队列大小
    worker_idle_time: 30s      # 工作协程空闲时间
    enable_metrics: true       # 是否启用指标
```
#### 初始化任务管理器
```go

import (
    "dgou/pkg/async"
    "dgou/pkg/config"
)

func main() {
    // 加载配置
    cfg := config.LoadConfig()
    
    // 初始化任务管理器
    _, err := async.InitTaskManager(cfg)
    if err != nil {
        log.Fatal(err)
    }
}
```
#### 创建并提交任务
```go

// 定义任务处理函数
func processImage(ctx context.Context, params interface{}) (interface{}, error) {
    imageData, ok := params.([]byte)
    if !ok {
        return nil, errors.New("Invalid image data")
    }
    
    // 处理图片
    result, err := resizeImage(imageData)
    if err != nil {
        return nil, err
    }
    
    return result, nil
}
    
// 创建任务
task := async.NewTask("process_image", processImage, imageBytes).
WithPriority(async.PriorityHigh).    // 设置高优先级
WithRetries(3, time.Second).         // 设置重试3次，每次间隔1秒
WithTimeout(30 * time.Second).       // 设置30秒超时
WithMetadata("user_id", userID)      // 添加元数据

// 提交任务
taskID, err := async.SubmitTask(task)
if err != nil {
     log.Printf("Failed to submit task: %v", err)
    return
}

log.Printf("Task submitted: %s", taskID)
```
#### 等待任务完成并获取结果
```go

// 等待任务完成（阻塞方式）
result, err := async.SubmitAndWait(task, 60*time.Second)
if err != nil {
	log.Printf("Task failed: %v", err)
    return
}

log.Printf("Task completed: %v", result.Value)

// 或者异步等待
go func() {
    // 等待任务完成
    if task.Wait(60 * time.Second) {
        result, err := task.GetResult()
        if err != nil {
             log.Printf("Task failed: %v", err)
        } else {
             log.Printf("Task completed successfully: %v", result.Value)
        }
    } else {
        log.Println("Task wait timeout")
    }
}()
```
#### 查询任务状态
```go

// 根据任务ID查询任务
task, err := async.GetTaskByID(taskID)
if err != nil {
	log.Printf("Failed to get task: %v", err)
    return
}

// 获取任务状态
state := task.GetState()
log.Printf("Task state: %s", state)

// 检查任务是否完成
if task.IsCompleted() {
    result, err := task.GetResult()
    if err != nil {
         log.Printf("Task failed: %v", err)
    } else {
         log.Printf("Task result: %v", result.Value)
    }
}
```
#### 高级用法
##### 创建自定义协程池
```go

// 创建协程池配置
config := &async.PoolConfig{
     MaxWorkers:     50,               // 50个工作协程
     MaxQueueSize:   5000,             // 最大队列大小5000
     WorkerIdleTime: 60 * time.Second, // 空闲60秒后回收
     EnableMetrics:  true,             // 启用指标
}

// 创建自定义协程池
pool, err := async.GetTaskManager().CreatePool("image_processing", config)
if err != nil {
    log.Fatal(err)
}

// 提交任务到自定义协程池
taskID, err := async.SubmitTaskToPool("image_processing", task)
if err != nil {
     log.Printf("Failed to submit task: %v", err)
    return
}
```
##### 任务优先级示例
```go

// 创建不同优先级的任务
lowPriorityTask := async.NewTask("low_task", handler, params).
WithPriority(async.PriorityLow)   // 低优先级

normalPriorityTask := async.NewTask("normal_task", handler, params).
WithPriority(async.PriorityNormal) // 正常优先级

highPriorityTask := async.NewTask("high_task", handler, params).
WithPriority(async.PriorityHigh)   // 高优先级

criticalPriorityTask := async.NewTask("critical_task", handler, params).
WithPriority(async.PriorityCritical) // 关键优先级

// 高优先级任务会先执行，即使后提交
```
##### 任务重试策略
```go

// 指数退避重试
task := async.NewTask("retry_task", handler, params).
WithRetries(5, time.Second). // 重试5次，每次间隔1秒
WithMetadata("max_retries", 5)

// 或者在任务处理函数中实现自定义重试逻辑
func handlerWithRetry(ctx context.Context, params interface{}) (interface{}, error) {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        result, err := doSomething(params)
        if err == nil {
            return result, nil
        }
        
        // 指数退避
        delay := time.Duration(math.Pow(2, float64(i))) * time.Second
        select {
             case <-time.After(delay):
               continue
             case <-ctx.Done():
               return nil, ctx.Err()
        }
    }
    return nil, errors.New("Max retries exceeded")
}
```
##### 任务依赖关系
```go

// 创建有依赖关系的任务链
func createTaskChain() {
    // 第一步任务
    task1 := async.NewTask("step1", step1Handler, params1)
    task1ID, _ := async.SubmitTask(task1)
    
    // 等待第一步完成
    if task1.Wait(30 * time.Second) {
        // 第二步任务，依赖第一步的结果
        task2 := async.NewTask("step2", step2Handler, params2)
        task2.WithMetadata("depends_on", task1ID)
        async.SubmitTask(task2)
    }
}

// 或者使用任务编排器
func createWorkflow() {
    tasks := []*async.Task{
        async.NewTask("step1", step1Handler, params1),
        async.NewTask("step2", step2Handler, params2),
        async.NewTask("step3", step3Handler, params3),
    }
    
    // 顺序执行
    for _, task := range tasks {
        if _, err := async.SubmitAndWait(task, 30*time.Second); err != nil {
             log.Printf("Task failed, stopping workflow: %v", err)
            break
        }
    }
}
```
##### 批量任务处理
```go

// 批量提交任务
func processBatch(items []Item) []string {
    taskIDs := make([]string, 0, len(items))
    
    for _, item := range items {
        task := async.NewTask("process_item", processItem, item)
        taskID, err := async.SubmitTask(task)
        if err != nil {
             log.Printf("Failed to submit task for item %v: %v", item.ID, err)
        continue
        }
        taskIDs = append(taskIDs, taskID)
    }
    
    return taskIDs
}

// 等待批量任务完成
func waitForBatch(taskIDs []string, timeout time.Duration) map[string]*async.TaskResult {
    results := make(map[string]*async.TaskResult)
    
    for _, taskID := range taskIDs {
        task, err := async.GetTaskByID(taskID)
        if err != nil {
             results[taskID] = &async.TaskResult{Error: err}
        continue
        }
        
        if task.Wait(timeout) {
        result, err := task.GetResult()
        if err != nil {
             results[taskID] = &async.TaskResult{Error: err}
        } else {
        results[taskID] = result
        }
        } else {
             results[taskID] = &async.TaskResult{Error: errors.New("Timeout")}
        }
    }
    
    return results
}
```
##### 任务取消和超时处理
```go

// 取消任务
func cancelTask(taskID string) error {
    return async.CancelTask(taskID)
}

// 带超时的任务执行
func executeWithTimeout(handler async.TaskHandler, params interface{}, timeout time.Duration) (interface{}, error) {
    task := async.NewTask("timeout_task", handler, params).
    WithTimeout(timeout)

    return async.SubmitAndWait(task, timeout+5*time.Second)
}

// 在任务处理函数中检查上下文
func handlerWithContextCheck(ctx context.Context, params interface{}) (interface{}, error) {
    // 定期检查上下文是否被取消
    for i := 0; i < 10; i++ {
        select {
             case <-ctx.Done():
               return nil, ctx.Err() // 任务被取消或超时
             default:
               // 继续执行
        }
        
        // 执行一部分工作
        if err := doPartialWork(params, i); err != nil {
            return nil, err
        }
    
        // 等待一段时间
        select {
             case <-time.After(100 * time.Millisecond):
               continue
             case <-ctx.Done():
               return nil, ctx.Err()
        }
    }
    
    return "completed", nil
}
```
##### 监控和指标
```go

// 获取协程池统计信息
func monitorPool() {
    manager := async.GetTaskManager()
    stats := manager.GetStats()
    
    fmt.Printf("Pool Statistics:\n")
    for poolName, poolStats := range stats {
         fmt.Printf("  Pool: %s\n", poolName)
         fmt.Printf("    Active Workers: %v\n", poolStats["active_workers"])
         fmt.Printf("    Queue Size: %v/%v\n", poolStats["queue_size"], poolStats["max_queue_size"])
         fmt.Printf("    Completed Tasks: %v\n", poolStats["completed_tasks"])
         fmt.Printf("    Failed Tasks: %v\n", poolStats["failed_tasks"])
         fmt.Printf("    Average Process Time: %v\n", poolStats["avg_process_time"])
    }
}

// 获取任务存储统计
func monitorTasks() {
    // 获取所有任务状态
    manager := async.GetTaskManager()
    defaultPool := manager.defaultPool
    taskStore := defaultPool.taskStore
    
    stats := taskStore.GetStats()
    fmt.Printf("Task Statistics:\n")
         fmt.Printf("  Total Tasks: %d\n", stats.Total)
         fmt.Printf("  Pending Tasks: %d\n", stats.Pending)
         fmt.Printf("  Running Tasks: %d\n", stats.Running)
         fmt.Printf("  Completed Tasks: %d\n", stats.Completed)
         fmt.Printf("  Failed Tasks: %d\n", stats.Failed)
}
```
##### 优雅关闭
```go

// 在应用关闭时优雅停止协程池
func gracefulShutdown() {
    manager := async.GetTaskManager()
    
    // 停止接收新任务
    log.Println("Stopping task submission...")
    
    // 等待一段时间让现有任务完成
    time.Sleep(10 * time.Second)
    
    // 停止所有协程池
    if err := manager.StopAll(); err != nil {
         log.Printf("Error stopping worker pools: %v", err)
    }
    
    log.Println("All worker pools stopped")
}
```
#### 配置说明
##### 协程池配置
```yaml

async:
# 默认协程池配置
    default:
        max_workers: 100           # 最大工作协程数，根据CPU核心数调整
        max_queue_size: 10000      # 任务队列最大大小，防止内存溢出
        worker_idle_time: 30s      # 工作协程空闲时间，超时后回收
        enable_metrics: true       # 启用性能指标收集

# 自定义协程池配置
pools:
    image_processing:
        max_workers: 50          # 图片处理专用池
        max_queue_size: 5000
        worker_idle_time: 60s

email_sending:
    max_workers: 20          # 邮件发送专用池
    max_queue_size: 2000
    worker_idle_time: 30s
```
##### 任务配置

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| Priority | PriorityNormal | 任务优先级 |
| MaxRetries | 3 | 最大重试次数 |
| RetryDelay | 1s | 重试延迟时间 |
| Timeout | 30s | 任务执行超时时间 |

#### 最佳实践
##### 合理设置协程池大小
```go

// 根据CPU核心数设置协程池大小
func calculatePoolSize() int {
    // 通常设置为CPU核心数的2-4倍
    cpuCores := runtime.NumCPU()
        return cpuCores * 3
}
    
// I/O密集型任务可以设置更大的协程池
func getIOPoolConfig() *async.PoolConfig {
    return &async.PoolConfig{
         MaxWorkers:     200,  // I/O密集型任务可以更多
         MaxQueueSize:   10000,
         WorkerIdleTime: 60 * time.Second,
    }
}
```
##### 任务幂等性设计
```go

// 确保任务可以安全重试
func idempotentHandler(ctx context.Context, params interface{}) (interface{}, error) {
    taskID := ctx.Value("task_id").(string)

    // 检查任务是否已经执行过
    if executed, err := checkTaskExecuted(taskID); err != nil {
        return nil, err
    } else if executed {
        // 已经执行过，直接返回结果
        return getTaskResult(taskID), nil
    }

    // 执行任务
    result, err := doRealWork(params)
    if err != nil {
        return nil, err
    }
    
    // 保存执行结果
    if err := saveTaskResult(taskID, result); err != nil {
        return nil, err
    }
    
    return result, nil
}
```
##### 资源限制和熔断
```go

// 使用信号量限制并发
var semaphore = make(chan struct{}, 10) // 最大10个并发

func handlerWithLimiter(ctx context.Context, params interface{}) (interface{}, error) {
    select {
         case semaphore <- struct{}{}:
           defer func() { <-semaphore }()
         case <-ctx.Done():
           return nil, ctx.Err()
    }
    
    return doWork(params)
}

// 熔断器实现
type CircuitBreaker struct {
    failures     int
    maxFailures  int
    resetTimeout time.Duration
    lastFailure  time.Time
    mu           sync.RWMutex
}

func (cb *CircuitBreaker) Execute(handler async.TaskHandler, params interface{}) (interface{}, error) {
    cb.mu.RLock()
    if cb.failures >= cb.maxFailures && time.Since(cb.lastFailure) < cb.resetTimeout {
        cb.mu.RUnlock()
        return nil, errors.New("Circuit breaker open")
    }
    cb.mu.RUnlock()
    
    result, err := handler(context.Background(), params)
    
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
    } else {
        if time.Since(cb.lastFailure) > cb.resetTimeout {
            cb.failures = 0
        }
    }

    return result, err
}
```
##### 错误处理和日志
```go

// 详细的错误处理和日志记录
func handlerWithLogging(ctx context.Context, params interface{}) (interface{}, error) {
    taskID := ctx.Value("task_id").(string)
    startTime := time.Now()
    
    logger.Info("Task started",
        logger.String("task_id", taskID),
        logger.String("handler", "process_data"),
        logger.Time("start_time", startTime),
    )
    
    defer func() {
        duration := time.Since(startTime)
        logger.Info("Task completed",
        logger.String("task_id", taskID),
        logger.Duration("duration", duration),
        )
    }()
    
    // 执行实际工作
    result, err := doWork(params)
    if err != nil {
        logger.Error("Task failed",
            logger.String("task_id", taskID),
            logger.ErrorField(err),
            logger.Any("params", params),
        )
        return nil, err
    }
    
    return result, nil
}
```
#### 故障排除
##### 任务队列积压
```go

// 监控队列积压
func monitorQueueBacklog() {
    manager := async.GetTaskManager()
    stats := manager.GetStats()
    
    for poolName, poolStats := range stats {
        queueSize := poolStats["queue_size"].(int)
        maxQueueSize := poolStats["max_queue_size"].(int)
        
        if float64(queueSize) > float64(maxQueueSize)*0.8 {
            logger.Warn("Queue approaching capacity",
            logger.String("pool", poolName),
            logger.Int("queue_size", queueSize),
            logger.Int("max_queue_size", maxQueueSize),
            )
            
            // 可以考虑动态调整协程池大小
            // 或者实现负载均衡到其他协程池
        }
    }
}

// 自动扩容策略
func autoScalePool(pool *async.WorkerPool) {
    stats := pool.GetStats()
    queueSize := stats["queue_size"].(int)
    activeWorkers := stats["active_workers"].(int)
    
    // 如果队列积压且工作协程全忙，考虑扩容
    if queueSize > 100 && activeWorkers == pool.config.MaxWorkers {
        // 记录日志，建议管理员调整配置
        logger.Warn("Pool may need scaling",
        logger.Int("queue_size", queueSize),
        logger.Int("active_workers", activeWorkers),
        logger.Int("max_workers", pool.config.MaxWorkers),
        )
    }
}
```
##### 内存泄漏检测
```go

// 定期检查任务存储
func checkMemoryLeak() {
    manager := async.GetTaskManager()
    defaultPool := manager.defaultPool
    taskStore := defaultPool.taskStore
    
    // 清理24小时前的已完成任务
    cleaned := taskStore.Cleanup(24 * time.Hour)
    if cleaned > 0 {
        logger.Info("Cleaned up expired tasks",
        logger.Int("count", cleaned),
        )
    }
    
    // 检查任务存储大小
    stats := taskStore.GetStats()
    if stats.Total > 10000 {
        logger.Warn("Task store growing large",
            logger.Int64("total_tasks", stats.Total),
            logger.Int64("pending_tasks", stats.Pending),
            logger.Int64("completed_tasks", stats.Completed),
        )
    }
}
```
##### 死锁检测
```go

// 监控工作协程状态
func monitorWorkerHealth() {
    manager := async.GetTaskManager()
    
    for poolName, pool := range manager.pools {
        stats := pool.GetStats()
        activeWorkers := stats["active_workers"].(int)
        
        // 检查是否有工作协程长时间处于活动状态
        // 这可能表示死锁或长时间运行的任务
        if activeWorkers > 0 {
            // 实现更详细的监控逻辑
            logger.Debug("Worker health check",
            logger.String("pool", poolName),
            logger.Int("active_workers", activeWorkers),
            )
        }
    }
}
```

这个异步任务组件提供了完整的生产级解决方案，支持高并发、高可靠性的异步任务处理。您可以根据实际需求调整配置和使用方式。
