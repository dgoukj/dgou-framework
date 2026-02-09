package auth

import (
	"context"
	"dgou/pkg/errors"
	"encoding/json"
	"fmt"
	"time"
)

// RedisSessionStore Redis会话存储实现
type RedisSessionStore struct {
	cache  CacheAdapter
	prefix string
}

// NewRedisSessionStore 创建Redis会话存储
func NewRedisSessionStore() *RedisSessionStore {
	return &RedisSessionStore{
		cache:  NewMemoryCacheAdapter(),
		prefix: "auth:session",
	}
}

// buildKey 构建键
func (rss *RedisSessionStore) buildKey(sessionID string) string {
	return fmt.Sprintf("%s:%s", rss.prefix, sessionID)
}

// buildUserSessionsKey 构建用户会话列表键
func (rss *RedisSessionStore) buildUserSessionsKey(userID uint64) string {
	return fmt.Sprintf("%s:user:%d", rss.prefix, userID)
}

// CreateSession 创建会话
func (rss *RedisSessionStore) CreateSession(ctx context.Context, session *Session) error {
	sessionKey := rss.buildKey(session.ID)
	userSessionsKey := rss.buildUserSessionsKey(session.UserID)

	// 序列化会话
	sessionData, err := json.Marshal(session)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal session")
	}

	// 存储会话
	ttl := session.ExpiresAt.Sub(time.Now())
	if ttl <= 0 {
		return errors.New(errors.CodeValidationFailed, "Session has already expired")
	}

	if err := rss.cache.Set(ctx, sessionKey, string(sessionData), ttl); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to save session to Redis")
	}

	// 将会话ID添加到用户的会话集合中
	if err := rss.cache.SAdd(ctx, userSessionsKey, session.ID); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to add session to user sessions")
	}

	// 设置用户会话集合的过期时间
	_ = rss.cache.Expire(ctx, userSessionsKey, ttl)

	return nil
}

// GetSession 获取会话
func (rss *RedisSessionStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	sessionKey := rss.buildKey(sessionID)

	sessionData, err := rss.cache.Get(ctx, sessionKey)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to get session")
	}

	if sessionData == "" {
		return nil, errors.New(errors.CodeResourceNotFound, "Session not found")
	}

	var session Session
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal session")
	}

	return &session, nil
}

// UpdateSession 更新会话
func (rss *RedisSessionStore) UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	sessionKey := rss.buildKey(sessionID)

	// 获取现有会话
	session, err := rss.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// 应用更新
	for key, value := range updates {
		switch key {
		case "last_active_at":
			if t, ok := value.(time.Time); ok {
				session.LastActiveAt = t
			}
		case "expires_at":
			if t, ok := value.(time.Time); ok {
				session.ExpiresAt = t
			}
		case "metadata":
			if meta, ok := value.(map[string]interface{}); ok {
				if session.Metadata == nil {
					session.Metadata = make(map[string]interface{})
				}
				for k, v := range meta {
					session.Metadata[k] = v
				}
			}
		}
	}

	// 保存更新后的会话
	sessionData, err := json.Marshal(session)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal updated session")
	}

	// 更新TTL
	ttl := session.ExpiresAt.Sub(time.Now())
	if ttl <= 0 {
		// 会话已过期，删除它
		return rss.DeleteSession(ctx, sessionID)
	}

	if err := rss.cache.Set(ctx, sessionKey, string(sessionData), ttl); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to update session")
	}

	return nil
}

// isResourceNotFoundError 检查是否为资源未找到错误
func isResourceNotFoundError(err error) bool {
	if customErr, ok := err.(*errors.Error); ok {
		return customErr.Code == errors.CodeResourceNotFound
	}
	// 检查错误消息是否包含特定字符串
	return err != nil && (err.Error() == "Session not found" || err.Error() == "Resource not found")
}

// DeleteSession 删除会话
func (rss *RedisSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	sessionKey := rss.buildKey(sessionID)

	// 先获取会话以获取用户ID
	session, err := rss.GetSession(ctx, sessionID)
	if err != nil && !isResourceNotFoundError(err) {
		// 如果不是"未找到"错误，则返回错误
		return err
	}

	// 删除会话
	if err := rss.cache.Delete(ctx, sessionKey); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to delete session")
	}

	// 从用户会话集合中删除
	if session != nil {
		userSessionsKey := rss.buildUserSessionsKey(session.UserID)
		_ = rss.cache.SRem(ctx, userSessionsKey, sessionID)
	}

	return nil
}

// DeleteUserSessions 删除用户所有会话
func (rss *RedisSessionStore) DeleteUserSessions(ctx context.Context, userID uint64) error {
	userSessionsKey := rss.buildUserSessionsKey(userID)

	// 获取用户的所有会话ID
	sessionIDs, err := rss.cache.SMembers(ctx, userSessionsKey)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to get user sessions")
	}

	// 删除每个会话
	for _, sessionID := range sessionIDs {
		_ = rss.DeleteSession(ctx, sessionID)
	}

	// 删除用户会话集合
	_ = rss.cache.Delete(ctx, userSessionsKey)

	return nil
}

// ListUserSessions 列出用户会话
func (rss *RedisSessionStore) ListUserSessions(ctx context.Context, userID uint64) ([]*Session, error) {
	userSessionsKey := rss.buildUserSessionsKey(userID)

	// 获取用户的所有会话ID
	sessionIDs, err := rss.cache.SMembers(ctx, userSessionsKey)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to get user sessions")
	}

	// 获取每个会话的详细信息
	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, err := rss.GetSession(ctx, sessionID)
		if err == nil {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// CleanupExpiredSessions 清理过期会话
func (rss *RedisSessionStore) CleanupExpiredSessions(ctx context.Context) error {
	// 内存缓存会自动清理过期的键
	return nil
}
