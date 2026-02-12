package queue

import (
	"context"
	"time"
)

// Message RabbitMQ 消息封装
type Message struct {
	Body            []byte
	ContentType     string
	ContentEncoding string
	DeliveryMode    uint8 // 1=非持久化, 2=持久化
	Priority        uint8
	CorrelationID   string
	ReplyTo         string
	Expiration      string
	MessageID       string
	Timestamp       time.Time
	Type            string
	UserID          string
	AppID           string
	Headers         map[string]interface{}

	// 内部字段（用于消息确认）
	deliveryTag uint64
	ackFn       func(multiple bool) error
	nackFn      func(multiple, requeue bool) error
}

// Ack 确认消息
func (m *Message) Ack(multiple bool) error {
	if m.ackFn != nil {
		return m.ackFn(multiple)
	}
	return nil
}

// Nack 拒绝消息（可选择是否重新入队）
func (m *Message) Nack(multiple, requeue bool) error {
	if m.nackFn != nil {
		return m.nackFn(multiple, requeue)
	}
	return nil
}

// Reject 拒绝消息（单个，相当于 Nack(false, requeue)）
func (m *Message) Reject(requeue bool) error {
	return m.Nack(false, requeue)
}

// Queue 队列操作接口（完整定义）
type Queue interface {
	// 连接管理
	Close() error

	// 发布消息
	Publish(ctx context.Context, body []byte, opts ...PublishOption) error

	// 消费消息
	Consume(ctx context.Context, handler func(ctx context.Context, msg *Message) error, opts ...ConsumeOption) error

	// 交换机操作
	DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args map[string]interface{}) error
	DeleteExchange(name string, ifUnused bool) error

	// 队列操作
	DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args map[string]interface{}) error
	DeleteQueue(name string, ifUnused, ifEmpty bool) error
	QueuePurge(name string) error
	QueueInspect(name string) (map[string]interface{}, error)

	// 绑定操作
	BindQueue(queueName, routingKey, exchange string, noWait bool, args map[string]interface{}) error
	UnbindQueue(queueName, routingKey, exchange string, args map[string]interface{}) error
}
