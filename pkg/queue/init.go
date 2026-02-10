// file: pkg/queue/init.go
package queue

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/logger"
	"sync"
	"time"
)

var (
	// defaultManager 默认队列管理器实例
	defaultManager *QueueManager
	// managerMutex 用于保护默认管理器实例
	managerMutex sync.RWMutex
	// isInitialized 标记队列组件是否已初始化
	isInitialized bool
	// initMutex 用于保护初始化过程
	initMutex sync.Mutex
)

// Init 初始化队列组件（使用默认配置）
func Init() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if isInitialized {
		logger.Info("Queue component already initialized")
		return nil
	}

	// 获取应用配置
	cfg := config.GetConfig()
	if cfg == nil {
		logger.Warn("Config not found, using default configuration")
		cfg = &config.Config{
			RabbitMQ: config.RabbitMQConfig{
				URL:            "amqp://guest:guest@localhost:5672/",
				Heartbeat:      60,
				ConnectionName: "default-queue",
				MaxReconnect:   10,
				ReconnectDelay: 5,
				PrefetchCount:  10,
			},
			Queue: config.QueueConfig{
				EnableDeadLetter:   true,
				EnableDelayedQueue: true,
				EnableRetry:        true,
				MaxRetries:         3,
			},
		}
	}

	return InitWithConfig(cfg)
}

// InitWithConfig 使用自定义配置初始化队列组件
func InitWithConfig(cfg *config.Config) error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if isInitialized {
		logger.Info("Queue component already initialized, reinitializing")
	}

	// 创建队列管理器
	manager, err := InitQueueManager(cfg)
	if err != nil {
		logger.Error("Failed to initialize queue manager",
			logger.ErrorField(err),
		)
		return err
	}

	// 设置默认管理器
	SetDefaultManager(manager)
	isInitialized = true

	logger.Info("Queue component initialized successfully")
	return nil
}

// SetDefaultManager 设置默认队列管理器
func SetDefaultManager(manager *QueueManager) {
	managerMutex.Lock()
	defer managerMutex.Unlock()
	defaultManager = manager
}

// GetDefaultManager 获取默认队列管理器
func GetDefaultManager() *QueueManager {
	managerMutex.RLock()
	defer managerMutex.RUnlock()
	return defaultManager
}

// IsInitialized 检查队列组件是否已初始化
func IsInitialized() bool {
	managerMutex.RLock()
	defer managerMutex.RUnlock()
	return defaultManager != nil && isInitialized
}

// GetProducer 获取默认生产者
func GetProducer(exchange, routingKey string) (*Producer, error) {
	manager := GetDefaultManager()
	if manager == nil {
		return nil, NewError("Queue manager not initialized, please call queue.Init() first")
	}
	return manager.GetProducer(exchange, routingKey), nil
}

// CreateConsumer 创建默认消费者
func CreateConsumer(queueName string, handler MessageHandler, options *ConsumerOptions) (*Consumer, error) {
	manager := GetDefaultManager()
	if manager == nil {
		return nil, NewError("Queue manager not initialized, please call queue.Init() first")
	}
	return manager.CreateConsumer(queueName, handler, options)
}

// StartConsumer 启动默认消费者
func StartConsumer(queueName string, handler MessageHandler, options *ConsumerOptions) error {
	manager := GetDefaultManager()
	if manager == nil {
		return NewError("Queue manager not initialized, please call queue.Init() first")
	}
	return manager.StartConsumer(queueName, handler, options)
}

// Stop 停止队列组件
func Stop() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if !isInitialized {
		logger.Info("Queue component not initialized")
		return nil
	}

	manager := GetDefaultManager()
	if manager == nil {
		isInitialized = false
		return nil
	}

	err := manager.Stop()
	if err != nil {
		logger.Error("Failed to stop queue manager",
			logger.ErrorField(err),
		)
		return err
	}

	// 清理状态
	managerMutex.Lock()
	defaultManager = nil
	isInitialized = false
	managerMutex.Unlock()

	logger.Info("Queue component stopped successfully")
	return nil
}

// GetMetrics 获取队列组件指标
func GetMetrics() (map[string]interface{}, error) {
	manager := GetDefaultManager()
	if manager == nil {
		return nil, NewError("Queue manager not initialized")
	}
	return manager.GetMetrics(), nil
}

// DeclareExchange 声明默认交换机
func DeclareExchange(config *ExchangeConfig) error {
	manager := GetDefaultManager()
	if manager == nil {
		return NewError("Queue manager not initialized")
	}
	return manager.DeclareExchange(config)
}

// DeclareQueue 声明默认队列
func DeclareQueue(config *QueueConfig) error {
	manager := GetDefaultManager()
	if manager == nil {
		return NewError("Queue manager not initialized")
	}
	return manager.DeclareQueue(config)
}

// Publish 发布消息（使用默认管理器）
func Publish(ctx context.Context, exchange, routingKey string, msg *Message) error {
	producer, err := GetProducer(exchange, routingKey)
	if err != nil {
		return err
	}
	return producer.Publish(ctx, msg)
}

// PublishWithRetry 发布消息（带重试，使用默认管理器）
func PublishWithRetry(ctx context.Context, exchange, routingKey string, msg *Message, maxRetries int, retryDelay time.Duration) error {
	producer, err := GetProducer(exchange, routingKey)
	if err != nil {
		return err
	}
	return producer.PublishWithRetry(ctx, msg, maxRetries, retryDelay)
}

// NewError 创建队列组件错误
func NewError(message string) error {
	return &QueueError{
		Message: message,
	}
}

// QueueError 队列组件错误
type QueueError struct {
	Message string
}

// Error 实现error接口
func (e *QueueError) Error() string {
	return e.Message
}

// 初始化检查
func init() {
	// 注册配置验证器
	config.RegisterValidator(func(cfg *config.Config) error {
		if cfg.RabbitMQ.URL == "" && cfg.RabbitMQ.Host == "" {
			logger.Warn("RabbitMQ configuration not found, queue component will not work properly")
		}
		return nil
	})

	// 注册优雅关闭处理器
	config.RegisterShutdownHandler(func() error {
		return Stop()
	})
}
