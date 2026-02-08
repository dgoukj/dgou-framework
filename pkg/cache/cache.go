package cache

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CacheType 缓存类型
type CacheType string

const (
	RedisType  CacheType = "redis"  // Redis缓存
	MemoryType CacheType = "memory" // 内存缓存
)

// CacheConfig 缓存配置
type CacheConfig struct {
	Type        CacheType `mapstructure:"type"`          // 缓存类型：redis, memory
	Prefix      string    `mapstructure:"prefix"`        // 缓存键前缀
	DefaultTTL  int       `mapstructure:"default_ttl"`   // 默认过期时间(秒)
	EnableStats bool      `mapstructure:"enable_stats"`  // 是否启用统计
	MaxMemoryMB int       `mapstructure:"max_memory_mb"` // 最大内存限制(MB)，仅内存缓存有效
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits        int64     `json:"hits"`         // 命中次数
	Misses      int64     `json:"misses"`       // 未命中次数
	Sets        int64     `json:"sets"`         // 设置次数
	Deletes     int64     `json:"deletes"`      // 删除次数
	Evictions   int64     `json:"evictions"`    // 淘汰次数
	MemoryUsage int64     `json:"memory_usage"` // 内存使用量(字节)
	UpdatedAt   time.Time `json:"updated_at"`   // 最后更新时间
}

// Cache 缓存接口
type Cache interface {
	// 基础操作
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// 批量操作
	MGet(ctx context.Context, keys []string) (map[string]string, error)
	MSet(ctx context.Context, values map[string]interface{}, ttl time.Duration) error
	MDelete(ctx context.Context, keys []string) error

	// 高级操作
	GetOrSet(ctx context.Context, key string, fn func() (interface{}, error), ttl time.Duration) (string, error)
	Increment(ctx context.Context, key string, value int64) (int64, error)
	Decrement(ctx context.Context, key string, value int64) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)

	// 集合操作
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SRem(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)

	// 哈希操作
	HSet(ctx context.Context, key string, field string, value interface{}) error
	HGet(ctx context.Context, key string, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDelete(ctx context.Context, key string, fields ...string) error

	// 列表操作
	LPush(ctx context.Context, key string, values ...interface{}) error
	LPop(ctx context.Context, key string) (string, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// 发布订阅
	Publish(ctx context.Context, channel string, message interface{}) error
	Subscribe(ctx context.Context, channel string, handler func(message string)) error

	// 分布式锁
	Lock(ctx context.Context, key string, ttl time.Duration) (string, error)
	Unlock(ctx context.Context, key, token string) error
	TryLock(ctx context.Context, key string, ttl time.Duration, waitTime time.Duration) (string, error)

	// 位操作
	GetBit(ctx context.Context, key string, offset int64) (int64, error)
	SetBit(ctx context.Context, key string, offset int64, value int) error

	// 管理操作
	Clear(ctx context.Context) error
	Stats() CacheStats
	Close() error

	// 添加 Ping 方法
	Ping(ctx context.Context) error
}

// BloomFilter 布隆过滤器接口
type BloomFilter interface {
	Add(ctx context.Context, key string, value string) error
	Exists(ctx context.Context, key string, value string) (bool, error)
	Clear(ctx context.Context, key string) error
}

// CacheManager 缓存管理器
type CacheManager struct {
	primary   Cache        // 主缓存（Redis）
	secondary Cache        // 二级缓存（内存）
	config    *CacheConfig // 配置
	bloom     BloomFilter  // 布隆过滤器
	stats     *CacheStats  // 统计信息
	mu        sync.RWMutex // 读写锁
	isRedisUp bool         // Redis是否可用
	fallback  bool         // 是否降级到内存缓存
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(cfg *CacheConfig) (*CacheManager, error) {
	manager := &CacheManager{
		config: cfg,
		stats: &CacheStats{
			UpdatedAt: time.Now(),
		},
		isRedisUp: false,
		fallback:  false,
	}

	// 初始化Redis缓存
	if err := manager.initRedis(); err != nil {
		logger.Warn("Failed to initialize Redis cache, falling back to memory",
			logger.ErrorField(err),
		)
		manager.fallback = true
	}

	// 初始化内存缓存
	if err := manager.initMemoryCache(); err != nil {
		return nil, fmt.Errorf("failed to initialize memory cache: %w", err)
	}

	// 初始化布隆过滤器
	if err := manager.initBloomFilter(); err != nil {
		logger.Warn("Failed to initialize bloom filter",
			logger.ErrorField(err),
		)
	}

	// 启动健康检查
	go manager.healthCheck()

	logger.Info("Cache manager initialized",
		logger.String("type", string(cfg.Type)),
		logger.Bool("redis_available", manager.isRedisUp),
		logger.Bool("fallback", manager.fallback),
	)

	return manager, nil
}

// initRedis 初始化Redis缓存
func (cm *CacheManager) initRedis() error {
	redisConfig := config.GetConfig().Redis

	// 检查Redis配置
	if redisConfig.Addr == "" {
		return errors.New(errors.CodeInternalError, "Redis address not configured")
	}

	// 创建Redis客户端
	redisCache, err := NewRedisCache(&redisConfig, cm.config)
	if err != nil {
		return fmt.Errorf("failed to create Redis client: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := redisCache.Ping(ctx); err != nil {
		redisCache.Close()
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	cm.primary = redisCache
	cm.isRedisUp = true

	return nil
}

// initMemoryCache 初始化内存缓存
func (cm *CacheManager) initMemoryCache() error {
	memoryCache, err := NewMemoryCache(cm.config)
	if err != nil {
		return fmt.Errorf("failed to create memory cache: %w", err)
	}

	cm.secondary = memoryCache
	return nil
}

// initBloomFilter 初始化布隆过滤器
func (cm *CacheManager) initBloomFilter() error {
	// 使用Redis布隆过滤器
	if cm.isRedisUp {
		bloom, err := NewRedisBloomFilter(cm.primary)
		if err != nil {
			return fmt.Errorf("failed to create Redis bloom filter: %w", err)
		}
		cm.bloom = bloom
	} else {
		// 使用内存布隆过滤器
		bloom, err := NewMemoryBloomFilter(100000, 0.001) // 10万元素，0.1%误差率
		if err != nil {
			return fmt.Errorf("failed to create memory bloom filter: %w", err)
		}
		cm.bloom = bloom
	}

	return nil
}

// healthCheck 健康检查
func (cm *CacheManager) healthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.checkRedisHealth()
		}
	}
}

// checkRedisHealth 检查Redis健康状态
func (cm *CacheManager) checkRedisHealth() {
	if cm.primary == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 测试Redis连接
	err := cm.primary.Ping(ctx)
	if err != nil {
		if cm.isRedisUp {
			logger.Warn("Redis connection lost, switching to fallback mode",
				logger.ErrorField(err),
			)
			cm.isRedisUp = false
			cm.fallback = true
		}
	} else {
		if !cm.isRedisUp {
			logger.Info("Redis connection restored, switching back to Redis")
			cm.isRedisUp = true
			cm.fallback = false
		}
	}
}

// getCache 获取当前使用的缓存实例
func (cm *CacheManager) getCache() Cache {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.fallback || !cm.isRedisUp {
		return cm.secondary
	}
	return cm.primary
}

// updateStats 更新统计信息
func (cm *CacheManager) updateStats(hit bool) {
	if !cm.config.EnableStats {
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if hit {
		cm.stats.Hits++
	} else {
		cm.stats.Misses++
	}
	cm.stats.UpdatedAt = time.Now()
}

// ==================== 缓存接口实现 ====================

// Get 获取缓存值（带缓存穿透防护）
func (cm *CacheManager) Get(ctx context.Context, key string) (string, error) {
	cache := cm.getCache()

	// 检查布隆过滤器（如果存在）
	if cm.bloom != nil {
		exists, err := cm.bloom.Exists(ctx, "bloom:"+key, key)
		if err == nil && !exists {
			cm.updateStats(false)
			return "", errors.New(errors.CodeResourceNotFound, "Key not found in bloom filter")
		}
	}

	value, err := cache.Get(ctx, key)
	if err != nil {
		// 检查是否是"未找到"错误
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Key not found") {
			cm.updateStats(false)

			// 缓存穿透防护：缓存空值
			if cm.bloom != nil {
				_ = cm.bloom.Add(ctx, "bloom:"+key, key)
			}
			// 设置空值，防止缓存穿透
			_ = cache.Set(ctx, key, "", 30*time.Second)
		}
	}

	cm.updateStats(true)
	return value, nil
}

// Set 设置缓存值
func (cm *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	cache := cm.getCache()

	// 设置布隆过滤器
	if cm.bloom != nil {
		_ = cm.bloom.Add(ctx, "bloom:"+key, key)
	}

	err := cache.Set(ctx, key, value, ttl)
	if err == nil {
		cm.mu.Lock()
		cm.stats.Sets++
		cm.mu.Unlock()
	}

	// 同时更新二级缓存
	if cm.secondary != nil && cache != cm.secondary {
		_ = cm.secondary.Set(ctx, key, value, ttl)
	}

	return err
}

// Delete 删除缓存值
func (cm *CacheManager) Delete(ctx context.Context, key string) error {
	cache := cm.getCache()

	// 清除布隆过滤器
	if cm.bloom != nil {
		_ = cm.bloom.Clear(ctx, "bloom:"+key)
	}

	err := cache.Delete(ctx, key)
	if err == nil {
		cm.mu.Lock()
		cm.stats.Deletes++
		cm.mu.Unlock()
	}

	// 同时删除二级缓存
	if cm.secondary != nil {
		_ = cm.secondary.Delete(ctx, key)
	}

	return err
}

// GetOrSet 获取或设置缓存值（防缓存击穿）
func (cm *CacheManager) GetOrSet(ctx context.Context, key string, fn func() (interface{}, error), ttl time.Duration) (string, error) {
	// 先尝试获取缓存
	value, err := cm.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	// 使用分布式锁防止缓存击穿
	lockKey := "lock:" + key
	token, err := cm.Lock(ctx, lockKey, 5*time.Second)
	if err != nil {
		// 获取锁失败，等待并重试
		time.Sleep(100 * time.Millisecond)
		return cm.GetOrSet(ctx, key, fn, ttl)
	}
	defer cm.Unlock(ctx, lockKey, token)

	// 再次检查缓存（双检查锁）
	value, err = cm.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	// 执行生成函数
	data, err := fn()
	if err != nil {
		return "", err
	}

	// 设置缓存
	if err := cm.Set(ctx, key, data, ttl); err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", data), nil
}

// MGet 批量获取
func (cm *CacheManager) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	cache := cm.getCache()
	return cache.MGet(ctx, keys)
}

// MSet 批量设置
func (cm *CacheManager) MSet(ctx context.Context, values map[string]interface{}, ttl time.Duration) error {
	cache := cm.getCache()
	return cache.MSet(ctx, values, ttl)
}

// MDelete 批量删除
func (cm *CacheManager) MDelete(ctx context.Context, keys []string) error {
	cache := cm.getCache()
	return cache.MDelete(ctx, keys)
}

// Increment 自增
func (cm *CacheManager) Increment(ctx context.Context, key string, value int64) (int64, error) {
	cache := cm.getCache()
	return cache.Increment(ctx, key, value)
}

// Decrement 自减
func (cm *CacheManager) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	cache := cm.getCache()
	return cache.Decrement(ctx, key, value)
}

// Expire 设置过期时间
func (cm *CacheManager) Expire(ctx context.Context, key string, ttl time.Duration) error {
	cache := cm.getCache()
	return cache.Expire(ctx, key, ttl)
}

// TTL 获取剩余过期时间
func (cm *CacheManager) TTL(ctx context.Context, key string) (time.Duration, error) {
	cache := cm.getCache()
	return cache.TTL(ctx, key)
}

// SAdd 集合添加
func (cm *CacheManager) SAdd(ctx context.Context, key string, members ...interface{}) error {
	cache := cm.getCache()
	return cache.SAdd(ctx, key, members...)
}

// SRem 集合删除
func (cm *CacheManager) SRem(ctx context.Context, key string, members ...interface{}) error {
	cache := cm.getCache()
	return cache.SRem(ctx, key, members...)
}

// SMembers 获取集合成员
func (cm *CacheManager) SMembers(ctx context.Context, key string) ([]string, error) {
	cache := cm.getCache()
	return cache.SMembers(ctx, key)
}

// SIsMember 判断是否是集合成员
func (cm *CacheManager) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	cache := cm.getCache()
	return cache.SIsMember(ctx, key, member)
}

// HSet 哈希设置
func (cm *CacheManager) HSet(ctx context.Context, key string, field string, value interface{}) error {
	cache := cm.getCache()
	return cache.HSet(ctx, key, field, value)
}

// HGet 哈希获取
func (cm *CacheManager) HGet(ctx context.Context, key string, field string) (string, error) {
	cache := cm.getCache()
	return cache.HGet(ctx, key, field)
}

// HGetAll 获取所有哈希字段
func (cm *CacheManager) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	cache := cm.getCache()
	return cache.HGetAll(ctx, key)
}

// HDelete 哈希删除
func (cm *CacheManager) HDelete(ctx context.Context, key string, fields ...string) error {
	cache := cm.getCache()
	return cache.HDelete(ctx, key, fields...)
}

// LPush 列表推送
func (cm *CacheManager) LPush(ctx context.Context, key string, values ...interface{}) error {
	cache := cm.getCache()
	return cache.LPush(ctx, key, values...)
}

// LPop 列表弹出
func (cm *CacheManager) LPop(ctx context.Context, key string) (string, error) {
	cache := cm.getCache()
	return cache.LPop(ctx, key)
}

// LRange 获取列表范围
func (cm *CacheManager) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	cache := cm.getCache()
	return cache.LRange(ctx, key, start, stop)
}

// Publish 发布消息
func (cm *CacheManager) Publish(ctx context.Context, channel string, message interface{}) error {
	cache := cm.getCache()
	return cache.Publish(ctx, channel, message)
}

// Subscribe 订阅消息
func (cm *CacheManager) Subscribe(ctx context.Context, channel string, handler func(message string)) error {
	cache := cm.getCache()
	return cache.Subscribe(ctx, channel, handler)
}

// Lock 获取分布式锁
func (cm *CacheManager) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	cache := cm.getCache()
	return cache.Lock(ctx, key, ttl)
}

// Unlock 释放分布式锁
func (cm *CacheManager) Unlock(ctx context.Context, key, token string) error {
	cache := cm.getCache()
	return cache.Unlock(ctx, key, token)
}

// TryLock 尝试获取分布式锁
func (cm *CacheManager) TryLock(ctx context.Context, key string, ttl time.Duration, waitTime time.Duration) (string, error) {
	cache := cm.getCache()
	return cache.TryLock(ctx, key, ttl, waitTime)
}

// Clear 清空缓存
func (cm *CacheManager) Clear(ctx context.Context) error {
	cache := cm.getCache()
	return cache.Clear(ctx)
}

// Stats 获取统计信息
func (cm *CacheManager) Stats() CacheStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := *cm.stats

	stats.MemoryUsage = cm.secondary.Stats().MemoryUsage

	return stats
}

// Close 关闭缓存
func (cm *CacheManager) Close() error {
	var errs []error

	if cm.primary != nil {
		if err := cm.primary.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if cm.secondary != nil {
		if err := cm.secondary.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing cache: %v", errs)
	}

	return nil
}

// Ping 测试连接
func (cm *CacheManager) Ping(ctx context.Context) error {
	cache := cm.getCache()
	if pingable, ok := cache.(interface{ Ping(context.Context) error }); ok {
		return pingable.Ping(ctx)
	}
	return nil
}

// IsRedisAvailable 检查Redis是否可用
func (cm *CacheManager) IsRedisAvailable() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.isRedisUp
}

// EnableFallback 启用降级模式
func (cm *CacheManager) EnableFallback() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.fallback = true
}

// DisableFallback 禁用降级模式
func (cm *CacheManager) DisableFallback() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.fallback = false
}

// ==================== 哈希函数 ====================

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

// generateLockToken 生成分布式锁令牌
func generateLockToken() string {
	return fmt.Sprintf("lock:%d:%d", time.Now().UnixNano(), time.Now().Unix())
}
