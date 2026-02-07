// file: pkg/queue/producer.go
package queue

import (
	"context"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Producer 消息生产者
type Producer struct {
	connection *Connection      // 连接管理器
	exchange   string           // 交换机名称
	routingKey string           // 路由键
	mu         sync.RWMutex     // 读写锁
	metrics    *ProducerMetrics // 生产者指标
}

// ProducerMetrics 生产者指标
type ProducerMetrics struct {
	MessagesPublished int64         `json:"messages_published"`
	MessagesFailed    int64         `json:"messages_failed"`
	LastPublishTime   time.Time     `json:"last_publish_time"`
	PublishLatency    time.Duration `json:"publish_latency"`
	mu                sync.RWMutex  `json:"-"`
}

// NewProducer 创建新的生产者
func NewProducer(connection *Connection, exchange, routingKey string) *Producer {
	return &Producer{
		connection: connection,
		exchange:   exchange,
		routingKey: routingKey,
		metrics:    &ProducerMetrics{},
	}
}

// Publish 发布消息
func (p *Producer) Publish(ctx context.Context, msg *Message) error {
	startTime := time.Now()

	// 验证消息
	if err := p.validateMessage(msg); err != nil {
		return err
	}

	// 获取通道
	channel, err := p.connection.GetChannel()
	if err != nil {
		return err
	}

	// 准备发布选项
	publishing := p.preparePublishing(msg)

	// 发布消息
	err = channel.PublishWithContext(
		ctx,
		p.exchange,
		p.routingKey,
		false, // mandatory
		false, // immediate
		publishing,
	)

	// 记录指标
	p.recordPublish(err == nil, time.Since(startTime))

	if err != nil {
		p.metrics.recordFailure()
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to publish message to exchange %s", p.exchange))
	}

	logger.Debug("Message published",
		logger.String("message_id", msg.ID),
		logger.String("exchange", p.exchange),
		logger.String("routing_key", p.routingKey),
		logger.String("content_type", msg.ContentType),
	)

	return nil
}

// PublishWithRetry 发布消息（带重试）
func (p *Producer) PublishWithRetry(ctx context.Context, msg *Message, maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		err := p.Publish(ctx, msg)
		if err == nil {
			return nil
		}

		lastErr = err

		// 记录重试
		logger.Warn("Message publish failed, retrying...",
			logger.String("message_id", msg.ID),
			logger.Int("attempt", i+1),
			logger.Int("max_retries", maxRetries),
			logger.ErrorField(err),
		)

		// 等待重试延迟
		select {
		case <-time.After(retryDelay):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errors.Wrap(lastErr, errors.CodeInternalError,
		fmt.Sprintf("Failed to publish message after %d retries", maxRetries))
}

// PublishBatch 批量发布消息
func (p *Producer) PublishBatch(ctx context.Context, messages []*Message) (int, []error) {
	var errors []error
	successCount := 0

	for _, msg := range messages {
		if err := p.Publish(ctx, msg); err != nil {
			errors = append(errors, fmt.Errorf("message %s: %v", msg.ID, err))
		} else {
			successCount++
		}
	}

	logger.Info("Batch publish completed",
		logger.Int("total", len(messages)),
		logger.Int("success", successCount),
		logger.Int("failed", len(errors)),
	)

	return successCount, errors
}

// validateMessage 验证消息
func (p *Producer) validateMessage(msg *Message) error {
	if msg == nil {
		return errors.New(errors.CodeValidationFailed, "Message is nil")
	}

	if msg.ID == "" {
		return errors.New(errors.CodeValidationFailed, "Message ID is required")
	}

	if len(msg.Body) == 0 {
		return errors.New(errors.CodeValidationFailed, "Message body is empty")
	}

	return nil
}

// preparePublishing 准备发布选项
func (p *Producer) preparePublishing(msg *Message) amqp.Publishing {
	publishing := amqp.Publishing{
		ContentType:     msg.ContentType,
		ContentEncoding: "utf-8",
		DeliveryMode:    uint8(msg.DeliveryMode),
		Priority:        msg.Priority,
		CorrelationId:   msg.CorrelationID,
		ReplyTo:         msg.ReplyTo,
		MessageId:       msg.MessageID,
		Timestamp:       msg.Timestamp,
		Type:            msg.Type,
		UserId:          msg.UserID,
		AppId:           msg.AppID,
		Body:            msg.Body,
	}

	// 设置消息头
	if msg.Headers != nil {
		publishing.Headers = amqp.Table{}
		for key, value := range msg.Headers {
			publishing.Headers[key] = value
		}
	}

	// 添加重试信息
	if publishing.Headers == nil {
		publishing.Headers = amqp.Table{}
	}
	publishing.Headers["x-retry-count"] = msg.RetryCount
	publishing.Headers["x-max-retries"] = msg.MaxRetries
	publishing.Headers["x-retry-delay"] = msg.RetryDelay.String()

	// 设置过期时间
	if msg.Expiration != "" {
		publishing.Expiration = msg.Expiration
	}

	return publishing
}

// recordPublish 记录发布指标
func (p *Producer) recordPublish(success bool, latency time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	if success {
		p.metrics.MessagesPublished++
		p.metrics.LastPublishTime = time.Now()
		p.metrics.PublishLatency = latency

		// 更新连接指标
		p.connection.metrics.recordMessageSent()
	} else {
		p.metrics.MessagesFailed++
	}
}

// recordFailure 记录失败
func (p *Producer) recordFailure() {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.MessagesFailed++
}

// GetMetrics 获取生产者指标
func (p *Producer) GetMetrics() *ProducerMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	return &ProducerMetrics{
		MessagesPublished: p.metrics.MessagesPublished,
		MessagesFailed:    p.metrics.MessagesFailed,
		LastPublishTime:   p.metrics.LastPublishTime,
		PublishLatency:    p.metrics.PublishLatency,
	}
}

// CreateMessage 创建消息
func CreateMessage(id string, body interface{}) (*Message, error) {
	var bodyBytes []byte
	var contentType string

	switch v := body.(type) {
	case []byte:
		bodyBytes = v
		contentType = "application/octet-stream"
	case string:
		bodyBytes = []byte(v)
		contentType = "text/plain"
	default:
		// 尝试JSON序列化
		var err error
		bodyBytes, err = json.Marshal(v)
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError,
				"Failed to marshal message body")
		}
		contentType = "application/json"
	}

	return &Message{
		ID:           id,
		Body:         bodyBytes,
		ContentType:  contentType,
		DeliveryMode: DeliveryModePersistent,
		Timestamp:    time.Now(),
		RetryCount:   0,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
	}, nil
}

// SetExchange 设置交换机
func (p *Producer) SetExchange(exchange string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.exchange = exchange
}

// SetRoutingKey 设置路由键
func (p *Producer) SetRoutingKey(routingKey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.routingKey = routingKey
}
