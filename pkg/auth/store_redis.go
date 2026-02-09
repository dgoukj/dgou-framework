package auth

import (
	"context"
	"crypto/sha256"
	"dgou/pkg/errors"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// CacheAdapter 缓存适配器接口
type CacheAdapter interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SRem(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

// RedisTokenStore Redis令牌存储实现
type RedisTokenStore struct {
	cache  CacheAdapter
	prefix string
}

// NewRedisTokenStore 创建Redis令牌存储
func NewRedisTokenStore() *RedisTokenStore {
	return &RedisTokenStore{
		cache:  NewMemoryCacheAdapter(),
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
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to get token from Redis")
	}

	if token == "" {
		return "", errors.New(errors.CodeResourceNotFound, "Token not found")
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

	// 序列化吊销信息
	infoJSON, err := json.Marshal(revocationInfo)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal revocation info")
	}

	if err := rts.cache.Set(ctx, key, string(infoJSON), 24*time.Hour); err != nil {
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

	if value == "" {
		return 0, errors.New(errors.CodeResourceNotFound, "User ID not found for token")
	}

	userID, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, errors.CodeInternalError, "Failed to parse user ID")
	}

	return userID, nil
}

// CleanupExpiredTokens 清理过期令牌
func (rts *RedisTokenStore) CleanupExpiredTokens(ctx context.Context) error {
	// 内存缓存会自动清理过期的键，这里不需要额外操作
	return nil
}

// ==================== 内存缓存适配器实现 ====================

// MemoryCacheAdapter 内存缓存适配器
type MemoryCacheAdapter struct {
	data map[string]cacheEntry
	mu   sync.RWMutex
}

type cacheEntry struct {
	value      string
	expiration time.Time
}

// NewMemoryCacheAdapter 创建内存缓存适配器
func NewMemoryCacheAdapter() *MemoryCacheAdapter {
	adapter := &MemoryCacheAdapter{
		data: make(map[string]cacheEntry),
	}

	// 启动后台清理任务
	go adapter.cleanupExpired()

	return adapter
}

// Set 设置值
func (m *MemoryCacheAdapter) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var valueStr string
	switch v := value.(type) {
	case string:
		valueStr = v
	case int, int64, uint, uint64:
		valueStr = fmt.Sprintf("%d", v)
	default:
		// 尝试 JSON 序列化
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
		}
		valueStr = string(jsonBytes)
	}

	m.data[key] = cacheEntry{
		value:      valueStr,
		expiration: time.Now().Add(expiration),
	}

	return nil
}

// Get 获取值
func (m *MemoryCacheAdapter) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	if !exists {
		return "", nil
	}

	// 检查是否过期
	if entry.expiration.Before(time.Now()) {
		// 异步删除过期键
		go func() {
			m.mu.Lock()
			delete(m.data, key)
			m.mu.Unlock()
		}()
		return "", nil
	}

	return entry.value, nil
}

// Delete 删除键
func (m *MemoryCacheAdapter) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

// Exists 检查键是否存在
func (m *MemoryCacheAdapter) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	if !exists {
		return false, nil
	}

	// 检查是否过期
	if entry.expiration.Before(time.Now()) {
		return false, nil
	}

	return true, nil
}

// SAdd 添加到集合
func (m *MemoryCacheAdapter) SAdd(ctx context.Context, key string, members ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取现有集合
	entry, exists := m.data[key]
	var set map[string]bool

	if exists {
		// 解析现有集合
		var existingMembers []string
		if err := json.Unmarshal([]byte(entry.value), &existingMembers); err == nil {
			set = make(map[string]bool)
			for _, member := range existingMembers {
				set[member] = true
			}
		}
	}

	if set == nil {
		set = make(map[string]bool)
	}

	// 添加新成员
	for _, member := range members {
		switch v := member.(type) {
		case string:
			set[v] = true
		default:
			set[fmt.Sprintf("%v", v)] = true
		}
	}

	// 转换回切片
	var result []string
	for member := range set {
		result = append(result, member)
	}

	// 序列化并存储
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal set")
	}

	m.data[key] = cacheEntry{
		value:      string(jsonBytes),
		expiration: entry.expiration,
	}

	return nil
}

// SRem 从集合中移除成员
func (m *MemoryCacheAdapter) SRem(ctx context.Context, key string, members ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.data[key]
	if !exists {
		return nil
	}

	// 解析现有集合
	var existingMembers []string
	if err := json.Unmarshal([]byte(entry.value), &existingMembers); err != nil {
		return nil // 如果解析失败，视为空集合
	}

	// 转换为 map 便于删除
	set := make(map[string]bool)
	for _, member := range existingMembers {
		set[member] = true
	}

	// 删除指定成员
	for _, member := range members {
		switch v := member.(type) {
		case string:
			delete(set, v)
		default:
			delete(set, fmt.Sprintf("%v", v))
		}
	}

	// 转换回切片
	var result []string
	for member := range set {
		result = append(result, member)
	}

	// 序列化并存储
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal set")
	}

	m.data[key] = cacheEntry{
		value:      string(jsonBytes),
		expiration: entry.expiration,
	}

	return nil
}

// SMembers 获取集合所有成员
func (m *MemoryCacheAdapter) SMembers(ctx context.Context, key string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	if !exists {
		return []string{}, nil
	}

	// 检查是否过期
	if entry.expiration.Before(time.Now()) {
		return []string{}, nil
	}

	// 解析集合
	var members []string
	if err := json.Unmarshal([]byte(entry.value), &members); err != nil {
		return []string{}, nil
	}

	return members, nil
}

// Expire 设置过期时间
func (m *MemoryCacheAdapter) Expire(ctx context.Context, key string, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.data[key]
	if !exists {
		return nil
	}

	entry.expiration = time.Now().Add(expiration)
	m.data[key] = entry

	return nil
}

// cleanupExpired 清理过期键的后台任务
func (m *MemoryCacheAdapter) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for key, entry := range m.data {
			if entry.expiration.Before(now) {
				delete(m.data, key)
			}
		}
		m.mu.Unlock()
	}
}
