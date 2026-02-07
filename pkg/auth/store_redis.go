package auth

import (
	"context"
	"crypto/sha256"
	"dgou/pkg/cache"
	"dgou/pkg/errors"
	"fmt"
	"strconv"
	"time"
)

// RedisTokenStore Redis令牌存储实现
type RedisTokenStore struct {
	cache  cache.Cache
	prefix string
}

// NewRedisTokenStore 创建Redis令牌存储
func NewRedisTokenStore() *RedisTokenStore {
	// 获取全局缓存实例
	cacheInstance := cache.GetCache()
	if cacheInstance == nil {
		// 如果缓存未初始化，创建一个内存缓存作为后备
		cacheInstance = &cache.MemoryCache{}
	}

	return &RedisTokenStore{
		cache:  cacheInstance,
		prefix: "auth:token",
	}
}

// buildKey 构建键
func (rts *RedisTokenStore) buildKey(userID uint64, tokenType TokenType) string {
	return fmt.Sprintf("%s:%d:%s", rts.prefix, userID, tokenType)
}

// buildRevokedKey 构建吊销令牌键
func (rts *RedisTokenStore) buildRevokedKey(token string) string {
	// 使用token的哈希作为键，避免存储原始token
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%s:revoked:%x", rts.prefix, hash[:16])
}

// SaveToken 保存令牌
func (rts *RedisTokenStore) SaveToken(ctx context.Context, userID uint64, tokenType TokenType, token string, expiresIn time.Duration) error {
	key := rts.buildKey(userID, tokenType)

	// 存储令牌
	if err := rts.cache.Set(ctx, key, token, expiresIn); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to save token to Redis")
	}

	// 如果是刷新令牌，还需要存储用户ID到令牌的映射
	if tokenType == TokenTypeRefresh {
		tokenKey := fmt.Sprintf("%s:user:%s", rts.prefix, token)
		if err := rts.cache.Set(ctx, tokenKey, userID, expiresIn); err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to save token-user mapping")
		}
	}

	return nil
}

// GetToken 获取令牌
func (rts *RedisTokenStore) GetToken(ctx context.Context, userID uint64, tokenType TokenType) (string, error) {
	key := rts.buildKey(userID, tokenType)

	token, err := rts.cache.Get(ctx, key)
	if err != nil {
		if errors.Is(err, errors.CodeResourceNotFound) {
			return "", errors.New(errors.CodeResourceNotFound, "Token not found")
		}
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to get token from Redis")
	}

	return token, nil
}

// DeleteToken 删除令牌
func (rts *RedisTokenStore) DeleteToken(ctx context.Context, userID uint64, tokenType TokenType) error {
	key := rts.buildKey(userID, tokenType)

	// 如果是刷新令牌，先获取令牌值，然后删除映射
	if tokenType == TokenTypeRefresh {
		if token, err := rts.GetToken(ctx, userID, tokenType); err == nil {
			tokenKey := fmt.Sprintf("%s:user:%s", rts.prefix, token)
			_ = rts.cache.Delete(ctx, tokenKey)
		}
	}

	// 删除主令牌
	if err := rts.cache.Delete(ctx, key); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to delete token from Redis")
	}

	return nil
}

// DeleteAllTokens 删除用户所有令牌
func (rts *RedisTokenStore) DeleteAllTokens(ctx context.Context, userID uint64) error {
	// 删除访问令牌
	_ = rts.DeleteToken(ctx, userID, TokenTypeAccess)

	// 删除刷新令牌
	_ = rts.DeleteToken(ctx, userID, TokenTypeRefresh)

	return nil
}

// IsTokenRevoked 检查令牌是否被吊销
func (rts *RedisTokenStore) IsTokenRevoked(ctx context.Context, token string) (bool, error) {
	key := rts.buildRevokedKey(token)

	exists, err := rts.cache.Exists(ctx, key)
	if err != nil {
		return false, errors.Wrap(err, errors.CodeInternalError, "Failed to check token revocation status")
	}

	return exists, nil
}

// RevokeToken 吊销令牌
func (rts *RedisTokenStore) RevokeToken(ctx context.Context, token string, reason string) error {
	key := rts.buildRevokedKey(token)

	// 将吊销信息存储24小时
	revocationInfo := map[string]interface{}{
		"token":  token,
		"reason": reason,
		"time":   time.Now().Format(time.RFC3339),
	}

	if err := rts.cache.Set(ctx, key, revocationInfo, 24*time.Hour); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to revoke token")
	}

	return nil
}

// GetUserIDFromToken 从令牌获取用户ID
func (rts *RedisTokenStore) GetUserIDFromToken(ctx context.Context, token string) (uint64, error) {
	key := fmt.Sprintf("%s:user:%s", rts.prefix, token)

	value, err := rts.cache.Get(ctx, key)
	if err != nil {
		return 0, errors.Wrap(err, errors.CodeInternalError, "Failed to get user ID from token")
	}

	userID, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, errors.CodeInternalError, "Failed to parse user ID")
	}

	return userID, nil
}

// CleanupExpiredTokens 清理过期令牌
func (rts *RedisTokenStore) CleanupExpiredTokens(ctx context.Context) error {
	// Redis会自动清理过期的键，这里不需要额外操作
	return nil
}
