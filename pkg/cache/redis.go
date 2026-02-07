package cache

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisCache Redis缓存实现
type RedisCache struct {
	client *redis.Client // Redis客户端
	config *CacheConfig  // 缓存配置
	prefix string        // 键前缀
}

// NewRedisCache 创建Redis缓存实例
func NewRedisCache(redisConfig *config.RedisConfig, cacheConfig *CacheConfig) (*RedisCache, error) {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     redisConfig.PoolSize,
		MinIdleConns: redisConfig.MinIdleConns,
		MaxRetries:   redisConfig.MaxRetries,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	rc := &RedisCache{
		client: client,
		config: cacheConfig,
		prefix: cacheConfig.Prefix,
	}

	return rc, nil
}

// buildKey 构建带前缀的键
func (rc *RedisCache) buildKey(key string) string {
	if rc.prefix == "" {
		return key
	}
	return rc.prefix + ":" + key
}

// Ping 测试连接
func (rc *RedisCache) Ping(ctx context.Context) error {
	return rc.client.Ping(ctx).Err()
}

// Get 获取缓存值
func (rc *RedisCache) Get(ctx context.Context, key string) (string, error) {
	fullKey := rc.buildKey(key)
	value, err := rc.client.Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errors.New(errors.CodeResourceNotFound, "Key not found")
		}
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to get value from Redis")
	}
	return value, nil
}

// Set 设置缓存值
func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := rc.buildKey(key)

	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	default:
		// 序列化为JSON
		data, err := json.Marshal(value)
		if err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
		}
		strValue = string(data)
	}

	if ttl <= 0 {
		ttl = time.Duration(rc.config.DefaultTTL) * time.Second
	}

	err := rc.client.Set(ctx, fullKey, strValue, ttl).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to set value in Redis")
	}

	return nil
}

// Delete 删除缓存值
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := rc.buildKey(key)
	err := rc.client.Del(ctx, fullKey).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to delete value from Redis")
	}
	return nil
}

// Exists 检查键是否存在
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := rc.buildKey(key)
	result, err := rc.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, errors.Wrap(err, errors.CodeInternalError, "Failed to check key existence")
	}
	return result > 0, nil
}

// MGet 批量获取
func (rc *RedisCache) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = rc.buildKey(key)
	}

	values, err := rc.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to MGet from Redis")
	}

	result := make(map[string]string)
	for i, value := range values {
		if value != nil {
			result[keys[i]] = value.(string)
		}
	}

	return result, nil
}

// MSet 批量设置
func (rc *RedisCache) MSet(ctx context.Context, values map[string]interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Duration(rc.config.DefaultTTL) * time.Second
	}

	pipe := rc.client.Pipeline()
	for key, value := range values {
		fullKey := rc.buildKey(key)

		var strValue string
		switch v := value.(type) {
		case string:
			strValue = v
		default:
			data, err := json.Marshal(value)
			if err != nil {
				return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
			}
			strValue = string(data)
		}

		pipe.Set(ctx, fullKey, strValue, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to MSet in Redis")
	}

	return nil
}

// MDelete 批量删除
func (rc *RedisCache) MDelete(ctx context.Context, keys []string) error {
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = rc.buildKey(key)
	}

	err := rc.client.Del(ctx, fullKeys...).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to MDelete from Redis")
	}
	return nil
}

// Increment 自增
func (rc *RedisCache) Increment(ctx context.Context, key string, value int64) (int64, error) {
	fullKey := rc.buildKey(key)
	result, err := rc.client.IncrBy(ctx, fullKey, value).Result()
	if err != nil {
		return 0, errors.Wrap(err, errors.CodeInternalError, "Failed to increment in Redis")
	}
	return result, nil
}

// Decrement 自减
func (rc *RedisCache) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	fullKey := rc.buildKey(key)
	result, err := rc.client.DecrBy(ctx, fullKey, value).Result()
	if err != nil {
		return 0, errors.Wrap(err, errors.CodeInternalError, "Failed to decrement in Redis")
	}
	return result, nil
}

// Expire 设置过期时间
func (rc *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := rc.buildKey(key)
	err := rc.client.Expire(ctx, fullKey, ttl).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to set expire in Redis")
	}
	return nil
}

// TTL 获取剩余过期时间
func (rc *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := rc.buildKey(key)
	ttl, err := rc.client.TTL(ctx, fullKey).Result()
	if err != nil {
		return 0, errors.Wrap(err, errors.CodeInternalError, "Failed to get TTL from Redis")
	}
	return ttl, nil
}

// SAdd 集合添加
func (rc *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	fullKey := rc.buildKey(key)
	err := rc.client.SAdd(ctx, fullKey, members...).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to SAdd in Redis")
	}
	return nil
}

// SRem 集合删除
func (rc *RedisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	fullKey := rc.buildKey(key)
	err := rc.client.SRem(ctx, fullKey, members...).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to SRem in Redis")
	}
	return nil
}

// SMembers 获取集合成员
func (rc *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	fullKey := rc.buildKey(key)
	members, err := rc.client.SMembers(ctx, fullKey).Result()
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to get SMembers from Redis")
	}
	return members, nil
}

// SIsMember 判断是否是集合成员
func (rc *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	fullKey := rc.buildKey(key)
	result, err := rc.client.SIsMember(ctx, fullKey, member).Result()
	if err != nil {
		return false, errors.Wrap(err, errors.CodeInternalError, "Failed to check SIsMember in Redis")
	}
	return result, nil
}

// HSet 哈希设置
func (rc *RedisCache) HSet(ctx context.Context, key string, field string, value interface{}) error {
	fullKey := rc.buildKey(key)

	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
		}
		strValue = string(data)
	}

	err := rc.client.HSet(ctx, fullKey, field, strValue).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to HSet in Redis")
	}
	return nil
}

// HGet 哈希获取
func (rc *RedisCache) HGet(ctx context.Context, key string, field string) (string, error) {
	fullKey := rc.buildKey(key)
	value, err := rc.client.HGet(ctx, fullKey, field).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errors.New(errors.CodeResourceNotFound, "Field not found")
		}
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to HGet from Redis")
	}
	return value, nil
}

// HGetAll 获取所有哈希字段
func (rc *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	fullKey := rc.buildKey(key)
	result, err := rc.client.HGetAll(ctx, fullKey).Result()
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to HGetAll from Redis")
	}
	return result, nil
}

// HDelete 哈希删除
func (rc *RedisCache) HDelete(ctx context.Context, key string, fields ...string) error {
	fullKey := rc.buildKey(key)
	err := rc.client.HDel(ctx, fullKey, fields...).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to HDelete from Redis")
	}
	return nil
}

// LPush 列表推送
func (rc *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	fullKey := rc.buildKey(key)

	var strValues []interface{}
	for _, value := range values {
		switch v := value.(type) {
		case string:
			strValues = append(strValues, v)
		default:
			data, err := json.Marshal(value)
			if err != nil {
				return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
			}
			strValues = append(strValues, string(data))
		}
	}

	err := rc.client.LPush(ctx, fullKey, strValues...).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to LPush in Redis")
	}
	return nil
}

// LPop 列表弹出
func (rc *RedisCache) LPop(ctx context.Context, key string) (string, error) {
	fullKey := rc.buildKey(key)
	value, err := rc.client.LPop(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errors.New(errors.CodeResourceNotFound, "List is empty")
		}
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to LPop from Redis")
	}
	return value, nil
}

// LRange 获取列表范围
func (rc *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	fullKey := rc.buildKey(key)
	values, err := rc.client.LRange(ctx, fullKey, start, stop).Result()
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to LRange from Redis")
	}
	return values, nil
}

// Publish 发布消息
func (rc *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	var strMessage string
	switch msg := message.(type) {
	case string:
		strMessage = msg
	default:
		data, err := json.Marshal(message)
		if err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal message")
		}
		strMessage = string(data)
	}

	err := rc.client.Publish(ctx, channel, strMessage).Err()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to publish message")
	}
	return nil
}

// Subscribe 订阅消息
func (rc *RedisCache) Subscribe(ctx context.Context, channel string, handler func(message string)) error {
	pubsub := rc.client.Subscribe(ctx, channel)

	go func() {
		defer pubsub.Close()

		ch := pubsub.Channel()
		for msg := range ch {
			handler(msg.Payload)
		}
	}()

	return nil
}

// Lock 获取分布式锁
func (rc *RedisCache) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	fullKey := "lock:" + rc.buildKey(key)
	token := generateLockToken()

	// 使用SET NX EX命令获取锁
	result, err := rc.client.SetNX(ctx, fullKey, token, ttl).Result()
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to acquire lock")
	}

	if !result {
		return "", errors.New(errors.CodeInternalError, "Lock already acquired")
	}

	return token, nil
}

// Unlock 释放分布式锁
func (rc *RedisCache) Unlock(ctx context.Context, key, token string) error {
	fullKey := "lock:" + rc.buildKey(key)

	// 使用Lua脚本确保原子性操作
	luaScript := `
    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("del", KEYS[1])
    else
        return 0
    end
    `

	cmd := rc.client.Eval(ctx, luaScript, []string{fullKey}, token)
	if cmd.Err() != nil {
		return errors.Wrap(cmd.Err(), errors.CodeInternalError, "Failed to release lock")
	}

	return nil
}

// TryLock 尝试获取分布式锁
func (rc *RedisCache) TryLock(ctx context.Context, key string, ttl time.Duration, waitTime time.Duration) (string, error) {
	fullKey := "lock:" + rc.buildKey(key)
	token := generateLockToken()

	start := time.Now()
	for time.Since(start) < waitTime {
		result, err := rc.client.SetNX(ctx, fullKey, token, ttl).Result()
		if err != nil {
			return "", errors.Wrap(err, errors.CodeInternalError, "Failed to try lock")
		}

		if result {
			return token, nil
		}

		// 等待一小段时间再重试
		time.Sleep(50 * time.Millisecond)
	}

	return "", errors.New(errors.CodeInternalError, "Timeout acquiring lock")
}

// Clear 清空缓存
func (rc *RedisCache) Clear(ctx context.Context) error {
	// 注意：在生产环境中要小心使用，避免误操作
	if rc.prefix == "" {
		return errors.New(errors.CodeInternalError, "Cannot clear all Redis data without prefix")
	}

	// 使用SCAN迭代删除所有带前缀的键
	var cursor uint64
	var keys []string

	for {
		var err error
		keys, cursor, err = rc.client.Scan(ctx, cursor, rc.prefix+":*", 100).Result()
		if err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to scan keys")
		}

		if len(keys) > 0 {
			if err := rc.client.Del(ctx, keys...).Err(); err != nil {
				return errors.Wrap(err, errors.CodeInternalError, "Failed to delete keys")
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}

// Close 关闭Redis连接
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// generateLockToken 生成锁令牌
func generateLockToken() string {
	return fmt.Sprintf("lock:%d:%d", time.Now().UnixNano(), time.Now().Unix())
}

// GetClient 获取Redis客户端（用于高级操作）
func (rc *RedisCache) GetClient() *redis.Client {
	return rc.client
}

// Stats 获取Redis统计信息
func (rc *RedisCache) Stats() *redis.PoolStats {
	return rc.client.PoolStats()
}

// RedisBloomFilter Redis布隆过滤器实现
type RedisBloomFilter struct {
	cache Cache
}

// NewRedisBloomFilter 创建Redis布隆过滤器
func NewRedisBloomFilter(cache Cache) (*RedisBloomFilter, error) {
	return &RedisBloomFilter{
		cache: cache,
	}, nil
}

// Add 添加元素到布隆过滤器
func (bf *RedisBloomFilter) Add(ctx context.Context, key string, value string) error {
	// Redis布隆过滤器使用RedisBloom模块
	// 这里简化实现，使用位图模拟
	// 实际项目中建议使用RedisBloom模块

	// 计算哈希值
	hash1 := hash1(value)
	hash2 := hash2(value)

	// 设置位
	if err := bf.cache.SetBit(ctx, key, hash1, 1); err != nil {
		return err
	}
	if err := bf.cache.SetBit(ctx, key, hash2, 1); err != nil {
		return err
	}

	return nil
}

// Exists 检查元素是否存在
func (bf *RedisBloomFilter) Exists(ctx context.Context, key string, value string) (bool, error) {
	// 计算哈希值
	hash1 := hash1(value)
	hash2 := hash2(value)

	// 获取位
	bit1, err := bf.cache.GetBit(ctx, key, hash1)
	if err != nil {
		return false, err
	}

	bit2, err := bf.cache.GetBit(ctx, key, hash2)
	if err != nil {
		return false, err
	}

	return bit1 == 1 && bit2 == 1, nil
}

// Clear 清空布隆过滤器
func (bf *RedisBloomFilter) Clear(ctx context.Context, key string) error {
	return bf.cache.Delete(ctx, key)
}

// hash1 哈希函数1
func hash1(value string) int64 {
	// 简单哈希函数，实际项目中应使用更好的哈希算法
	var hash int64 = 5381
	for i := 0; i < len(value); i++ {
		hash = ((hash << 5) + hash) + int64(value[i])
	}
	return hash % (1 << 32)
}

// hash2 哈希函数2
func hash2(value string) int64 {
	// 另一个哈希函数
	var hash int64 = 0
	for i := 0; i < len(value); i++ {
		hash = (hash * 31) + int64(value[i])
	}
	return hash % (1 << 32)
}
