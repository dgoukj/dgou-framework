package app

import (
	"context"
	"dgou/pkg/logger"
	"sort"
	"sync"
	"time"
)

// ShutdownPriority 关闭优先级
type ShutdownPriority int

const (
	PriorityHighest ShutdownPriority = iota // 最高优先级（最先执行）
	PriorityHigh
	PriorityNormal
	PriorityLow
	PriorityLowest // 最低优先级（最后执行）
)

// ShutdownHandler 优雅关闭处理器
type ShutdownHandler struct {
	handlers []shutdownEntry
	mu       sync.RWMutex
	timeout  time.Duration
	executed bool
}

type shutdownEntry struct {
	handler  func()
	priority ShutdownPriority
	name     string
}

// NewShutdownHandler 创建新的关闭处理器
func NewShutdownHandler(timeout time.Duration) *ShutdownHandler {
	return &ShutdownHandler{
		handlers: make([]shutdownEntry, 0),
		timeout:  timeout,
	}
}

// Register 注册关闭处理器
func (s *ShutdownHandler) Register(handler func(), priority ShutdownPriority, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executed {
		logger.Warn("Shutdown handler already executed, ignoring new registration")
		return
	}

	s.handlers = append(s.handlers, shutdownEntry{
		handler:  handler,
		priority: priority,
		name:     name,
	})
}

// RegisterWithDefault 使用默认优先级注册
func (s *ShutdownHandler) RegisterWithDefault(handler func(), name string) {
	s.Register(handler, PriorityNormal, name)
}

// Execute 执行所有关闭处理器
func (s *ShutdownHandler) Execute() {
	s.mu.Lock()
	if s.executed {
		s.mu.Unlock()
		return
	}
	s.executed = true
	s.mu.Unlock()

	logger.Info("Executing shutdown handlers")

	// 按优先级排序（优先级高的先执行）
	s.sortHandlers()

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	// 执行所有处理器
	done := make(chan struct{})
	go func() {
		s.executeAll()
		close(done)
	}()

	// 等待完成或超时
	select {
	case <-done:
		logger.Info("All shutdown handlers executed successfully")
	case <-ctx.Done():
		logger.Error("Shutdown timeout exceeded")
	}
}

// executeAll 执行所有处理器
func (s *ShutdownHandler) executeAll() {
	s.mu.RLock()
	handlers := s.handlers
	s.mu.RUnlock()

	for _, entry := range handlers {
		start := time.Now()
		logger.Info("Executing shutdown handler",
			logger.String("name", entry.name),
			logger.Int("priority", int(entry.priority)),
		)

		// 执行处理器
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic in shutdown handler",
						logger.String("name", entry.name),
						logger.Any("panic", r),
					)
				}
			}()

			entry.handler()
		}()

		elapsed := time.Since(start)
		logger.Info("Shutdown handler completed",
			logger.String("name", entry.name),
			logger.Duration("elapsed", elapsed),
		)
	}
}

// sortHandlers 按优先级排序处理器
func (s *ShutdownHandler) sortHandlers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	sort.Slice(s.handlers, func(i, j int) bool {
		return s.handlers[i].priority < s.handlers[j].priority
	})
}

// Clear 清除所有处理器
func (s *ShutdownHandler) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers = make([]shutdownEntry, 0)
	s.executed = false
}

// IsExecuted 检查是否已执行
func (s *ShutdownHandler) IsExecuted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.executed
}

// GetHandlerCount 获取处理器数量
func (s *ShutdownHandler) GetHandlerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.handlers)
}
