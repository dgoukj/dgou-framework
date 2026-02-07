// file: pkg/async/store.go
package async

import (
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"sync"
	"time"
)

// TaskStore 任务存储
type TaskStore struct {
	tasks map[string]*Task // 任务映射
	mu    sync.RWMutex     // 读写锁
	stats *StoreStats      // 统计信息
}

// StoreStats 存储统计
type StoreStats struct {
	Total     int64 `json:"total"`
	Pending   int64 `json:"pending"`
	Running   int64 `json:"running"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// NewTaskStore 创建任务存储
func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks: make(map[string]*Task),
		stats: &StoreStats{},
	}
}

// Add 添加任务
func (s *TaskStore) Add(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	s.stats.Total++
	s.stats.Pending++
}

// Get 获取任务
func (s *TaskStore) Get(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, errors.New(errors.CodeResourceNotFound,
			"Task not found: "+taskID)
	}

	return task, nil
}

// Update 更新任务状态
func (s *TaskStore) Update(task *Task, oldState, newState TaskState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新统计信息
	s.updateStats(oldState, newState)

	// 更新任务
	s.tasks[task.ID] = task
}

// updateStats 更新统计信息
func (s *TaskStore) updateStats(oldState, newState TaskState) {
	// 减少旧状态的计数
	switch oldState {
	case TaskStatePending:
		s.stats.Pending--
	case TaskStateRunning:
		s.stats.Running--
	case TaskStateSuccess, TaskStateFailed, TaskStateTimeout, TaskStateCancelled:
		s.stats.Completed--
	}

	// 增加新状态的计数
	switch newState {
	case TaskStatePending:
		s.stats.Pending++
	case TaskStateRunning:
		s.stats.Running++
	case TaskStateSuccess:
		s.stats.Completed++
	case TaskStateFailed, TaskStateTimeout, TaskStateCancelled:
		s.stats.Completed++
		s.stats.Failed++
	}
}

// Delete 删除任务
func (s *TaskStore) Delete(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.tasks[taskID]; exists {
		// 更新统计信息
		switch task.State {
		case TaskStatePending:
			s.stats.Pending--
		case TaskStateRunning:
			s.stats.Running--
		case TaskStateSuccess, TaskStateFailed, TaskStateTimeout, TaskStateCancelled:
			s.stats.Completed--
		}
		s.stats.Total--

		delete(s.tasks, taskID)
	}
}

// Cleanup 清理过期任务
func (s *TaskStore) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deletedCount := 0

	for taskID, task := range s.tasks {
		// 只清理已完成的任务
		if task.IsCompleted() {
			completedAt := task.CompletedAt
			if completedAt != nil && now.Sub(*completedAt) > maxAge {
				s.deleteTask(task)
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		logger.Debug("Cleaned up expired tasks",
			logger.Int("count", deletedCount),
		)
	}

	return deletedCount
}

// deleteTask 删除任务（不获取锁，由调用者确保线程安全）
func (s *TaskStore) deleteTask(task *Task) {
	// 更新统计信息
	switch task.State {
	case TaskStatePending:
		s.stats.Pending--
	case TaskStateRunning:
		s.stats.Running--
	case TaskStateSuccess, TaskStateFailed, TaskStateTimeout, TaskStateCancelled:
		s.stats.Completed--
	}
	s.stats.Total--

	delete(s.tasks, task.ID)
}

// GetStats 获取统计信息
func (s *TaskStore) GetStats() *StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	return &StoreStats{
		Total:     s.stats.Total,
		Pending:   s.stats.Pending,
		Running:   s.stats.Running,
		Completed: s.stats.Completed,
		Failed:    s.stats.Failed,
	}
}

// GetAll 获取所有任务
func (s *TaskStore) GetAll() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// GetByState 根据状态获取任务
func (s *TaskStore) GetByState(state TaskState) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range s.tasks {
		if task.State == state {
			tasks = append(tasks, task)
		}
	}

	return tasks
}
