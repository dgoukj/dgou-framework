package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	pkgErrors "github.com/pkg/errors"
)

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type RedisCache struct {
	client *redis.Client
	prefix string
}

func NewRedis(cfg RedisConfig, prefix string) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, pkgErrors.Wrap(err, "redis ping")
	}
	return &RedisCache{
		client: client,
		prefix: prefix,
	}, nil
}

func (r *RedisCache) key(k string) string {
	if r.prefix == "" {
		return k
	}
	return r.prefix + ":" + k
}

func (r *RedisCache) GetClient() *redis.Client {
	return r.client
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, r.key(key)).Result()
	if err == redis.Nil {
		return "", pkgErrors.New("key not found")
	}
	return val, err
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var str string
	switch v := value.(type) {
	case string:
		str = v
	default:
		b, _ := json.Marshal(v)
		str = string(b)
	}
	return r.client.Set(ctx, r.key(key), str, ttl).Err()
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.key(key)).Err()
}

func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, r.key(key)).Result()
	return n > 0, err
}

func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

func (r *RedisCache) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	return r.client.IncrBy(ctx, r.key(key), delta).Result()
}

func (r *RedisCache) Decr(ctx context.Context, key string, delta int64) (int64, error) {
	return r.client.DecrBy(ctx, r.key(key), delta).Result()
}

func (r *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, r.key(key), members...).Err()
}
func (r *RedisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SRem(ctx, r.key(key), members...).Err()
}
func (r *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, r.key(key)).Result()
}
func (r *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, r.key(key), member).Result()
}

func (r *RedisCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	return r.client.HSet(ctx, r.key(key), field, value).Err()
}
func (r *RedisCache) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, r.key(key), field).Result()
}
func (r *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, r.key(key)).Result()
}
func (r *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, r.key(key), fields...).Err()
}

func (r *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.LPush(ctx, r.key(key), values...).Err()
}
func (r *RedisCache) LPop(ctx context.Context, key string) (string, error) {
	return r.client.LPop(ctx, r.key(key)).Result()
}
func (r *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, r.key(key), start, stop).Result()
}

func (r *RedisCache) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	token := generateToken()
	ok, err := r.client.SetNX(ctx, r.key("lock:"+key), token, ttl).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", pkgErrors.New("lock already acquired")
	}
	return token, nil
}

func (r *RedisCache) Unlock(ctx context.Context, key, token string) error {
	lua := `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
	_, err := r.client.Eval(ctx, lua, []string{r.key("lock:" + key)}, token).Result()
	return err
}

func generateToken() string {
	b := make([]byte, 16)
	// 实际可使用 uuid.New().String()
	return fmt.Sprintf("%x", b)
}
