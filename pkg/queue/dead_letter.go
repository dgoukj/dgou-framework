// file: pkg/queue/dead_letter.go
package queue

import (
	"context"
	"dgou/pkg/logger"
	"fmt"
	"os"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DeadLetterConfig 死信队列配置
type DeadLetterConfig struct {
	Enabled     bool          `mapstructure:"enabled"`      // 是否启用死信队列
	Exchange    string        `mapstructure:"exchange"`     // 死信交换机
	Queue       string        `mapstructure:"queue"`        // 死信队列
	RoutingKey  string        `mapstructure:"routing_key"`  // 死信路由键
	TTL         time.Duration `mapstructure:"ttl"`          // 消息TTL
	MaxLength   int           `mapstructure:"max_length"`   // 最大队列长度
	MaxBytes    int           `mapstructure:"max_bytes"`    // 最大队列字节数
	MaxPriority uint8         `mapstructure:"max_priority"` // 最大优先级
	AutoCreate  bool          `mapstructure:"auto_create"`  // 是否自动创建
}

// DeadLetterManager 死信队列管理器
type DeadLetterManager struct {
	connection  *Connection       // 连接管理器
	config      *DeadLetterConfig // 配置
	producer    *Producer         // 生产者
	consumer    *Consumer         // 消费者
	dlxExchange string            // 死信交换机名称
	dlqQueue    string            // 死信队列名称
}

// NewDeadLetterConfig 创建死信队列配置
func NewDeadLetterConfig() *DeadLetterConfig {
	return &DeadLetterConfig{
		Enabled:    true,
		Exchange:   "dlx.exchange",
		Queue:      "dlx.queue",
		RoutingKey: "#",
		TTL:        7 * 24 * time.Hour, // 7天
		MaxLength:  10000,
		AutoCreate: true,
	}
}

// NewDeadLetterManager 创建死信队列管理器
func NewDeadLetterManager(connection *Connection, config *DeadLetterConfig) (*DeadLetterManager, error) {
	if config == nil {
		config = NewDeadLetterConfig()
	}

	manager := &DeadLetterManager{
		connection:  connection,
		config:      config,
		dlxExchange: fmt.Sprintf("%s.dlx", config.Exchange),
		dlqQueue:    fmt.Sprintf("%s.dlq", config.Queue),
	}

	// 自动创建死信交换机和队列
	if config.AutoCreate {
		if err := manager.createDeadLetterResources(); err != nil {
			return nil, err
		}
	}

	// 创建生产者
	manager.producer = NewProducer(connection, manager.dlxExchange, config.RoutingKey)

	logger.Info("Dead letter manager initialized",
		logger.String("dlx_exchange", manager.dlxExchange),
		logger.String("dlq_queue", manager.dlqQueue),
	)

	return manager, nil
}

// createDeadLetterResources 创建死信资源
func (m *DeadLetterManager) createDeadLetterResources() error {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return err
	}

	// 创建死信交换机
	err = channel.ExchangeDeclare(
		m.dlxExchange,
		"topic", // 死信交换机通常使用topic类型
		true,    // durable
		false,   // autoDelete
		false,   // internal
		false,   // noWait
		nil,     // args
	)

	if err != nil {
		return err
	}

	// 创建死信队列
	args := amqp.Table{
		"x-message-ttl":          int32(m.config.TTL.Milliseconds()),
		"x-max-length":           int32(m.config.MaxLength),
		"x-max-length-bytes":     int32(m.config.MaxBytes),
		"x-max-priority":         m.config.MaxPriority,
		"x-dead-letter-exchange": "", // 死信队列的死信交换机为空
	}

	_, err = channel.QueueDeclare(
		m.dlqQueue,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		args,
	)

	if err != nil {
		return err
	}

	// 绑定死信队列到死信交换机
	err = channel.QueueBind(
		m.dlqQueue,
		m.config.RoutingKey,
		m.dlxExchange,
		false, // noWait
		nil,   // args
	)

	if err != nil {
		return err
	}

	logger.Info("Dead letter resources created",
		logger.String("exchange", m.dlxExchange),
		logger.String("queue", m.dlqQueue),
	)

	return nil
}

// SendToDeadLetter 发送消息到死信队列
func (m *DeadLetterManager) SendToDeadLetter(msg *Message, reason string) error {
	if !m.config.Enabled {
		logger.Warn("Dead letter queue is disabled, message will be discarded",
			logger.String("message_id", msg.ID),
			logger.String("reason", reason),
		)
		return nil
	}

	// 添加死信原因到消息头
	if msg.Headers == nil {
		msg.Headers = make(map[string]interface{})
	}
	msg.Headers["x-dead-letter-reason"] = reason
	msg.Headers["x-dead-letter-timestamp"] = time.Now().Format(time.RFC3339)

	// 设置死信路由键
	dlRoutingKey := fmt.Sprintf("%s.dlx", msg.Exchange)
	if msg.Exchange == "" {
		dlRoutingKey = "default.dlx"
	}

	// 发送到死信队列
	ctx := context.Background()
	err := m.producer.Publish(ctx, msg)

	if err != nil {
		logger.Error("Failed to send message to dead letter queue",
			logger.String("message_id", msg.ID),
			logger.String("reason", reason),
			logger.ErrorField(err),
		)
		return err
	}

	logger.Warn("Message sent to dead letter queue",
		logger.String("message_id", msg.ID),
		logger.String("reason", reason),
		logger.String("dlx_exchange", m.dlxExchange),
		logger.String("dl_routing_key", dlRoutingKey),
	)

	return nil
}

// ProcessDeadLetter 处理死信队列中的消息
func (m *DeadLetterManager) ProcessDeadLetter(handler MessageHandler) error {
	if !m.config.Enabled {
		return fmt.Errorf("dead letter queue is disabled")
	}

	// 创建消费者
	options := &ConsumerOptions{
		ConsumerTag: fmt.Sprintf("dlq-consumer-%s", m.dlqQueue),
		AutoAck:     false,
	}

	m.consumer = NewConsumer(m.connection, m.dlqQueue, handler, options)

	// 开始消费死信队列
	ctx := context.Background()
	err := m.consumer.Start(ctx)
	if err != nil {
		return err
	}

	logger.Info("Dead letter queue processor started",
		logger.String("queue", m.dlqQueue),
	)

	return nil
}

// GetDeadLetterStats 获取死信队列统计信息
func (m *DeadLetterManager) GetDeadLetterStats() (map[string]interface{}, error) {
	if !m.config.Enabled {
		return map[string]interface{}{
			"enabled": false,
		}, nil
	}

	channel, err := m.connection.GetChannel()
	if err != nil {
		return nil, err
	}

	// 获取队列信息
	queue, err := channel.QueueDeclarePassive(
		m.dlqQueue,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"enabled":        true,
		"exchange":       m.dlxExchange,
		"queue":          m.dlqQueue,
		"message_count":  queue.Messages,
		"consumer_count": queue.Consumers,
		"ttl":            m.config.TTL.String(),
		"max_length":     m.config.MaxLength,
		"max_bytes":      m.config.MaxBytes,
		"max_priority":   m.config.MaxPriority,
	}, nil
}

// Stop 停止死信队列管理器
func (m *DeadLetterManager) Stop() error {
	if m.consumer != nil {
		m.consumer.Stop()
	}

	logger.Info("Dead letter manager stopped")
	return nil
}

// SetupDeadLetterForQueue 为队列设置死信
func (m *DeadLetterManager) SetupDeadLetterForQueue(queueName string, maxRetries int) error {
	if !m.config.Enabled {
		return nil
	}

	channel, err := m.connection.GetChannel()
	if err != nil {
		return err
	}

	// 尝试声明队列（如果队列不存在，会创建新队列）
	// 如果队列已存在，声明时会检查参数是否匹配，不匹配会报错
	args := amqp.Table{
		"x-dead-letter-exchange":    m.dlxExchange,
		"x-dead-letter-routing-key": fmt.Sprintf("%s.dlx", queueName),
		"x-max-retries":             int32(maxRetries),
	}

	_, err = channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		args,
	)

	if err != nil {
		// 如果队列已存在且参数不匹配，尝试获取队列信息
		queue, inspectErr := channel.QueueInspect(queueName)
		if inspectErr != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
		}

		// 队列已存在，记录警告
		logger.Warn("Queue already exists with different parameters",
			logger.String("queue", queueName),
			logger.Int("messages", queue.Messages),
			logger.Int("consumers", queue.Consumers),
		)

		// 无法修改已存在队列的参数，RabbitMQ不支持此操作
		// 建议使用策略或重新创建队列
		return fmt.Errorf("queue %s already exists with different parameters. "+
			"Consider using RabbitMQ policies or recreating the queue", queueName)
	}

	logger.Info("Dead letter configured for queue",
		logger.String("queue", queueName),
		logger.String("dlx_exchange", m.dlxExchange),
		logger.Int("max_retries", maxRetries),
	)

	return nil
}

// deleteAndRecreateQueue 删除并重新创建队列
func (m *DeadLetterManager) deleteAndRecreateQueue(channel *amqp.Channel, queueName string, args amqp.Table) error {
	// 获取队列信息
	queue, err := channel.QueueInspect(queueName)
	if err != nil {
		return fmt.Errorf("failed to inspect queue %s: %w", queueName, err)
	}

	// 记录警告
	logger.Warn("Deleting and recreating queue to apply dead letter configuration",
		logger.String("queue", queueName),
		logger.Int("message_count", queue.Messages),
		logger.Int("consumer_count", queue.Consumers),
	)

	// 检查队列是否为空且无消费者
	if queue.Messages > 0 || queue.Consumers > 0 {
		return fmt.Errorf("cannot delete queue %s: it has %d messages and %d consumers",
			queueName, queue.Messages, queue.Consumers)
	}

	// 删除队列
	_, err = channel.QueueDelete(
		queueName,
		false, // ifUnused
		false, // ifEmpty
		false, // noWait
	)
	if err != nil {
		return fmt.Errorf("failed to delete queue: %w", err)
	}

	// 重新创建队列
	_, err = channel.QueueDeclare(
		queueName,
		true,  // durable (使用默认值)
		false, // autoDelete (使用默认值)
		false, // exclusive (使用默认值)
		false, // noWait
		args,
	)
	if err != nil {
		return fmt.Errorf("failed to recreate queue: %w", err)
	}

	logger.Info("Queue recreated with dead letter configuration",
		logger.String("queue", queueName),
		logger.String("dlx_exchange", m.dlxExchange),
	)

	return nil
}

// createQueueWithDeadLetter 创建带死信配置的队列
func (m *DeadLetterManager) createQueueWithDeadLetter(channel *amqp.Channel, queueName string, maxRetries int) error {
	args := amqp.Table{
		"x-dead-letter-exchange":    m.dlxExchange,
		"x-dead-letter-routing-key": fmt.Sprintf("%s.dlx", queueName),
		"x-max-retries":             int32(maxRetries),
	}

	_, err := channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		args,
	)

	if err != nil {
		return fmt.Errorf("failed to create queue with dead letter: %w", err)
	}

	logger.Info("Queue created with dead letter configuration",
		logger.String("queue", queueName),
		logger.String("dlx_exchange", m.dlxExchange),
		logger.Int("max_retries", maxRetries),
	)

	return nil
}

// applyDeadLetterPolicy 使用RabbitMQ策略设置死信（推荐）
func (m *DeadLetterManager) applyDeadLetterPolicy(queueName string, maxRetries int) error {
	// 注意：RabbitMQ策略需要通过管理API或CLI设置
	// 这里提供一个占位符，实际实现需要调用RabbitMQ管理API

	logger.Info("Please set dead letter policy via RabbitMQ management",
		logger.String("queue", queueName),
		logger.String("policy_name", fmt.Sprintf("dlx-policy-%s", queueName)),
		logger.String("policy_pattern", fmt.Sprintf("^%s$", queueName)),
		logger.String("policy_definition", fmt.Sprintf(`{
            "dead-letter-exchange": "%s",
            "dead-letter-routing-key": "%s.dlx",
            "max-retries": %d
        }`, m.dlxExchange, queueName, maxRetries)),
	)

	return nil
}

// shouldDeleteAndRecreate 判断是否应该删除重建队列
func (m *DeadLetterManager) shouldDeleteAndRecreate(queueName string) bool {
	// 在生产环境中，应该使用策略而不是删除重建
	// 这里可以根据配置决定是否允许删除重建
	if m.config.AutoCreate {
		// 只有在新创建的队列时才允许删除重建
		return false
	}

	// 可以根据环境变量或配置决定
	envValue := os.Getenv("ALLOW_QUEUE_RECREATE")
	return strings.ToLower(envValue) == "true"
}
