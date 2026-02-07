// file: pkg/queue/manager.go
package queue

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"github.com/streadway/amqp"
	"sync"
	"time"
)

// QueueManager 队列管理器
type QueueManager struct {
	config          *RabbitMQConfig      // 配置
	connection      *Connection          // 连接管理器
	producers       map[string]*Producer // 生产者映射
	consumers       map[string]*Consumer // 消费者映射
	deadLetterMgr   *DeadLetterManager   // 死信队列管理器
	delayedQueueMgr *DelayedQueueManager // 延迟队列管理器
	retryManager    *RetryManager        // 重试管理器
	mu              sync.RWMutex         // 读写锁
}

// QueueManagerOptions 队列管理器选项
type QueueManagerOptions struct {
	EnableDeadLetter   bool                `json:"enable_dead_letter"`
	EnableDelayedQueue bool                `json:"enable_delayed_queue"`
	DeadLetterConfig   *DeadLetterConfig   `json:"dead_letter_config"`
	DelayedQueueConfig *DelayedQueueConfig `json:"delayed_queue_config"`
	RetryConfig        *RetryConfig        `json:"retry_config"`
}

// NewQueueManager 创建队列管理器
func NewQueueManager(rabbitConfig *RabbitMQConfig, options *QueueManagerOptions) (*QueueManager, error) {
	if rabbitConfig == nil {
		return nil, errors.New(errors.CodeValidationFailed, "RabbitMQ config is required")
	}

	if options == nil {
		options = &QueueManagerOptions{
			EnableDeadLetter:   true,
			EnableDelayedQueue: true,
		}
	}

	// 创建连接管理器
	connection := NewConnection(rabbitConfig)

	// 连接RabbitMQ
	if err := connection.Connect(); err != nil {
		return nil, err
	}

	// 创建队列管理器
	manager := &QueueManager{
		config:     rabbitConfig,
		connection: connection,
		producers:  make(map[string]*Producer),
		consumers:  make(map[string]*Consumer),
	}

	// 初始化死信队列管理器
	if options.EnableDeadLetter {
		deadLetterMgr, err := NewDeadLetterManager(connection, options.DeadLetterConfig)
		if err != nil {
			logger.Warn("Failed to initialize dead letter manager",
				logger.ErrorField(err),
			)
		} else {
			manager.deadLetterMgr = deadLetterMgr
		}
	}

	// 初始化延迟队列管理器
	if options.EnableDelayedQueue {
		delayedQueueMgr, err := NewDelayedQueueManager(connection, options.DelayedQueueConfig)
		if err != nil {
			logger.Warn("Failed to initialize delayed queue manager",
				logger.ErrorField(err),
			)
		} else {
			manager.delayedQueueMgr = delayedQueueMgr
		}
	}

	// 初始化重试管理器
	manager.retryManager = NewRetryManager(options.RetryConfig)

	logger.Info("Queue manager initialized successfully",
		logger.Bool("dead_letter_enabled", manager.deadLetterMgr != nil),
		logger.Bool("delayed_queue_enabled", manager.delayedQueueMgr != nil),
	)

	return manager, nil
}

// GetProducer 获取或创建生产者
func (m *QueueManager) GetProducer(exchange, routingKey string) *Producer {
	key := fmt.Sprintf("%s:%s", exchange, routingKey)

	m.mu.RLock()
	producer, exists := m.producers[key]
	m.mu.RUnlock()

	if exists {
		return producer
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if producer, exists = m.producers[key]; exists {
		return producer
	}

	// 创建新的生产者
	producer = NewProducer(m.connection, exchange, routingKey)
	m.producers[key] = producer

	logger.Debug("Producer created",
		logger.String("exchange", exchange),
		logger.String("routing_key", routingKey),
	)

	return producer
}

// CreateConsumer 创建消费者
func (m *QueueManager) CreateConsumer(queueName string, handler MessageHandler, options *ConsumerOptions) (*Consumer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在消费者
	if consumer, exists := m.consumers[queueName]; exists && consumer.IsConsuming() {
		return nil, errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Consumer for queue %s is already running", queueName))
	}

	// 创建新的消费者
	consumer := NewConsumer(m.connection, queueName, handler, options)
	m.consumers[queueName] = consumer

	logger.Debug("Consumer created",
		logger.String("queue", queueName),
	)

	return consumer, nil
}

// StartConsumer 启动消费者
func (m *QueueManager) StartConsumer(queueName string, handler MessageHandler, options *ConsumerOptions) error {
	consumer, err := m.CreateConsumer(queueName, handler, options)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return consumer.Start(ctx)
}

// StopConsumer 停止消费者
func (m *QueueManager) StopConsumer(queueName string) error {
	m.mu.Lock()
	consumer, exists := m.consumers[queueName]
	m.mu.Unlock()

	if !exists {
		return nil
	}

	if err := consumer.Stop(); err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.consumers, queueName)
	m.mu.Unlock()

	return nil
}

// GetDeadLetterManager 获取死信队列管理器
func (m *QueueManager) GetDeadLetterManager() *DeadLetterManager {
	return m.deadLetterMgr
}

// GetDelayedQueueManager 获取延迟队列管理器
func (m *QueueManager) GetDelayedQueueManager() *DelayedQueueManager {
	return m.delayedQueueMgr
}

// GetRetryManager 获取重试管理器
func (m *QueueManager) GetRetryManager() *RetryManager {
	return m.retryManager
}

// DeclareExchange 声明交换机
func (m *QueueManager) DeclareExchange(config *ExchangeConfig) error {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return err
	}

	return channel.ExchangeDeclare(
		config.Name,
		string(config.Type),
		config.Durable,
		config.AutoDelete,
		config.Internal,
		config.NoWait,
		config.Args,
	)
}

// DeclareQueue 声明队列
func (m *QueueManager) DeclareQueue(config *QueueConfig) error {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return err
	}

	_, err = channel.QueueDeclare(
		config.Name,
		config.Durable,
		config.AutoDelete,
		config.Exclusive,
		config.NoWait,
		config.Args,
	)

	if err != nil {
		return err
	}

	// 绑定队列到默认交换机
	if config.BindingKey != "" {
		return channel.QueueBind(
			config.Name,
			config.BindingKey,
			"", // 默认交换机
			config.NoWait,
			nil,
		)
	}

	return nil
}

// BindQueue 绑定队列到交换机
func (m *QueueManager) BindQueue(queueName, exchange, routingKey string, args amqp.Table) error {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return err
	}

	return channel.QueueBind(
		queueName,
		routingKey,
		exchange,
		false, // noWait
		args,
	)
}

// PurgeQueue 清空队列
func (m *QueueManager) PurgeQueue(queueName string) (int, error) {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return 0, err
	}

	return channel.QueuePurge(queueName, false)
}

// DeleteQueue 删除队列
func (m *QueueManager) DeleteQueue(queueName string, ifUnused, ifEmpty bool) (int, error) {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return 0, err
	}

	return channel.QueueDelete(queueName, ifUnused, ifEmpty, false)
}

// GetQueueStats 获取队列统计信息
func (m *QueueManager) GetQueueStats(queueName string) (map[string]interface{}, error) {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return nil, err
	}

	queue, err := channel.QueueDeclarePassive(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":        queue.Name,
		"messages":    queue.Messages,
		"consumers":   queue.Consumers,
		"durable":     true,
		"auto_delete": false,
		"exclusive":   false,
	}, nil
}

// GetMetrics 获取管理器指标
func (m *QueueManager) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := map[string]interface{}{
		"connection":      m.connection.GetMetrics(),
		"producers_count": len(m.producers),
		"consumers_count": len(m.consumers),
		"consumers":       make(map[string]interface{}),
	}

	// 收集消费者指标
	for queueName, consumer := range m.consumers {
		metrics["consumers"].(map[string]interface{})[queueName] = consumer.GetMetrics()
	}

	// 收集生产者指标
	producersMetrics := make(map[string]interface{})
	for key, producer := range m.producers {
		producersMetrics[key] = producer.GetMetrics()
	}
	metrics["producers"] = producersMetrics

	// 收集死信队列指标
	if m.deadLetterMgr != nil {
		if stats, err := m.deadLetterMgr.GetDeadLetterStats(); err == nil {
			metrics["dead_letter"] = stats
		}
	}

	// 收集延迟队列指标
	if m.delayedQueueMgr != nil {
		if stats, err := m.delayedQueueMgr.GetDelayedQueueStats(); err == nil {
			metrics["delayed_queue"] = stats
		}
	}

	return metrics
}

// Stop 停止队列管理器
func (m *QueueManager) Stop() error {
	logger.Info("Stopping queue manager...")

	// 停止所有消费者
	m.mu.Lock()
	for queueName, consumer := range m.consumers {
		if err := consumer.Stop(); err != nil {
			logger.Warn("Failed to stop consumer",
				logger.String("queue", queueName),
				logger.ErrorField(err),
			)
		}
	}
	m.consumers = make(map[string]*Consumer)
	m.mu.Unlock()

	// 停止死信队列管理器
	if m.deadLetterMgr != nil {
		m.deadLetterMgr.Stop()
	}

	// 停止连接管理器
	if err := m.connection.Stop(); err != nil {
		return err
	}

	logger.Info("Queue manager stopped successfully")
	return nil
}

// InitQueueManager 初始化队列管理器
func InitQueueManager(cfg *config.Config) (*QueueManager, error) {
	rabbitConfig := NewRabbitMQConfig(cfg)

	options := &QueueManagerOptions{
		EnableDeadLetter:   cfg.Queue.EnableDeadLetter,
		EnableDelayedQueue: cfg.Queue.EnableDelayedQueue,
		DeadLetterConfig: &DeadLetterConfig{
			Enabled:    cfg.Queue.EnableDeadLetter,
			Exchange:   cfg.Queue.DeadLetterExchange,
			Queue:      cfg.Queue.DeadLetterQueue,
			RoutingKey: cfg.Queue.DeadLetterRoutingKey,
			TTL:        cfg.Queue.DeadLetterTTL,
			MaxLength:  cfg.Queue.DeadLetterMaxLength,
			AutoCreate: true,
		},
		DelayedQueueConfig: &DelayedQueueConfig{
			Enabled:     cfg.Queue.EnableDelayedQueue,
			Exchange:    cfg.Queue.DelayedExchange,
			QueuePrefix: cfg.Queue.DelayedQueuePrefix,
			MaxDelay:    cfg.Queue.DelayedMaxDelay,
			AutoCreate:  true,
		},
		RetryConfig: &RetryConfig{
			Enabled:       cfg.Queue.EnableRetry,
			MaxRetries:    cfg.Queue.MaxRetries,
			InitialDelay:  cfg.Queue.RetryInitialDelay,
			MaxDelay:      cfg.Queue.RetryMaxDelay,
			Strategy:      RetryStrategy(cfg.Queue.RetryStrategy),
			BackoffFactor: cfg.Queue.RetryBackoffFactor,
			Jitter:        cfg.Queue.RetryJitter,
			JitterFactor:  cfg.Queue.RetryJitterFactor,
		},
	}

	return NewQueueManager(rabbitConfig, options)
}
