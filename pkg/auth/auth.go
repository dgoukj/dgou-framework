package auth

import (
	"context"
	"github.com/dgoukj/dgou-framework/pkg/cache"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	pkgErrors "github.com/pkg/errors"
)

// Config JWT配置
type Config struct {
	Secret     string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// Manager JWT认证管理器
type Manager struct {
	cache      cache.Cache
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager 创建认证管理器
func NewManager(cache cache.Cache, cfg Config) *Manager {
	return &Manager{
		cache:      cache,
		secret:     []byte(cfg.Secret),
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}
}

// UserClaims JWT自定义声明（包含用户基本信息）
type UserClaims struct {
	UserID   uint64   `json:"uid"`
	Username string   `json:"uname"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// Generate 生成访问令牌和刷新令牌
func (m *Manager) Generate(userID uint64, username string, roles []string) (access, refresh string, err error) {
	now := time.Now()
	// 访问令牌
	accessClaims := UserClaims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatUint(userID, 10),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(m.secret)
	if err != nil {
		return "", "", err
	}
	// 刷新令牌
	refreshClaims := jwt.RegisteredClaims{
		Issuer:    m.issuer,
		Subject:   strconv.FormatUint(userID, 10),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        uuid.New().String(),
	}
	refresh, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(m.secret)
	if err != nil {
		return "", "", err
	}
	// 存储刷新令牌至缓存（用于吊销）
	ctx := context.Background()
	_ = m.cache.Set(ctx, "refresh:"+refreshClaims.ID, userID, m.refreshTTL)
	return access, refresh, nil
}

// Parse 解析并验证访问令牌
func (m *Manager) Parse(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, pkgErrors.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, pkgErrors.Wrap(err, "parse token")
	}
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, pkgErrors.New("invalid token")
}

// Refresh 刷新令牌
func (m *Manager) Refresh(refreshToken string) (newAccess, newRefresh string, err error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(refreshToken, claims, func(t *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return "", "", pkgErrors.Wrap(err, "invalid refresh token")
	}
	// 检查是否吊销
	ctx := context.Background()
	val, err := m.cache.Get(ctx, "refresh:"+claims.ID)
	if err != nil || val == "" {
		return "", "", pkgErrors.New("refresh token revoked or expired")
	}
	userID, _ := strconv.ParseUint(val, 10, 64)
	// ⚠️ 此处应由业务层重新查询用户信息，示例中仅用空用户名和角色
	return m.Generate(userID, "", nil)
}

// Revoke 吊销刷新令牌
func (m *Manager) Revoke(tokenID string) error {
	return m.cache.Delete(context.Background(), "refresh:"+tokenID)
}
