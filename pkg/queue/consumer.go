// file: pkg/queue/consumer.go
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

// Consumer 消息消费者
type Consumer struct {
	connection  *Connection      // 连接管理器
	queueName   string           // 队列名称
	consumerTag string           // 消费者标签
	autoAck     bool             // 是否自动确认
	exclusive   bool             // 是否排他
	noLocal     bool             // 是否不接收本地消息
	noWait      bool             // 是否不等待
	args        amqp.Table       // 额外参数
	handler     MessageHandler   // 消息处理器
	mu          sync.RWMutex     // 读写锁
	isConsuming bool             // 是否正在消费
	stopCh      chan struct{}    // 停止通道
	metrics     *ConsumerMetrics // 消费者指标
}

// ConsumerMetrics 消费者指标
type ConsumerMetrics struct {
	MessagesConsumed  int64         `json:"messages_consumed"`
	MessagesAcked     int64         `json:"messages_acked"`
	MessagesNacked    int64         `json:"messages_nacked"`
	MessagesRejected  int64         `json:"messages_rejected"`
	ProcessingErrors  int64         `json:"processing_errors"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	LastConsumeTime   time.Time     `json:"last_consume_time"`
	mu                sync.RWMutex  `json:"-"`
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	Handle(ctx context.Context, msg *Message) error
}

// MessageHandlerFunc 消息处理器函数类型
type MessageHandlerFunc func(ctx context.Context, msg *Message) error

// Handle 实现MessageHandler接口
func (f MessageHandlerFunc) Handle(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

// ConsumerOptions 消费者选项
type ConsumerOptions struct {
	ConsumerTag string     `json:"consumer_tag"`
	AutoAck     bool       `json:"auto_ack"`
	Exclusive   bool       `json:"exclusive"`
	NoLocal     bool       `json:"no_local"`
	NoWait      bool       `json:"no_wait"`
	Args        amqp.Table `json:"args"`
}

// NewConsumer 创建新的消费者
func NewConsumer(connection *Connection, queueName string, handler MessageHandler, options *ConsumerOptions) *Consumer {
	if options == nil {
		options = &ConsumerOptions{
			AutoAck:   false,
			Exclusive: false,
			NoLocal:   false,
			NoWait:    false,
		}
	}

	if options.ConsumerTag == "" {
		options.ConsumerTag = fmt.Sprintf("consumer-%s-%d", queueName, time.Now().UnixNano())
	}

	return &Consumer{
		connection:  connection,
		queueName:   queueName,
		consumerTag: options.ConsumerTag,
		autoAck:     options.AutoAck,
		exclusive:   options.Exclusive,
		noLocal:     options.NoLocal,
		noWait:      options.NoWait,
		args:        options.Args,
		handler:     handler,
		stopCh:      make(chan struct{}),
		metrics:     &ConsumerMetrics{},
	}
}

// Start 开始消费消息
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.isConsuming {
		c.mu.Unlock()
		return errors.New(errors.CodeValidationFailed,
			"Consumer is already running")
	}
	c.isConsuming = true
	c.mu.Unlock()

	logger.Info("Starting consumer",
		logger.String("queue", c.queueName),
		logger.String("consumer_tag", c.consumerTag),
		logger.Bool("auto_ack", c.autoAck),
	)

	// 获取通道
	channel, err := c.connection.GetChannel()
	if err != nil {
		c.mu.Lock()
		c.isConsuming = false
		c.mu.Unlock()
		return err
	}

	// 开始消费
	deliveries, err := channel.Consume(
		c.queueName,
		c.consumerTag,
		c.autoAck,
		c.exclusive,
		c.noLocal,
		c.noWait,
		c.args,
	)

	if err != nil {
		c.mu.Lock()
		c.isConsuming = false
		c.mu.Unlock()
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to start consumer for queue %s", c.queueName))
	}

	// 启动消息处理协程
	go c.processDeliveries(ctx, deliveries)

	logger.Info("Consumer started successfully",
		logger.String("queue", c.queueName),
		logger.String("consumer_tag", c.consumerTag),
	)

	return nil
}

// processDeliveries 处理消息投递
func (c *Consumer) processDeliveries(ctx context.Context, deliveries <-chan amqp.Delivery) {
	logger.Debug("Starting to process deliveries",
		logger.String("queue", c.queueName),
	)

	for {
		select {
		case <-c.stopCh:
			logger.Info("Consumer stopped",
				logger.String("queue", c.queueName),
				logger.String("consumer_tag", c.consumerTag),
			)
			return

		case delivery, ok := <-deliveries:
			if !ok {
				logger.Warn("Delivery channel closed",
					logger.String("queue", c.queueName),
				)

				// 通道关闭，可能是连接断开，尝试重启
				c.mu.Lock()
				c.isConsuming = false
				c.mu.Unlock()

				// 等待一段时间后尝试重新启动
				time.Sleep(5 * time.Second)
				if err := c.Start(ctx); err != nil {
					logger.Error("Failed to restart consumer",
						logger.String("queue", c.queueName),
						logger.ErrorField(err),
					)
				}
				return
			}

			// 处理消息
			c.processDelivery(ctx, delivery)
		}
	}
}

// processDelivery 处理单个消息投递
func (c *Consumer) processDelivery(ctx context.Context, delivery amqp.Delivery) {
	startTime := time.Now()

	// 转换消息格式
	msg := c.convertDeliveryToMessage(delivery)

	logger.Debug("Message received",
		logger.String("message_id", msg.ID),
		logger.String("queue", c.queueName),
		logger.String("consumer_tag", c.consumerTag),
		logger.Int("body_size", len(msg.Body)),
	)

	// 创建处理上下文
	processCtx := context.WithValue(ctx, "delivery", delivery)
	processCtx = context.WithValue(processCtx, "message", msg)

	// 调用消息处理器
	var processErr error
	defer func() {
		// 记录处理结果
		c.recordProcessing(processErr == nil, time.Since(startTime))

		if processErr != nil {
			c.metrics.recordError()
			logger.Error("Message processing failed",
				logger.String("message_id", msg.ID),
				logger.String("queue", c.queueName),
				logger.ErrorField(processErr),
			)
		}
	}()

	processErr = c.handler.Handle(processCtx, msg)

	// 根据处理结果确认消息
	c.handleAckNack(delivery, processErr)
}

// convertDeliveryToMessage 转换投递消息为Message
func (c *Consumer) convertDeliveryToMessage(delivery amqp.Delivery) *Message {
	msg := &Message{
		ID:            delivery.MessageId,
		Exchange:      delivery.Exchange,
		RoutingKey:    delivery.RoutingKey,
		Body:          delivery.Body,
		ContentType:   delivery.ContentType,
		DeliveryMode:  DeliveryMode(delivery.DeliveryMode),
		Priority:      delivery.Priority,
		CorrelationID: delivery.CorrelationId,
		ReplyTo:       delivery.ReplyTo,
		MessageID:     delivery.MessageId,
		Timestamp:     delivery.Timestamp,
		Type:          delivery.Type,
		UserID:        delivery.UserId,
		AppID:         delivery.AppId,
		Headers:       make(map[string]interface{}),
	}

	// 转换消息头
	if delivery.Headers != nil {
		for key, value := range delivery.Headers {
			msg.Headers[key] = value
		}
	}

	// 提取重试信息
	if retryCount, ok := delivery.Headers["x-retry-count"]; ok {
		if count, ok := retryCount.(int32); ok {
			msg.RetryCount = int(count)
		}
	}

	if maxRetries, ok := delivery.Headers["x-max-retries"]; ok {
		if max, ok := maxRetries.(int32); ok {
			msg.MaxRetries = int(max)
		}
	}

	if retryDelay, ok := delivery.Headers["x-retry-delay"]; ok {
		if delay, ok := retryDelay.(string); ok {
			if d, err := time.ParseDuration(delay); err == nil {
				msg.RetryDelay = d
			}
		}
	}

	return msg
}

// handleAckNack 处理消息确认/拒绝
func (c *Consumer) handleAckNack(delivery amqp.Delivery, processErr error) {
	if c.autoAck {
		// 自动确认模式下不进行手动确认
		return
	}

	if processErr == nil {
		// 处理成功，确认消息
		if err := delivery.Ack(false); err != nil {
			logger.Error("Failed to ack message",
				logger.String("message_id", delivery.MessageId),
				logger.ErrorField(err),
			)
		} else {
			c.metrics.recordAck()
			logger.Debug("Message acknowledged",
				logger.String("message_id", delivery.MessageId),
			)
		}
	} else {
		// 处理失败，根据错误类型决定是否重试
		if c.shouldRetry(delivery, processErr) {
			// 需要重试，拒绝消息并重新入队
			if err := delivery.Nack(false, true); err != nil {
				logger.Error("Failed to nack message",
					logger.String("message_id", delivery.MessageId),
					logger.ErrorField(err),
				)
			} else {
				c.metrics.recordNack()
				logger.Warn("Message nacked and requeued",
					logger.String("message_id", delivery.MessageId),
					logger.ErrorField(processErr),
				)
			}
		} else {
			// 不需要重试，拒绝消息并丢弃
			if err := delivery.Nack(false, false); err != nil {
				logger.Error("Failed to reject message",
					logger.String("message_id", delivery.MessageId),
					logger.ErrorField(err),
				)
			} else {
				c.metrics.recordReject()
				logger.Error("Message rejected",
					logger.String("message_id", delivery.MessageId),
					logger.ErrorField(processErr),
				)
			}
		}
	}
}

// shouldRetry 判断是否应该重试
func (c *Consumer) shouldRetry(delivery amqp.Delivery, processErr error) bool {
	// 检查消息头中的重试信息
	var retryCount, maxRetries int
	var retryDelay time.Duration

	if retryCountVal, ok := delivery.Headers["x-retry-count"]; ok {
		if count, ok := retryCountVal.(int32); ok {
			retryCount = int(count)
		}
	}

	if maxRetriesVal, ok := delivery.Headers["x-max-retries"]; ok {
		if max, ok := maxRetriesVal.(int32); ok {
			maxRetries = int(max)
		}
	}

	if retryDelayVal, ok := delivery.Headers["x-retry-delay"]; ok {
		if delay, ok := retryDelayVal.(string); ok {
			if d, err := time.ParseDuration(delay); err == nil {
				retryDelay = d
			}
		}
	}

	// 默认值
	if maxRetries == 0 {
		maxRetries = 3
	}

	if retryDelay == 0 {
		retryDelay = 5 * time.Second
	}

	// 检查是否达到最大重试次数
	if retryCount >= maxRetries {
		logger.Warn("Max retries exceeded",
			logger.String("message_id", delivery.MessageId),
			logger.Int("retry_count", retryCount),
			logger.Int("max_retries", maxRetries),
		)
		return false
	}

	// 检查错误类型，某些错误不应该重试
	if errors.Is(processErr, errors.CodeValidationFailed) {
		// 验证错误不应该重试
		return false
	}

	// 更新重试次数
	retryCount++

	logger.Info("Message will be retried",
		logger.String("message_id", delivery.MessageId),
		logger.Int("retry_count", retryCount),
		logger.Int("max_retries", maxRetries),
		logger.Duration("retry_delay", retryDelay),
		logger.ErrorField(processErr),
	)

	return true
}

// recordProcessing 记录处理指标
func (c *Consumer) recordProcessing(success bool, processingTime time.Duration) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.MessagesConsumed++
	c.metrics.LastConsumeTime = time.Now()

	// 更新平均处理时间
	if c.metrics.AvgProcessingTime == 0 {
		c.metrics.AvgProcessingTime = processingTime
	} else {
		// 简单移动平均
		c.metrics.AvgProcessingTime = (c.metrics.AvgProcessingTime + processingTime) / 2
	}

	// 更新连接指标
	c.connection.metrics.recordMessageReceived()
}

// recordAck 记录确认
func (c *Consumer) recordAck() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.MessagesAcked++
}

// recordNack 记录拒绝并重新入队
func (c *Consumer) recordNack() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.MessagesNacked++
}

// recordReject 记录拒绝并丢弃
func (c *Consumer) recordReject() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.MessagesRejected++
}

// recordError 记录处理错误
func (c *Consumer) recordError() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.ProcessingErrors++
}

// Stop 停止消费者
func (c *Consumer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConsuming {
		return nil
	}

	close(c.stopCh)
	c.isConsuming = false

	// 取消消费者
	channel, err := c.connection.GetChannel()
	if err == nil {
		channel.Cancel(c.consumerTag, false)
	}

	logger.Info("Consumer stopped",
		logger.String("queue", c.queueName),
		logger.String("consumer_tag", c.consumerTag),
	)

	return nil
}

// IsConsuming 检查是否正在消费
func (c *Consumer) IsConsuming() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConsuming
}

// GetMetrics 获取消费者指标
func (c *Consumer) GetMetrics() *ConsumerMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	return &ConsumerMetrics{
		MessagesConsumed:  c.metrics.MessagesConsumed,
		MessagesAcked:     c.metrics.MessagesAcked,
		MessagesNacked:    c.metrics.MessagesNacked,
		MessagesRejected:  c.metrics.MessagesRejected,
		ProcessingErrors:  c.metrics.ProcessingErrors,
		AvgProcessingTime: c.metrics.AvgProcessingTime,
		LastConsumeTime:   c.metrics.LastConsumeTime,
	}
}
