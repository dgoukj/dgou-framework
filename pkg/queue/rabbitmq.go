package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	pkgErrors "github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/dgoukj/dgou-framework/pkg/logger" // 请根据实际模块名调整
)

// RabbitMQConfig RabbitMQ 配置
type RabbitMQConfig struct {
	URL            string
	Host           string
	Port           int
	Username       string
	Password       string
	Vhost          string
	Heartbeat      int
	DialTimeout    time.Duration
	MaxRetries     int
	RetryDelay     time.Duration
	PrefetchCount  int
	PrefetchSize   int
	GlobalPrefetch bool
}

// RabbitMQQueue RabbitMQ 实现
type RabbitMQQueue struct {
	config      *RabbitMQConfig
	conn        *amqp.Connection
	channel     *amqp.Channel
	mu          sync.RWMutex
	closed      atomic.Bool
	reconnect   chan struct{}
	done        chan struct{}
	logger      *logger.Logger
	notifyClose chan *amqp.Error
}

// NewRabbitMQ 创建 RabbitMQ 客户端
func NewRabbitMQ(cfg *RabbitMQConfig, log *logger.Logger) (*RabbitMQQueue, error) {
	if log == nil {
		return nil, pkgErrors.New("logger is required")
	}
	q := &RabbitMQQueue{
		config:    cfg,
		logger:    log,
		reconnect: make(chan struct{}),
		done:      make(chan struct{}),
	}
	if err := q.connect(); err != nil {
		return nil, err
	}
	go q.reconnector()
	return q, nil
}

// connect 建立连接和通道
func (q *RabbitMQQueue) connect() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}

	// 构建连接字符串
	var amqpURL string
	if q.config.URL != "" {
		amqpURL = q.config.URL
	} else {
		amqpURL = fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
			q.config.Username, q.config.Password,
			q.config.Host, q.config.Port,
			q.config.Vhost)
	}

	// 连接参数
	config := amqp.Config{
		Heartbeat: time.Duration(q.config.Heartbeat) * time.Second,
		Dial:      amqp.DefaultDial(q.config.DialTimeout),
	}

	conn, err := amqp.DialConfig(amqpURL, config)
	if err != nil {
		return pkgErrors.Wrap(err, "dial rabbitmq")
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return pkgErrors.Wrap(err, "open channel")
	}

	// 设置 QoS
	if err := ch.Qos(q.config.PrefetchCount, q.config.PrefetchSize, q.config.GlobalPrefetch); err != nil {
		q.logger.Warn("set qos failed", zap.Error(err))
	}

	q.conn = conn
	q.channel = ch
	q.notifyClose = make(chan *amqp.Error)
	q.channel.NotifyClose(q.notifyClose)
	q.conn.NotifyClose(q.notifyClose)

	q.logger.Info("rabbitmq connected", zap.String("url", amqpURL))
	return nil
}

// reconnector 自动重连
func (q *RabbitMQQueue) reconnector() {
	for {
		select {
		case <-q.done:
			return
		case err := <-q.notifyClose:
			if err != nil {
				q.logger.Error("rabbitmq connection closed", zap.Error(err))
				retries := 0
				maxRetries := q.config.MaxRetries
				if maxRetries <= 0 {
					maxRetries = 10
				}
				for {
					if retries >= maxRetries {
						q.logger.Fatal("rabbitmq max retries exceeded, giving up")
						return
					}
					select {
					case <-q.done:
						return
					case <-time.After(q.config.RetryDelay):
					}
					if err := q.connect(); err == nil {
						q.logger.Info("rabbitmq reconnected")
						break
					}
					retries++
				}
			}
		}
	}
}

// Close 关闭队列
func (q *RabbitMQQueue) Close() error {
	if q.closed.CompareAndSwap(false, true) {
		close(q.done)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.channel != nil {
		_ = q.channel.Close()
	}
	if q.conn != nil {
		_ = q.conn.Close()
	}
	return nil
}

// Publish 发布消息
func (q *RabbitMQQueue) Publish(ctx context.Context, body []byte, opts ...PublishOption) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}

	opt := &PublishOptions{
		Exchange:        "",
		RoutingKey:      "",
		Mandatory:       false,
		Immediate:       false,
		ContentType:     "application/octet-stream",
		ContentEncoding: "",
		DeliveryMode:    amqp.Persistent,
		Priority:        0,
		Timestamp:       time.Now(),
	}
	for _, o := range opts {
		o(opt)
	}

	publishing := amqp.Publishing{
		ContentType:     opt.ContentType,
		ContentEncoding: opt.ContentEncoding,
		DeliveryMode:    opt.DeliveryMode,
		Priority:        opt.Priority,
		CorrelationId:   opt.CorrelationID,
		ReplyTo:         opt.ReplyTo,
		Expiration:      opt.Expiration,
		MessageId:       opt.MessageID,
		Timestamp:       opt.Timestamp,
		Type:            opt.Type,
		UserId:          opt.UserID,
		AppId:           opt.AppID,
		Body:            body,
	}
	if opt.Headers != nil {
		publishing.Headers = amqp.Table(opt.Headers)
	}

	return ch.PublishWithContext(ctx,
		opt.Exchange,
		opt.RoutingKey,
		opt.Mandatory,
		opt.Immediate,
		publishing,
	)
}

// Consume 消费消息
func (q *RabbitMQQueue) Consume(ctx context.Context, handler func(ctx context.Context, msg *Message) error, opts ...ConsumeOption) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}

	opt := &ConsumeOptions{
		Queue:          "",
		Consumer:       "",
		AutoAck:        false,
		Exclusive:      false,
		NoLocal:        false,
		NoWait:         false,
		Args:           nil,
		PrefetchCount:  q.config.PrefetchCount,
		PrefetchSize:   q.config.PrefetchSize,
		GlobalPrefetch: q.config.GlobalPrefetch,
	}
	for _, o := range opts {
		o(opt)
	}

	// 设置 QoS
	if err := ch.Qos(opt.PrefetchCount, opt.PrefetchSize, opt.GlobalPrefetch); err != nil {
		return pkgErrors.Wrap(err, "set qos")
	}

	deliveries, err := ch.Consume(
		opt.Queue,
		opt.Consumer,
		opt.AutoAck,
		opt.Exclusive,
		opt.NoLocal,
		opt.NoWait,
		opt.Args,
	)
	if err != nil {
		return pkgErrors.Wrap(err, "consume")
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-q.done:
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				msg := &Message{
					Body:            d.Body,
					ContentType:     d.ContentType,
					ContentEncoding: d.ContentEncoding,
					DeliveryMode:    d.DeliveryMode,
					Priority:        d.Priority,
					CorrelationID:   d.CorrelationId,
					ReplyTo:         d.ReplyTo,
					Expiration:      d.Expiration,
					MessageID:       d.MessageId,
					Timestamp:       d.Timestamp,
					Type:            d.Type,
					UserID:          d.UserId,
					AppID:           d.AppId,
					Headers:         map[string]interface{}(d.Headers),
					deliveryTag:     d.DeliveryTag,
					ackFn: func(multiple bool) error {
						return d.Ack(multiple)
					},
					nackFn: func(multiple, requeue bool) error {
						return d.Nack(multiple, requeue)
					},
				}
				// 调用业务处理器
				if err := handler(ctx, msg); err != nil {
					// 处理失败，拒绝并重新入队
					_ = msg.Nack(false, true)
				} else {
					if !opt.AutoAck {
						_ = msg.Ack(false)
					}
				}
			}
		}
	}()

	return nil
}

// DeclareExchange 声明交换机
func (q *RabbitMQQueue) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args map[string]interface{}) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}
	return ch.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, amqp.Table(args))
}

// DeleteExchange 删除交换机
func (q *RabbitMQQueue) DeleteExchange(name string, ifUnused bool) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}
	return ch.ExchangeDelete(name, ifUnused, false)
}

// DeclareQueue 声明队列
func (q *RabbitMQQueue) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args map[string]interface{}) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}
	_, err := ch.QueueDeclare(name, durable, autoDelete, exclusive, noWait, amqp.Table(args))
	return err
}

// DeleteQueue 删除队列
func (q *RabbitMQQueue) DeleteQueue(name string, ifUnused, ifEmpty bool) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}
	_, err := ch.QueueDelete(name, ifUnused, ifEmpty, false)
	return err
}

// QueuePurge 清空队列
func (q *RabbitMQQueue) QueuePurge(name string) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}
	_, err := ch.QueuePurge(name, false)
	return err
}

// QueueInspect 查看队列信息（返回名称、消息数、消费者数）
// 注意：AMQP 协议不返回队列的 durable/auto_delete/exclusive 等属性，因此该方法只返回有限信息。
func (q *RabbitMQQueue) QueueInspect(name string) (map[string]interface{}, error) {
	if q.closed.Load() {
		return nil, pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return nil, pkgErrors.New("channel not available")
	}
	info, err := ch.QueueInspect(name)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"name":      info.Name,
		"messages":  info.Messages,
		"consumers": info.Consumers,
	}, nil
}

// BindQueue 绑定队列到交换机
func (q *RabbitMQQueue) BindQueue(queueName, routingKey, exchange string, noWait bool, args map[string]interface{}) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}
	return ch.QueueBind(queueName, routingKey, exchange, noWait, amqp.Table(args))
}

// UnbindQueue 解绑队列
func (q *RabbitMQQueue) UnbindQueue(queueName, routingKey, exchange string, args map[string]interface{}) error {
	if q.closed.Load() {
		return pkgErrors.New("queue is closed")
	}
	q.mu.RLock()
	ch := q.channel
	q.mu.RUnlock()
	if ch == nil {
		return pkgErrors.New("channel not available")
	}
	return ch.QueueUnbind(queueName, routingKey, exchange, amqp.Table(args))
}
