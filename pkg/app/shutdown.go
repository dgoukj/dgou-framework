package app

import (
	"context"
	"dgou/pkg/logger"
	"go.uber.org/zap"
	"sort"
	"sync"
	"time"
)

type ShutdownPriority int

const (
	PriorityHighest ShutdownPriority = iota
	PriorityHigh
	PriorityNormal
	PriorityLow
	PriorityLowest
)

type ShutdownHandler struct {
	log      *logger.Logger
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

func NewShutdownHandler(log *logger.Logger, timeout time.Duration) *ShutdownHandler {
	return &ShutdownHandler{
		log:     log,
		timeout: timeout,
	}
}

func (s *ShutdownHandler) Register(handler func(), priority ShutdownPriority, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executed {
		s.log.Warn("Shutdown handler already executed, ignoring new registration", zap.String("name", name))
		return
	}
	s.handlers = append(s.handlers, shutdownEntry{
		handler:  handler,
		priority: priority,
		name:     name,
	})
}

func (s *ShutdownHandler) RegisterWithDefault(handler func(), name string) {
	s.Register(handler, PriorityNormal, name)
}

func (s *ShutdownHandler) Execute() {
	s.mu.Lock()
	if s.executed {
		s.mu.Unlock()
		return
	}
	s.executed = true
	s.mu.Unlock()

	s.log.Info("Executing shutdown handlers")

	// 按优先级排序
	s.sortHandlers()

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.executeAll()
		close(done)
	}()

	select {
	case <-done:
		s.log.Info("All shutdown handlers executed successfully")
	case <-ctx.Done():
		s.log.Error("Shutdown timeout exceeded")
	}
}

func (s *ShutdownHandler) executeAll() {
	s.mu.RLock()
	handlers := s.handlers
	s.mu.RUnlock()

	for _, entry := range handlers {
		start := time.Now()
		s.log.Info("Executing shutdown handler", zap.String("name", entry.name), zap.Int("priority", int(entry.priority)))
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.log.Error("Panic in shutdown handler", zap.String("name", entry.name), zap.Any("panic", r))
				}
			}()
			entry.handler()
		}()
		s.log.Info("Shutdown handler completed", zap.String("name", entry.name), zap.Duration("elapsed", time.Since(start)))
	}
}

func (s *ShutdownHandler) sortHandlers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	sort.Slice(s.handlers, func(i, j int) bool {
		return s.handlers[i].priority < s.handlers[j].priority
	})
}
