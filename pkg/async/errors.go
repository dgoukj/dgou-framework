// file: pkg/async/errors.go
package async

import "dgou/pkg/errors"

var (
	// 异步任务相关错误码
	ErrTaskNotFound      = errors.New(errors.CodeResourceNotFound, "Task not found")
	ErrTaskQueueFull     = errors.New(errors.CodeTooManyRequests, "Task queue is full")
	ErrWorkerPoolStopped = errors.New(errors.CodeInternalError, "Worker pool is stopped")
	ErrTaskTimeout       = errors.New(errors.CodeInternalError, "Task execution timeout")
	ErrTaskCancelled     = errors.New(errors.CodeInternalError, "Task cancelled")
)

// IsTaskNotFound 检查是否为任务未找到错误
func IsTaskNotFound(err error) bool {
	return errors.Is(err, ErrTaskNotFound)
}

// IsTaskQueueFull 检查是否为任务队列已满错误
func IsTaskQueueFull(err error) bool {
	return errors.Is(err, ErrTaskQueueFull)
}
