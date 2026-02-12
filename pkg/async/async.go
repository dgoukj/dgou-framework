package async

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	pkgErrors "github.com/pkg/errors"
)

// Task 任务定义
type Task struct {
	ID         string
	Name       string
	Handler    TaskHandler
	Payload    interface{}
	Priority   int // 越小优先级越高
	MaxRetries int
	RetryDelay time.Duration
	Timeout    time.Duration

	// 内部状态
	state       TaskState
	retryCount  int
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	result      interface{}
	err         error
	done        chan struct{}
	createdAt   time.Time
	startedAt   *time.Time
	completedAt *time.Time
}

type TaskState int32

const (
	TaskStatePending TaskState = iota
	TaskStateRunning
	TaskStateSuccess
	TaskStateFailed
	TaskStateTimeout
	TaskStateCancelled
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

type TaskHandler func(ctx context.Context, payload interface{}) (interface{}, error)

// NewTask 创建新任务
func NewTask(name string, handler TaskHandler, payload interface{}) *Task {
	return &Task{
		ID:         uuid.New().String(),
		Name:       name,
		Handler:    handler,
		Payload:    payload,
		Priority:   0,
		MaxRetries: 3,
		RetryDelay: time.Second,
		Timeout:    30 * time.Second,
		state:      TaskStatePending,
		done:       make(chan struct{}),
		createdAt:  time.Now(),
	}
}

// WithPriority 设置优先级
func (t *Task) WithPriority(p int) *Task {
	t.Priority = p
	return t
}

// WithRetries 设置重试策略
func (t *Task) WithRetries(max int, delay time.Duration) *Task {
	t.MaxRetries = max
	t.RetryDelay = delay
	return t
}

// WithTimeout 设置超时
func (t *Task) WithTimeout(timeout time.Duration) *Task {
	t.Timeout = timeout
	return t
}

// Wait 等待任务完成
func (t *Task) Wait(timeout time.Duration) bool {
	select {
	case <-t.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Result 返回任务结果
func (t *Task) Result() (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == TaskStateSuccess || t.state == TaskStateFailed ||
		t.state == TaskStateTimeout || t.state == TaskStateCancelled {
		return t.result, t.err
	}
	return nil, pkgErrors.New("task not completed")
}

// State 返回任务状态
func (t *Task) State() TaskState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// Cancel 取消任务
func (t *Task) Cancel() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == TaskStateRunning && t.cancel != nil {
		t.cancel()
		t.state = TaskStateCancelled
		t.completedAt = nowPtr()
		t.err = pkgErrors.New("task cancelled")
		close(t.done)
		return true
	}
	return false
}

// execute 执行任务（内部调用）
func (t *Task) execute(ctx context.Context) {
	t.mu.Lock()
	if t.state != TaskStatePending {
		t.mu.Unlock()
		return
	}
	t.state = TaskStateRunning
	now := time.Now()
	t.startedAt = &now
	var taskCtx context.Context
	taskCtx, t.cancel = context.WithTimeout(ctx, t.Timeout)
	t.mu.Unlock()

	var result interface{}
	var err error

	// 重试循环
	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		select {
		case <-taskCtx.Done():
			t.mu.Lock()
			if pkgErrors.Is(taskCtx.Err(), context.DeadlineExceeded) {
				t.state = TaskStateTimeout
			} else {
				t.state = TaskStateCancelled
			}
			t.completedAt = nowPtr()
			t.err = taskCtx.Err()
			close(t.done)
			t.mu.Unlock()
			return
		default:
		}

		// 执行用户 handler
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = pkgErrors.Errorf("panic: %v", r)
				}
			}()
			result, err = t.Handler(taskCtx, t.Payload)
		}()

		if err == nil {
			t.mu.Lock()
			t.state = TaskStateSuccess
			t.result = result
			t.completedAt = nowPtr()
			close(t.done)
			t.mu.Unlock()
			return
		}

		// 失败，准备重试
		t.mu.Lock()
		t.retryCount = attempt + 1
		t.mu.Unlock()

		if attempt >= t.MaxRetries {
			break
		}

		// 等待重试延迟
		select {
		case <-time.After(t.RetryDelay):
			continue
		case <-taskCtx.Done():
			t.mu.Lock()
			t.state = TaskStateTimeout
			t.completedAt = nowPtr()
			t.err = taskCtx.Err()
			close(t.done)
			t.mu.Unlock()
			return
		}
	}

	// 重试耗尽
	t.mu.Lock()
	t.state = TaskStateFailed
	t.err = err
	t.completedAt = nowPtr()
	close(t.done)
	t.mu.Unlock()
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}

// Pool 协程池
type Pool struct {
	name       string
	maxWorkers int
	maxQueue   int
	queue      *priorityQueue
	workers    []*worker
	mu         sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	running    int32
}

type priorityQueue []*Task

func (pq priorityQueue) Len() int            { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool  { return pq[i].Priority < pq[j].Priority }
func (pq priorityQueue) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x interface{}) { *pq = append(*pq, x.(*Task)) }
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	x := old[n-1]
	*pq = old[0 : n-1]
	return x
}

type worker struct {
	id   int
	pool *Pool
	ch   chan *Task
	quit chan struct{}
}

func NewPool(name string, maxWorkers, maxQueue int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		name:       name,
		maxWorkers: maxWorkers,
		maxQueue:   maxQueue,
		queue:      &priorityQueue{},
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (p *Pool) Start() error {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return pkgErrors.New("pool already running")
	}
	heap.Init(p.queue)
	p.workers = make([]*worker, p.maxWorkers)
	for i := 0; i < p.maxWorkers; i++ {
		w := &worker{
			id:   i,
			pool: p,
			ch:   make(chan *Task, 1),
			quit: make(chan struct{}),
		}
		p.workers[i] = w
		p.wg.Add(1)
		go w.run()
	}
	go p.dispatch()
	return nil
}

func (p *Pool) Stop() {
	atomic.StoreInt32(&p.running, 0)
	p.cancel()
	for _, w := range p.workers {
		close(w.quit)
	}
	p.wg.Wait()
}

func (p *Pool) Submit(task *Task) error {
	if atomic.LoadInt32(&p.running) == 0 {
		return pkgErrors.New("pool not running")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.queue.Len() >= p.maxQueue {
		return pkgErrors.New("queue full")
	}
	heap.Push(p.queue, task)
	return nil
}

func (p *Pool) dispatch() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}
		p.mu.Lock()
		if p.queue.Len() == 0 {
			p.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			continue
		}
		task := heap.Pop(p.queue).(*Task)
		p.mu.Unlock()

		// 寻找空闲 worker
		var w *worker
		for _, worker := range p.workers {
			select {
			case worker.ch <- task:
				w = worker
				break
			default:
			}
			if w != nil {
				break
			}
		}
		if w == nil {
			// 无空闲 worker，重新入队
			p.mu.Lock()
			heap.Push(p.queue, task)
			p.mu.Unlock()
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func (w *worker) run() {
	defer w.pool.wg.Done()
	for {
		select {
		case task := <-w.ch:
			task.execute(w.pool.ctx)
		case <-w.quit:
			return
		case <-w.pool.ctx.Done():
			return
		}
	}
}
