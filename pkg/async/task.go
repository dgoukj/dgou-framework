// file: pkg/async/task.go
package async

import (
	"context"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// TaskState 任务状态
type TaskState int32

const (
	TaskStatePending   TaskState = iota // 等待中
	TaskStateRunning                    // 执行中
	TaskStateSuccess                    // 成功
	TaskStateFailed                     // 失败
	TaskStateTimeout                    // 超时
	TaskStateCancelled                  // 已取消
)

func (s TaskState) String() string {
	switch s {
	case TaskStatePending:
		return "pending"
	case TaskStateRunning:
		return "running"
	case TaskStateSuccess:
		return "success"
	case TaskStateFailed:
		return "failed"
	case TaskStateTimeout:
		return "timeout"
	case TaskStateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Priority 任务优先级
type Priority int

const (
	PriorityLow      Priority = iota // 低优先级
	PriorityNormal                   // 正常优先级
	PriorityHigh                     // 高优先级
	PriorityCritical                 // 关键优先级
)

// TaskResult 任务结果
type TaskResult struct {
	Value interface{} `json:"value,omitempty"`
	Error error       `json:"error,omitempty"`
}

// TaskHandler 任务处理函数
type TaskHandler func(ctx context.Context, params interface{}) (interface{}, error)

// Task 任务定义
type Task struct {
	ID          string                 `json:"id"`                     // 任务ID
	Name        string                 `json:"name"`                   // 任务名称
	Handler     TaskHandler            `json:"-"`                      // 任务处理函数
	Params      interface{}            `json:"params,omitempty"`       // 任务参数
	Priority    Priority               `json:"priority"`               // 任务优先级
	MaxRetries  int                    `json:"max_retries"`            // 最大重试次数
	RetryDelay  time.Duration          `json:"retry_delay"`            // 重试延迟
	Timeout     time.Duration          `json:"timeout"`                // 执行超时
	CreatedAt   time.Time              `json:"created_at"`             // 创建时间
	StartedAt   *time.Time             `json:"started_at,omitempty"`   // 开始时间
	CompletedAt *time.Time             `json:"completed_at,omitempty"` // 完成时间
	State       TaskState              `json:"state"`                  // 任务状态
	Result      *TaskResult            `json:"result,omitempty"`       // 任务结果
	RetryCount  int                    `json:"retry_count"`            // 当前重试次数
	Metadata    map[string]interface{} `json:"metadata,omitempty"`     // 元数据
	parentCtx   context.Context        `json:"-"`                      // 父上下文
	cancel      context.CancelFunc     `json:"-"`                      // 取消函数
	mu          sync.RWMutex           `json:"-"`                      // 读写锁
	notifyCh    chan struct{}          `json:"-"`                      // 通知通道
}

// NewTask 创建新任务
func NewTask(name string, handler TaskHandler, params interface{}) *Task {
	id := generateTaskID()
	return &Task{
		ID:         id,
		Name:       name,
		Handler:    handler,
		Params:     params,
		Priority:   PriorityNormal,
		MaxRetries: 3,
		RetryDelay: time.Second,
		Timeout:    30 * time.Second,
		CreatedAt:  time.Now(),
		State:      TaskStatePending,
		Metadata:   make(map[string]interface{}),
		notifyCh:   make(chan struct{}, 1),
	}
}

// WithPriority 设置优先级
func (t *Task) WithPriority(priority Priority) *Task {
	t.Priority = priority
	return t
}

// WithRetries 设置重试策略
func (t *Task) WithRetries(maxRetries int, retryDelay time.Duration) *Task {
	t.MaxRetries = maxRetries
	t.RetryDelay = retryDelay
	return t
}

// WithTimeout 设置超时时间
func (t *Task) WithTimeout(timeout time.Duration) *Task {
	t.Timeout = timeout
	return t
}

// WithMetadata 设置元数据
func (t *Task) WithMetadata(key string, value interface{}) *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Metadata[key] = value
	return t
}

// GetMetadata 获取元数据
func (t *Task) GetMetadata(key string) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	value, exists := t.Metadata[key]
	return value, exists
}

// Execute 执行任务
func (t *Task) Execute(ctx context.Context) error {
	t.mu.Lock()
	if t.State != TaskStatePending {
		t.mu.Unlock()
		return errors.New(errors.CodeValidationFailed, "Task is not in pending state")
	}

	// 设置任务状态为执行中
	t.State = TaskStateRunning
	now := time.Now()
	t.StartedAt = &now
	t.parentCtx = ctx

	// 创建带超时的上下文
	var taskCtx context.Context
	taskCtx, t.cancel = context.WithTimeout(ctx, t.Timeout)
	t.mu.Unlock()

	// 通知状态变化
	t.notifyStateChange()

	// 执行任务（支持重试）
	var result *TaskResult
	for t.RetryCount <= t.MaxRetries {
		select {
		case <-taskCtx.Done():
			// 任务超时或被取消
			t.mu.Lock()
			if errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
				t.State = TaskStateTimeout
			} else {
				t.State = TaskStateCancelled
			}
			now := time.Now()
			t.CompletedAt = &now
			t.Result = &TaskResult{Error: taskCtx.Err()}
			t.mu.Unlock()
			t.notifyStateChange()
			return taskCtx.Err()

		default:
			// 执行任务
			func() {
				defer func() {
					if r := recover(); r != nil {
						stack := debug.Stack()
						err := fmt.Errorf("panic in task execution: %v\n%s", r, stack)
						result = &TaskResult{Error: err}
						logger.Error("Task panic recovered",
							logger.String("task_id", t.ID),
							logger.String("task_name", t.Name),
							logger.Any("panic", r),
							logger.String("stack", string(stack)),
						)
					}
				}()

				value, err := t.Handler(taskCtx, t.Params)
				result = &TaskResult{Value: value, Error: err}
			}()

			// 检查执行结果
			if result.Error == nil {
				// 执行成功
				t.mu.Lock()
				t.State = TaskStateSuccess
				now := time.Now()
				t.CompletedAt = &now
				t.Result = result
				t.mu.Unlock()
				t.notifyStateChange()
				return nil
			}

			// 执行失败，检查是否重试
			t.mu.Lock()
			t.RetryCount++
			if t.RetryCount > t.MaxRetries {
				// 重试次数用完
				t.State = TaskStateFailed
				now := time.Now()
				t.CompletedAt = &now
				t.Result = result
				t.mu.Unlock()
				t.notifyStateChange()
				return result.Error
			}
			t.mu.Unlock()

			// 等待重试延迟
			logger.Warn("Task execution failed, retrying",
				logger.String("task_id", t.ID),
				logger.String("task_name", t.Name),
				logger.ErrorField(result.Error),
				logger.Int("retry_count", t.RetryCount),
				logger.Int("max_retries", t.MaxRetries),
			)

			select {
			case <-time.After(t.RetryDelay):
				// 继续重试
				continue
			case <-taskCtx.Done():
				// 任务超时或被取消
				t.mu.Lock()
				if errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
					t.State = TaskStateTimeout
				} else {
					t.State = TaskStateCancelled
				}
				now := time.Now()
				t.CompletedAt = &now
				t.Result = &TaskResult{Error: taskCtx.Err()}
				t.mu.Unlock()
				t.notifyStateChange()
				return taskCtx.Err()
			}
		}
	}

	return nil
}

// Cancel 取消任务
func (t *Task) Cancel() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State == TaskStateRunning && t.cancel != nil {
		t.cancel()
		t.State = TaskStateCancelled
		now := time.Now()
		t.CompletedAt = &now
		t.Result = &TaskResult{Error: errors.New(errors.CodeInternalError, "Task cancelled")}
		t.notifyStateChange()
		return true
	}

	return false
}

// Wait 等待任务完成
func (t *Task) Wait(timeout time.Duration) bool {
	select {
	case <-t.notifyCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

// GetState 获取任务状态
func (t *Task) GetState() TaskState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.State
}

// GetResult 获取任务结果
func (t *Task) GetResult() (*TaskResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.State != TaskStateSuccess && t.State != TaskStateFailed &&
		t.State != TaskStateTimeout && t.State != TaskStateCancelled {
		return nil, errors.New(errors.CodeValidationFailed, "Task is not completed")
	}

	return t.Result, nil
}

// IsCompleted 检查任务是否完成
func (t *Task) IsCompleted() bool {
	state := t.GetState()
	return state == TaskStateSuccess || state == TaskStateFailed ||
		state == TaskStateTimeout || state == TaskStateCancelled
}

// notifyStateChange 通知状态变化
func (t *Task) notifyStateChange() {
	select {
	case t.notifyCh <- struct{}{}:
	default:
		// 通道已满，忽略
	}
}

// taskIDCounter 任务ID计数器
var taskIDCounter uint64

// generateTaskID 生成任务ID
func generateTaskID() string {
	id := atomic.AddUint64(&taskIDCounter, 1)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("task_%d_%d", timestamp, id)
}
