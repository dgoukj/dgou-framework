package auth

import (
	"context"
	"dgou/pkg/errors"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

// APIKeyManager API密钥管理器
type APIKeyManager struct {
	keyStore APIKeyStore
	prefix   string
}

// APIKeyStore API密钥存储接口
type APIKeyStore interface {
	SaveAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKey(ctx context.Context, keyID string) (*APIKey, error)
	DeleteAPIKey(ctx context.Context, keyID string) error
	ListUserAPIKeys(ctx context.Context, userID uint64) ([]*APIKey, error)
	UpdateAPIKeyUsage(ctx context.Context, keyID string, lastUsed time.Time) error
}

// APIKey API密钥结构
type APIKey struct {
	ID          string                 `json:"id"`
	UserID      uint64                 `json:"user_id"`
	Name        string                 `json:"name"`
	KeyHash     string                 `json:"-"` // 不序列化
	Prefix      string                 `json:"prefix"`
	LastUsedAt  *time.Time             `json:"last_used_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	RevokedAt   *time.Time             `json:"revoked_at,omitempty"`
	Permissions []Permission           `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewAPIKeyManager 创建API密钥管理器
func NewAPIKeyManager(store APIKeyStore) *APIKeyManager {
	return &APIKeyManager{
		keyStore: store,
		prefix:   "api_key",
	}
}

// GenerateAPIKey 生成API密钥
func (akm *APIKeyManager) GenerateAPIKey(ctx context.Context, userID uint64, name string, permissions []Permission, expiresIn *time.Duration) (*APIKey, string, error) {
	// 生成密钥ID和密钥
	keyID, err := generateAPIKeyID()
	if err != nil {
		return nil, "", err
	}

	secret, err := generateAPISecret()
	if err != nil {
		return nil, "", err
	}

	// 计算哈希
	keyHash, err := hashAPIKey(secret)
	if err != nil {
		return nil, "", err
	}

	// 创建API密钥对象
	apiKey := &APIKey{
		ID:          keyID,
		UserID:      userID,
		Name:        name,
		KeyHash:     keyHash,
		Prefix:      secret[:8], // 使用前8个字符作为前缀
		CreatedAt:   time.Now(),
		Permissions: permissions,
		Metadata: map[string]interface{}{
			"generated_by": "api_key_manager",
		},
	}

	// 设置过期时间
	if expiresIn != nil {
		expiresAt := time.Now().Add(*expiresIn)
		apiKey.ExpiresAt = &expiresAt
	}

	// 存储API密钥
	if err := akm.keyStore.SaveAPIKey(ctx, apiKey); err != nil {
		return nil, "", errors.Wrap(err, errors.CodeInternalError, "Failed to save API key")
	}

	// 返回完整密钥（格式：keyID.secret）
	fullKey := fmt.Sprintf("%s.%s", keyID, secret)

	return apiKey, fullKey, nil
}

// ValidateAPIKey 验证API密钥
func (akm *APIKeyManager) ValidateAPIKey(ctx context.Context, apiKey string) (*APIKey, error) {
	// 解析API密钥
	keyID, secret, err := parseAPIKey(apiKey)
	if err != nil {
		return nil, err
	}

	// 从存储中获取API密钥
	storedKey, err := akm.keyStore.GetAPIKey(ctx, keyID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeUnauthorized, "Invalid API key")
	}

	// 检查是否已吊销
	if storedKey.RevokedAt != nil {
		return nil, errors.New(errors.CodeUnauthorized, "API key has been revoked")
	}

	// 检查是否已过期
	if storedKey.ExpiresAt != nil && storedKey.ExpiresAt.Before(time.Now()) {
		return nil, errors.New(errors.CodeUnauthorized, "API key has expired")
	}

	// 验证密钥
	if !verifyAPIKey(secret, storedKey.KeyHash) {
		return nil, errors.New(errors.CodeUnauthorized, "Invalid API key")
	}

	// 更新最后使用时间
	now := time.Now()
	if err := akm.keyStore.UpdateAPIKeyUsage(ctx, keyID, now); err != nil {
		// 不因为更新失败而返回错误
		fmt.Printf("Failed to update API key usage: %v\n", err)
	}

	return storedKey, nil
}

// RevokeAPIKey 吊销API密钥
func (akm *APIKeyManager) RevokeAPIKey(ctx context.Context, keyID string) error {
	// 获取API密钥
	apiKey, err := akm.keyStore.GetAPIKey(ctx, keyID)
	if err != nil {
		return errors.Wrap(err, errors.CodeResourceNotFound, "API key not found")
	}

	// 标记为已吊销
	now := time.Now()
	apiKey.RevokedAt = &now

	// 更新存储
	if err := akm.keyStore.SaveAPIKey(ctx, apiKey); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to revoke API key")
	}

	return nil
}

// ListUserAPIKeys 列出用户API密钥
func (akm *APIKeyManager) ListUserAPIKeys(ctx context.Context, userID uint64) ([]*APIKey, error) {
	keys, err := akm.keyStore.ListUserAPIKeys(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to list API keys")
	}

	// 清理敏感信息
	for _, key := range keys {
		key.KeyHash = ""
	}

	return keys, nil
}

// APIKeyMiddleware API密钥认证中间件
func (akm *APIKeyManager) APIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取API密钥
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// 尝试从查询参数获取
			apiKey = c.Query("api_key")
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "API key is required",
			})
			c.Abort()
			return
		}

		// 验证API密钥
		keyInfo, err := akm.ValidateAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 设置用户信息到上下文
		claims := &UserClaims{
			UserID:      keyInfo.UserID,
			Permissions: keyInfo.Permissions,
			CustomClaims: map[string]interface{}{
				"api_key_id":   keyInfo.ID,
				"api_key_name": keyInfo.Name,
			},
		}

		SetUserToContext(c, claims)
		c.Set("api_key_info", keyInfo)

		c.Next()
	}
}

// generateAPIKeyID 生成API密钥ID
func generateAPIKeyID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate API key ID")
	}

	return fmt.Sprintf("ak_%x", bytes), nil
}

// generateAPISecret 生成API密钥
func generateAPISecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate API secret")
	}

	// 使用base64 URL编码
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// hashAPIKey 哈希API密钥
func hashAPIKey(apiKey string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to hash API key")
	}

	return string(hash), nil
}

// verifyAPIKey 验证API密钥
func verifyAPIKey(apiKey, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(apiKey))
	return err == nil
}

// parseAPIKey 解析API密钥
func parseAPIKey(fullKey string) (string, string, error) {
	parts := strings.SplitN(fullKey, ".", 2)
	if len(parts) != 2 {
		return "", "", errors.New(errors.CodeValidationFailed, "Invalid API key format")
	}

	return parts[0], parts[1], nil
}
