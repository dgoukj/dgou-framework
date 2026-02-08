package cache

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"sync"
	"time"
)

var (
	// globalCache 全局缓存实例
	globalCache *CacheManager
	// cacheOnce 确保单例初始化
	cacheOnce sync.Once
)

// InitCache 初始化缓存（单例模式）
func InitCache(cfg *config.Config) (*CacheManager, error) {
	var initErr error

	cacheOnce.Do(func() {
		// 创建缓存配置
		cacheConfig := &CacheConfig{
			Type:        RedisType,
			Prefix:      "app",
			DefaultTTL:  3600, // 默认1小时
			EnableStats: true,
			MaxMemoryMB: 100, // 内存缓存最大100MB
		}

		// 从应用配置覆盖
		if cfg.Redis.Addr != "" {
			cacheConfig.Type = RedisType
		}

		// 创建缓存管理器
		cacheManager, err := NewCacheManager(cacheConfig)
		if err != nil {
			initErr = err
			return
		}

		globalCache = cacheManager

		// 注册优雅关闭
		// 这里需要与应用的优雅关闭机制集成

		logger.Info("Cache initialized successfully",
			logger.String("type", string(cacheConfig.Type)),
			logger.Bool("redis_available", cacheManager.IsRedisAvailable()),
		)
	})

	return globalCache, initErr
}

// GetCache 获取全局缓存实例
func GetCache() *CacheManager {
	if globalCache == nil {
		logger.Error("Cache not initialized, please call InitCache first")
		// 尝试初始化
		cfg := config.GetConfig()
		cache, err := InitCache(cfg)
		if err != nil {
			logger.Error("Failed to initialize cache", logger.ErrorField(err))
			return nil
		}
		return cache
	}
	return globalCache
}

// CloseCache 关闭缓存连接
func CloseCache() error {
	if globalCache == nil {
		return nil
	}
	return globalCache.Close()
}

// 快捷方法
var (
	// Get 快捷获取
	Get = func(ctx context.Context, key string) (string, error) {
		cache := GetCache()
		if cache == nil {
			return "", errors.New(errors.CodeInternalError, "Cache not initialized")
		}
		return cache.Get(ctx, key)
	}

	// Set 快捷设置
	Set = func(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
		cache := GetCache()
		if cache == nil {
			return errors.New(errors.CodeInternalError, "Cache not initialized")
		}

		var expire time.Duration
		if len(ttl) > 0 {
			expire = ttl[0]
		}

		return cache.Set(ctx, key, value, expire)
	}

	// Delete 快捷删除
	Delete = func(ctx context.Context, key string) error {
		cache := GetCache()
		if cache == nil {
			return errors.New(errors.CodeInternalError, "Cache not initialized")
		}
		return cache.Delete(ctx, key)
	}

	// GetOrSet 快捷获取或设置
	GetOrSet = func(ctx context.Context, key string, fn func() (interface{}, error), ttl ...time.Duration) (string, error) {
		cache := GetCache()
		if cache == nil {
			return "", errors.New(errors.CodeInternalError, "Cache not initialized")
		}

		var expire time.Duration
		if len(ttl) > 0 {
			expire = ttl[0]
		}

		return cache.GetOrSet(ctx, key, fn, expire)
	}

	// Lock 快捷锁
	Lock = func(ctx context.Context, key string, ttl time.Duration) (string, error) {
		cache := GetCache()
		if cache == nil {
			return "", errors.New(errors.CodeInternalError, "Cache not initialized")
		}
		return cache.Lock(ctx, key, ttl)
	}

	// Unlock 快捷解锁
	Unlock = func(ctx context.Context, key, token string) error {
		cache := GetCache()
		if cache == nil {
			return errors.New(errors.CodeInternalError, "Cache not initialized")
		}
		return cache.Unlock(ctx, key, token)
	}
)
