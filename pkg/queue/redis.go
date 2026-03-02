// pkg/queue/redis.go
package queue

import (
	"context"
	"time"

	"github.com/dgoukj/dgou-framework/pkg/logger"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type RedisQueue struct {
	client *redis.Client
	log    *logger.Logger
	prefix string
}

var _ Queue = (*RedisQueue)(nil)

func NewRedisQueue(client *redis.Client, log *logger.Logger) *RedisQueue {
	return &RedisQueue{
		client: client,
		log:    log,
		prefix: "queue:",
	}
}

func (q *RedisQueue) key(queueName string) string {
	return q.prefix + queueName
}

func (q *RedisQueue) Close() error { return nil }

func (q *RedisQueue) Publish(ctx context.Context, body []byte, opts ...PublishOption) error {
	opt := &PublishOptions{}
	for _, o := range opts {
		o(opt)
	}
	queueName := opt.RoutingKey
	if queueName == "" {
		queueName = "default"
	}
	return q.client.LPush(ctx, q.key(queueName), body).Err()
}

func (q *RedisQueue) Consume(ctx context.Context, handler func(ctx context.Context, msg *Message) error, opts ...ConsumeOption) error {
	opt := &ConsumeOptions{}
	for _, o := range opts {
		o(opt)
	}
	queueName := opt.Queue
	if queueName == "" {
		queueName = "default"
	}
	key := q.key(queueName)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			result, err := q.client.BRPop(ctx, 0*time.Second, key).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				q.log.Error("redis BRPOP failed", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}
			if len(result) < 2 {
				continue
			}
			msg := &Message{Body: []byte(result[1])}
			if err := handler(ctx, msg); err != nil {
				q.log.Error("handler failed", zap.Error(err))
			}
		}
	}
}

// 以下方法 Redis 不支持，返回 nil
func (q *RedisQueue) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args map[string]interface{}) error {
	return nil
}
func (q *RedisQueue) DeleteExchange(name string, ifUnused bool) error { return nil }
func (q *RedisQueue) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args map[string]interface{}) error {
	return nil
}
func (q *RedisQueue) DeleteQueue(name string, ifUnused, ifEmpty bool) error    { return nil }
func (q *RedisQueue) QueuePurge(name string) error                             { return nil }
func (q *RedisQueue) QueueInspect(name string) (map[string]interface{}, error) { return nil, nil }
func (q *RedisQueue) BindQueue(queueName, routingKey, exchange string, noWait bool, args map[string]interface{}) error {
	return nil
}
func (q *RedisQueue) UnbindQueue(queueName, routingKey, exchange string, args map[string]interface{}) error {
	return nil
}
