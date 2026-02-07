// file: pkg/queue/delayed_queue.go
package queue

import (
	"context"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DelayedQueueConfig 延迟队列配置
type DelayedQueueConfig struct {
	Enabled     bool          `mapstructure:"enabled"`      // 是否启用延迟队列
	Exchange    string        `mapstructure:"exchange"`     // 延迟交换机
	QueuePrefix string        `mapstructure:"queue_prefix"` // 队列前缀
	MaxDelay    time.Duration `mapstructure:"max_delay"`    // 最大延迟时间
	AutoCreate  bool          `mapstructure:"auto_create"`  // 是否自动创建
}

// DelayedMessage 延迟消息
type DelayedMessage struct {
	Message    *Message      `json:"message"`     // 原始消息
	Delay      time.Duration `json:"delay"`       // 延迟时间
	TargetTime time.Time     `json:"target_time"` // 目标时间
	CreatedAt  time.Time     `json:"created_at"`  // 创建时间
}

// DelayedQueueManager 延迟队列管理器
type DelayedQueueManager struct {
	connection *Connection         // 连接管理器
	config     *DelayedQueueConfig // 配置
	producer   *Producer           // 生产者
	queues     map[string]string   // 延迟队列映射
	mu         sync.RWMutex        // 读写锁
}

// NewDelayedQueueConfig 创建延迟队列配置
func NewDelayedQueueConfig() *DelayedQueueConfig {
	return &DelayedQueueConfig{
		Enabled:     true,
		Exchange:    "delayed.exchange",
		QueuePrefix: "delayed.",
		MaxDelay:    30 * 24 * time.Hour, // 30天
		AutoCreate:  true,
	}
}

// NewDelayedQueueManager 创建延迟队列管理器
func NewDelayedQueueManager(connection *Connection, config *DelayedQueueConfig) (*DelayedQueueManager, error) {
	if config == nil {
		config = NewDelayedQueueConfig()
	}

	manager := &DelayedQueueManager{
		connection: connection,
		config:     config,
		queues:     make(map[string]string),
	}

	// 创建生产者
	manager.producer = NewProducer(connection, config.Exchange, "")

	// 自动创建延迟交换机
	if config.AutoCreate {
		if err := manager.createDelayedExchange(); err != nil {
			return nil, err
		}
	}

	logger.Info("Delayed queue manager initialized",
		logger.String("exchange", config.Exchange),
		logger.Duration("max_delay", config.MaxDelay),
	)

	return manager, nil
}

// createDelayedExchange 创建延迟交换机
func (m *DelayedQueueManager) createDelayedExchange() error {
	channel, err := m.connection.GetChannel()
	if err != nil {
		return err
	}

	// 创建延迟交换机（使用插件支持的x-delayed-message类型）
	args := amqp.Table{
		"x-delayed-type": "direct",
	}

	err = channel.ExchangeDeclare(
		m.config.Exchange,
		"x-delayed-message", // RabbitMQ延迟消息插件类型
		true,                // durable
		false,               // autoDelete
		false,               // internal
		false,               // noWait
		args,
	)

	if err != nil {
		// 如果失败，可能是插件未安装，回退到普通交换机
		logger.Warn("x-delayed-message exchange type not supported, using direct exchange",
			logger.String("exchange", m.config.Exchange),
			logger.ErrorField(err),
		)

		err = channel.ExchangeDeclare(
			m.config.Exchange,
			"direct",
			true,
			false,
			false,
			false,
			nil,
		)

		if err != nil {
			return err
		}
	}

	logger.Info("Delayed exchange created",
		logger.String("exchange", m.config.Exchange),
	)

	return nil
}

// SendDelayed 发送延迟消息
func (m *DelayedQueueManager) SendDelayed(ctx context.Context, msg *Message, delay time.Duration) error {
	if !m.config.Enabled {
		return errors.New(errors.CodeValidationFailed, "Delayed queue is disabled")
	}

	// 检查延迟时间
	if delay <= 0 {
		return errors.New(errors.CodeValidationFailed, "Delay must be greater than 0")
	}

	if delay > m.config.MaxDelay {
		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Delay exceeds maximum allowed: %s", m.config.MaxDelay))
	}

	// 创建延迟队列（如果需要）
	queueName, err := m.getOrCreateDelayedQueue(delay)
	if err != nil {
		return err
	}

	// 设置消息头中的延迟信息
	if msg.Headers == nil {
		msg.Headers = make(map[string]interface{})
	}
	msg.Headers["x-delay"] = int(delay.Milliseconds())
	msg.Headers["x-delayed-target-time"] = time.Now().Add(delay).Format(time.RFC3339)

	// 设置路由键为队列名
	m.producer.SetRoutingKey(queueName)

	// 发送消息
	err = m.producer.Publish(ctx, msg)
	if err != nil {
		return err
	}

	logger.Info("Delayed message scheduled",
		logger.String("message_id", msg.ID),
		logger.Duration("delay", delay),
		logger.String("target_time", msg.Headers["x-delayed-target-time"].(string)),
		logger.String("queue", queueName),
	)

	return nil
}

// getOrCreateDelayedQueue 获取或创建延迟队列
func (m *DelayedQueueManager) getOrCreateDelayedQueue(delay time.Duration) (string, error) {
	// 根据延迟时间分组创建队列
	delayKey := m.getDelayKey(delay)

	m.mu.RLock()
	queueName, exists := m.queues[delayKey]
	m.mu.RUnlock()

	if exists {
		return queueName, nil
	}

	// 创建新的延迟队列
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if queueName, exists = m.queues[delayKey]; exists {
		return queueName, nil
	}

	// 生成队列名
	queueName = fmt.Sprintf("%s%s", m.config.QueuePrefix, delayKey)

	// 创建队列
	channel, err := m.connection.GetChannel()
	if err != nil {
		return "", err
	}

	// 设置队列TTL和死信
	args := amqp.Table{
		"x-message-ttl":             int32(delay.Milliseconds()),
		"x-dead-letter-exchange":    m.config.Exchange,
		"x-dead-letter-routing-key": queueName,
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
		return "", err
	}

	// 绑定队列到交换机
	err = channel.QueueBind(
		queueName,
		queueName,
		m.config.Exchange,
		false,
		nil,
	)

	if err != nil {
		return "", err
	}

	// 缓存队列名
	m.queues[delayKey] = queueName

	logger.Debug("Delayed queue created",
		logger.String("queue", queueName),
		logger.Duration("delay", delay),
		logger.String("delay_key", delayKey),
	)

	return queueName, nil
}

// getDelayKey 获取延迟键
func (m *DelayedQueueManager) getDelayKey(delay time.Duration) string {
	// 将延迟时间转换为分钟级别的键，减少队列数量
	minutes := int(delay.Minutes())
	if minutes < 1 {
		return "immediate"
	} else if minutes < 60 {
		return fmt.Sprintf("minutes_%d", minutes)
	} else if minutes < 1440 { // 24小时
		hours := minutes / 60
		return fmt.Sprintf("hours_%d", hours)
	} else {
		days := minutes / 1440
		return fmt.Sprintf("days_%d", days)
	}
}

// ConsumeDelayed 消费延迟队列
func (m *DelayedQueueManager) ConsumeDelayed(queueName string, handler MessageHandler) (*Consumer, error) {
	if !m.config.Enabled {
		return nil, errors.New(errors.CodeValidationFailed, "Delayed queue is disabled")
	}

	// 创建消费者
	options := &ConsumerOptions{
		ConsumerTag: fmt.Sprintf("delayed-consumer-%s", queueName),
		AutoAck:     false,
	}

	consumer := NewConsumer(m.connection, queueName, handler, options)

	// 开始消费
	ctx := context.Background()
	err := consumer.Start(ctx)
	if err != nil {
		return nil, err
	}

	logger.Info("Delayed queue consumer started",
		logger.String("queue", queueName),
	)

	return consumer, nil
}

// GetDelayedQueueStats 获取延迟队列统计信息
func (m *DelayedQueueManager) GetDelayedQueueStats() (map[string]interface{}, error) {
	if !m.config.Enabled {
		return map[string]interface{}{
			"enabled": false,
		}, nil
	}

	stats := map[string]interface{}{
		"enabled":   true,
		"exchange":  m.config.Exchange,
		"max_delay": m.config.MaxDelay.String(),
		"queues":    make(map[string]interface{}),
	}

	// 获取所有延迟队列的信息
	channel, err := m.connection.GetChannel()
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for delayKey, queueName := range m.queues {
		queue, err := channel.QueueDeclarePassive(
			queueName,
			true,
			false,
			false,
			false,
			nil,
		)

		if err == nil {
			stats["queues"].(map[string]interface{})[delayKey] = map[string]interface{}{
				"queue_name":     queueName,
				"message_count":  queue.Messages,
				"consumer_count": queue.Consumers,
			}
		}
	}

	return stats, nil
}
