package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthType 认证类型
type AuthType string

const (
	AuthTypeJWT    AuthType = "jwt"     // JWT认证
	AuthTypeOAuth2 AuthType = "oauth2"  // OAuth2.0认证
	AuthTypeAPIKey AuthType = "api_key" // API密钥认证
)

// TokenType Token类型
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"  // 访问令牌
	TokenTypeRefresh TokenType = "refresh" // 刷新令牌
)

// UserRole 用户角色
type UserRole string

const (
	RoleSuperAdmin UserRole = "super_admin" // 超级管理员
	RoleAdmin      UserRole = "admin"       // 管理员
	RoleUser       UserRole = "user"        // 普通用户
	RoleGuest      UserRole = "guest"       // 访客
)

// Permission 权限定义
type Permission string

const (
	// 用户权限
	PermissionUserCreate Permission = "user:create"
	PermissionUserRead   Permission = "user:read"
	PermissionUserUpdate Permission = "user:update"
	PermissionUserDelete Permission = "user:delete"

	// 文章权限
	PermissionArticleCreate Permission = "article:create"
	PermissionArticleRead   Permission = "article:read"
	PermissionArticleUpdate Permission = "article:update"
	PermissionArticleDelete Permission = "article:delete"

	// 系统权限
	PermissionSystemConfig Permission = "system:config"
	PermissionSystemAdmin  Permission = "system:admin"
)

// AuthConfig 认证配置
type AuthConfig struct {
	Type          AuthType `mapstructure:"type"`           // 认证类型
	JWTSecret     string   `mapstructure:"jwt_secret"`     // JWT密钥
	JWTExpire     int      `mapstructure:"jwt_expire"`     // JWT过期时间(分钟)
	RefreshExpire int      `mapstructure:"refresh_expire"` // 刷新令牌过期时间(天)
	Issuer        string   `mapstructure:"issuer"`         // 签发者
	Audience      string   `mapstructure:"audience"`       // 受众
	Enable2FA     bool     `mapstructure:"enable_2fa"`     // 是否启用双因子认证
	EnableRBAC    bool     `mapstructure:"enable_rbac"`    // 是否启用RBAC
	TokenHeader   string   `mapstructure:"token_header"`   // Token请求头
	TokenPrefix   string   `mapstructure:"token_prefix"`   // Token前缀
}

// UserClaims 用户声明
type UserClaims struct {
	UserID        uint64                 `json:"user_id"`
	Username      string                 `json:"username"`
	Email         string                 `json:"email"`
	Roles         []UserRole             `json:"roles"`
	Permissions   []Permission           `json:"permissions"`
	IsActive      bool                   `json:"is_active"`
	TwoFactorAuth bool                   `json:"two_factor_auth"`
	CustomClaims  map[string]interface{} `json:"custom_claims,omitempty"`
	jwt.RegisteredClaims
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// AuthResult 认证结果
type AuthResult struct {
	UserID      uint64                 `json:"user_id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	Roles       []UserRole             `json:"roles"`
	Permissions []Permission           `json:"permissions"`
	TokenPair   *TokenPair             `json:"token_pair,omitempty"`
	Session     *Session               `json:"session,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Session 会话信息
type Session struct {
	ID           string                 `json:"id"`
	UserID       uint64                 `json:"user_id"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	CreatedAt    time.Time              `json:"created_at"`
	LastActiveAt time.Time              `json:"last_active_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TwoFactorAuth 双因子认证
type TwoFactorAuth struct {
	Enabled     bool     `json:"enabled"`
	Method      string   `json:"method"` // totp, sms, email
	Secret      string   `json:"secret,omitempty"`
	BackupCodes []string `json:"backup_codes,omitempty"`
}

// OAuth2Provider OAuth2.0提供商
type OAuth2Provider string

const (
	OAuth2Google    OAuth2Provider = "google"
	OAuth2GitHub    OAuth2Provider = "github"
	OAuth2Facebook  OAuth2Provider = "facebook"
	OAuth2Microsoft OAuth2Provider = "microsoft"
	OAuth2Apple     OAuth2Provider = "apple"
)

// OAuth2Config OAuth2.0配置
type OAuth2Config struct {
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	RedirectURL  string   `mapstructure:"redirect_url"`
	Scopes       []string `mapstructure:"scopes"`
}

// RBACConfig RBAC配置
type RBACConfig struct {
	Roles map[UserRole][]Permission `mapstructure:"roles"`
}

// AuthManager 认证管理器
type AuthManager struct {
	config       *AuthConfig                      // 认证配置
	oauth2Config map[OAuth2Provider]*OAuth2Config // OAuth2配置
	rbacConfig   *RBACConfig                      // RBAC配置
	tokenStore   TokenStore                       // 令牌存储
	sessionStore SessionStore                     // 会话存储
	userProvider UserProvider                     // 用户提供者
	logger       *logger.Logger                   // 日志
	mu           sync.RWMutex                     // 读写锁
}

// TokenStore 令牌存储接口
type TokenStore interface {
	SaveToken(ctx context.Context, userID uint64, tokenType TokenType, token string, expiresIn time.Duration) error
	GetToken(ctx context.Context, userID uint64, tokenType TokenType) (string, error)
	DeleteToken(ctx context.Context, userID uint64, tokenType TokenType) error
	DeleteAllTokens(ctx context.Context, userID uint64) error
	IsTokenRevoked(ctx context.Context, token string) (bool, error)
	RevokeToken(ctx context.Context, token string, reason string) error
}

// SessionStore 会话存储接口
type SessionStore interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error
	DeleteSession(ctx context.Context, sessionID string) error
	DeleteUserSessions(ctx context.Context, userID uint64) error
	ListUserSessions(ctx context.Context, userID uint64) ([]*Session, error)
}

// UserProvider 用户提供者接口
type UserProvider interface {
	GetUserByID(ctx context.Context, userID uint64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	VerifyCredentials(ctx context.Context, username, password string) (*User, error)
	UpdateLastLogin(ctx context.Context, userID uint64, ipAddress string) error
	GetUserPermissions(ctx context.Context, userID uint64) ([]Permission, error)
}

// User 用户结构
type User struct {
	ID            uint64                 `json:"id"`
	Username      string                 `json:"username"`
	Email         string                 `json:"email"`
	PasswordHash  string                 `json:"-"` // 不序列化到JSON
	Roles         []UserRole             `json:"roles"`
	IsActive      bool                   `json:"is_active"`
	IsVerified    bool                   `json:"is_verified"`
	TwoFactorAuth *TwoFactorAuth         `json:"two_factor_auth,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	LastLoginAt   *time.Time             `json:"last_login_at,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewAuthManager 创建认证管理器
func NewAuthManager(config *AuthConfig, userProvider UserProvider) (*AuthManager, error) {
	if config.JWTSecret == "" || len(config.JWTSecret) < 32 {
		return nil, errors.New(errors.CodeValidationFailed,
			"JWT secret must be at least 32 characters")
	}

	if config.JWTExpire <= 0 {
		config.JWTExpire = 60 // 默认60分钟
	}

	if config.RefreshExpire <= 0 {
		config.RefreshExpire = 7 // 默认7天
	}

	if config.TokenHeader == "" {
		config.TokenHeader = "Authorization"
	}

	if config.TokenPrefix == "" {
		config.TokenPrefix = "Bearer"
	}

	manager := &AuthManager{
		config:       config,
		oauth2Config: make(map[OAuth2Provider]*OAuth2Config),
		userProvider: userProvider,
		logger:       logger.Logger,
	}

	// 初始化默认RBAC配置
	manager.initRBAC()

	// 初始化令牌存储（使用Redis）
	manager.tokenStore = NewRedisTokenStore()

	// 初始化会话存储
	manager.sessionStore = NewRedisSessionStore()

	return manager, nil
}

// initRBAC 初始化RBAC配置
func (am *AuthManager) initRBAC() {
	am.rbacConfig = &RBACConfig{
		Roles: map[UserRole][]Permission{
			RoleSuperAdmin: {
				PermissionUserCreate, PermissionUserRead, PermissionUserUpdate, PermissionUserDelete,
				PermissionArticleCreate, PermissionArticleRead, PermissionArticleUpdate, PermissionArticleDelete,
				PermissionSystemConfig, PermissionSystemAdmin,
			},
			RoleAdmin: {
				PermissionUserCreate, PermissionUserRead, PermissionUserUpdate,
				PermissionArticleCreate, PermissionArticleRead, PermissionArticleUpdate, PermissionArticleDelete,
				PermissionSystemConfig,
			},
			RoleUser: {
				PermissionUserRead, PermissionUserUpdate,
				PermissionArticleCreate, PermissionArticleRead, PermissionArticleUpdate, PermissionArticleDelete,
			},
			RoleGuest: {
				PermissionArticleRead,
			},
		},
	}
}

// SetOAuth2Config 设置OAuth2配置
func (am *AuthManager) SetOAuth2Config(provider OAuth2Provider, config *OAuth2Config) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.oauth2Config[provider] = config
}

// SetRBACConfig 设置RBAC配置
func (am *AuthManager) SetRBACConfig(config *RBACConfig) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.rbacConfig = config
}

// ==================== 密码相关方法 ====================

// HashPassword 哈希密码
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to hash password")
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func VerifyPassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return errors.New(errors.CodeUnauthorized, "Invalid password")
	}
	return nil
}

// GenerateRandomPassword 生成随机密码
func GenerateRandomPassword(length int) (string, error) {
	if length < 8 {
		length = 8
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate random password")
	}

	// 转换为base64并取前length个字符
	encoded := base64.URLEncoding.EncodeToString(bytes)
	if len(encoded) > length {
		encoded = encoded[:length]
	}

	return encoded, nil
}

// GenerateAPIKey 生成API密钥
func GenerateAPIKey(prefix string) (string, error) {
	// 生成32字节的随机数据
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate API key")
	}

	// 转换为base64
	key := base64.URLEncoding.EncodeToString(bytes)

	// 添加前缀
	if prefix != "" {
		key = prefix + "_" + key
	}

	return key, nil
}

// ==================== JWT相关方法 ====================

// generateJWTToken 生成JWT令牌
func (am *AuthManager) generateJWTToken(claims *UserClaims, tokenType TokenType) (string, error) {
	// 设置令牌过期时间
	var expireTime time.Duration
	switch tokenType {
	case TokenTypeAccess:
		expireTime = time.Duration(am.config.JWTExpire) * time.Minute
	case TokenTypeRefresh:
		expireTime = time.Duration(am.config.RefreshExpire) * 24 * time.Hour
	default:
		expireTime = time.Duration(am.config.JWTExpire) * time.Minute
	}

	// 设置声明
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    am.config.Issuer,
		Subject:   strconv.FormatUint(claims.UserID, 10),
		Audience:  jwt.ClaimStrings{am.config.Audience},
		ExpiresAt: jwt.NewNumericDate(now.Add(expireTime)),
		NotBefore: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        generateTokenID(),
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名令牌
	signedToken, err := token.SignedString([]byte(am.config.JWTSecret))
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to sign JWT token")
	}

	return signedToken, nil
}

// parseJWTToken 解析JWT令牌
func (am *AuthManager) parseJWTToken(tokenString string) (*UserClaims, error) {
	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(am.config.JWTSecret), nil
	})

	if err != nil {
		return nil, errors.Wrap(err, errors.CodeUnauthorized, "Failed to parse JWT token")
	}

	// 验证令牌
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New(errors.CodeUnauthorized, "Invalid JWT token")
}

// GenerateTokenPair 生成令牌对
func (am *AuthManager) GenerateTokenPair(user *User) (*TokenPair, error) {
	// 获取用户权限
	permissions, err := am.userProvider.GetUserPermissions(context.Background(), user.ID)
	if err != nil {
		permissions = am.getDefaultPermissions(user.Roles)
	}

	// 创建访问令牌声明
	accessClaims := &UserClaims{
		UserID:        user.ID,
		Username:      user.Username,
		Email:         user.Email,
		Roles:         user.Roles,
		Permissions:   permissions,
		IsActive:      user.IsActive,
		TwoFactorAuth: user.TwoFactorAuth != nil && user.TwoFactorAuth.Enabled,
	}

	// 创建刷新令牌声明
	refreshClaims := &UserClaims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
	}

	// 生成令牌
	accessToken, err := am.generateJWTToken(accessClaims, TokenTypeAccess)
	if err != nil {
		return nil, err
	}

	refreshToken, err := am.generateJWTToken(refreshClaims, TokenTypeRefresh)
	if err != nil {
		return nil, err
	}

	// 存储刷新令牌
	ctx := context.Background()
	expireTime := time.Duration(am.config.RefreshExpire) * 24 * time.Hour
	if err := am.tokenStore.SaveToken(ctx, user.ID, TokenTypeRefresh, refreshToken, expireTime); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to save refresh token")
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(am.config.JWTExpire * 60), // 转换为秒
	}, nil
}

// getDefaultPermissions 获取默认权限
func (am *AuthManager) getDefaultPermissions(roles []UserRole) []Permission {
	var permissions []Permission
	seen := make(map[Permission]bool)

	for _, role := range roles {
		if rolePerms, ok := am.rbacConfig.Roles[role]; ok {
			for _, perm := range rolePerms {
				if !seen[perm] {
					permissions = append(permissions, perm)
					seen[perm] = true
				}
			}
		}
	}

	return permissions
}

// ==================== 认证方法 ====================

// Authenticate 用户名密码认证
func (am *AuthManager) Authenticate(ctx context.Context, username, password string, ipAddress, userAgent string) (*AuthResult, error) {
	// 验证凭证
	user, err := am.userProvider.VerifyCredentials(ctx, username, password)
	if err != nil {
		return nil, err
	}

	// 检查用户状态
	if !user.IsActive {
		return nil, errors.New(errors.CodeForbidden, "User account is disabled")
	}

	// 检查是否需要双因子认证
	if user.TwoFactorAuth != nil && user.TwoFactorAuth.Enabled {
		// 返回需要双因子认证的结果
		return &AuthResult{
			UserID:   user.ID,
			Username: user.Username,
			Email:    user.Email,
			Metadata: map[string]interface{}{
				"requires_2fa": true,
				"2fa_method":   user.TwoFactorAuth.Method,
			},
		}, nil
	}

	// 生成令牌对
	tokenPair, err := am.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// 创建会话
	session, err := am.createSession(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		am.logger.Warn("Failed to create session", logger.ErrorField(err))
		// 不因为会话创建失败而返回错误
	}

	// 更新最后登录时间
	if err := am.userProvider.UpdateLastLogin(ctx, user.ID, ipAddress); err != nil {
		am.logger.Warn("Failed to update last login", logger.ErrorField(err))
	}

	return &AuthResult{
		UserID:      user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Roles:       user.Roles,
		Permissions: am.getDefaultPermissions(user.Roles),
		TokenPair:   tokenPair,
		Session:     session,
	}, nil
}

// AuthenticateWith2FA 双因子认证
func (am *AuthManager) AuthenticateWith2FA(ctx context.Context, username, password, code string, ipAddress, userAgent string) (*AuthResult, error) {
	// 先进行基础认证
	authResult, err := am.Authenticate(ctx, username, password, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	// 检查是否需要双因子认证
	if authResult.Metadata == nil || !authResult.Metadata["requires_2fa"].(bool) {
		return authResult, nil
	}

	// 获取用户
	user, err := am.userProvider.GetUserByID(ctx, authResult.UserID)
	if err != nil {
		return nil, err
	}

	// 验证双因子认证码
	if err := am.verify2FACode(user, code); err != nil {
		return nil, err
	}

	// 生成完整的令牌对
	tokenPair, err := am.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// 创建会话
	session, err := am.createSession(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		am.logger.Warn("Failed to create session", logger.ErrorField(err))
	}

	authResult.TokenPair = tokenPair
	authResult.Session = session
	authResult.Metadata = nil // 清除双因子认证元数据

	return authResult, nil
}

// verify2FACode 验证双因子认证码
func (am *AuthManager) verify2FACode(user *User, code string) error {
	if user.TwoFactorAuth == nil || !user.TwoFactorAuth.Enabled {
		return errors.New(errors.CodeValidationFailed, "Two-factor authentication is not enabled")
	}

	// TODO: 实现具体的双因子认证码验证逻辑
	// 这里只是一个示例，实际需要根据不同的双因子认证方法实现
	switch user.TwoFactorAuth.Method {
	case "totp":
		// 验证TOTP代码
		return am.verifyTOTPCode(user.TwoFactorAuth.Secret, code)
	case "sms":
		// 验证短信验证码
		return am.verifySMSCode(user.ID, code)
	case "email":
		// 验证邮箱验证码
		return am.verifyEmailCode(user.Email, code)
	default:
		return errors.New(errors.CodeValidationFailed, "Unsupported two-factor authentication method")
	}
}

// verifyTOTPCode 验证TOTP代码（示例）
func (am *AuthManager) verifyTOTPCode(secret, code string) error {
	// TODO: 实现TOTP验证逻辑
	// 这里应该使用如 github.com/pquerna/otp 等库
	return nil
}

// verifySMSCode 验证短信验证码（示例）
func (am *AuthManager) verifySMSCode(userID uint64, code string) error {
	// TODO: 实现短信验证码验证逻辑
	return nil
}

// verifyEmailCode 验证邮箱验证码（示例）
func (am *AuthManager) verifyEmailCode(email, code string) error {
	// TODO: 实现邮箱验证码验证逻辑
	return nil
}

// ==================== 令牌验证和刷新 ====================

// ValidateToken 验证令牌
func (am *AuthManager) ValidateToken(ctx context.Context, tokenString string) (*UserClaims, error) {
	// 解析令牌
	claims, err := am.parseJWTToken(tokenString)
	if err != nil {
		return nil, err
	}

	// 检查令牌是否被吊销
	revoked, err := am.tokenStore.IsTokenRevoked(ctx, tokenString)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to check token revocation")
	}

	if revoked {
		return nil, errors.New(errors.CodeUnauthorized, "Token has been revoked")
	}

	// 检查用户状态
	user, err := am.userProvider.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeUnauthorized, "User not found")
	}

	if !user.IsActive {
		return nil, errors.New(errors.CodeForbidden, "User account is disabled")
	}

	return claims, nil
}

// RefreshToken 刷新令牌
func (am *AuthManager) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// 验证刷新令牌
	claims, err := am.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	// 验证令牌类型
	if claims.TokenType != string(TokenTypeRefresh) && claims.TokenType != "" {
		return nil, errors.New(errors.CodeUnauthorized, "Invalid token type for refresh")
	}

	// 从存储中验证刷新令牌
	storedToken, err := am.tokenStore.GetToken(ctx, claims.UserID, TokenTypeRefresh)
	if err != nil {
		return nil, errors.New(errors.CodeUnauthorized, "Refresh token not found or expired")
	}

	if storedToken != refreshToken {
		return nil, errors.New(errors.CodeUnauthorized, "Invalid refresh token")
	}

	// 获取用户
	user, err := am.userProvider.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeUnauthorized, "User not found")
	}

	// 生成新的令牌对
	newTokenPair, err := am.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// 删除旧的刷新令牌
	if err := am.tokenStore.DeleteToken(ctx, claims.UserID, TokenTypeRefresh); err != nil {
		am.logger.Warn("Failed to delete old refresh token", logger.ErrorField(err))
	}

	return newTokenPair, nil
}

// RevokeToken 吊销令牌
func (am *AuthManager) RevokeToken(ctx context.Context, token string, reason string) error {
	// 解析令牌以获取用户ID
	claims, err := am.parseJWTToken(token)
	if err != nil {
		return err
	}

	// 删除用户的刷新令牌
	if err := am.tokenStore.DeleteToken(ctx, claims.UserID, TokenTypeRefresh); err != nil {
		am.logger.Warn("Failed to delete refresh token", logger.ErrorField(err))
	}

	// 将令牌标记为已吊销
	if err := am.tokenStore.RevokeToken(ctx, token, reason); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to revoke token")
	}

	return nil
}

// RevokeAllUserTokens 吊销用户所有令牌
func (am *AuthManager) RevokeAllUserTokens(ctx context.Context, userID uint64) error {
	// 删除所有令牌
	if err := am.tokenStore.DeleteAllTokens(ctx, userID); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to delete user tokens")
	}

	// 删除所有会话
	if err := am.sessionStore.DeleteUserSessions(ctx, userID); err != nil {
		am.logger.Warn("Failed to delete user sessions", logger.ErrorField(err))
	}

	return nil
}

// ==================== 会话管理 ====================

// createSession 创建会话
func (am *AuthManager) createSession(ctx context.Context, userID uint64, ipAddress, userAgent string) (*Session, error) {
	session := &Session{
		ID:           generateSessionID(),
		UserID:       userID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(am.config.RefreshExpire) * 24 * time.Hour),
		Metadata: map[string]interface{}{
			"created_by": "auth_manager",
		},
	}

	if err := am.sessionStore.CreateSession(ctx, session); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to create session")
	}

	return session, nil
}

// UpdateSessionActivity 更新会话活跃时间
func (am *AuthManager) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	updates := map[string]interface{}{
		"last_active_at": time.Now(),
	}

	if err := am.sessionStore.UpdateSession(ctx, sessionID, updates); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to update session activity")
	}

	return nil
}

// GetUserSessions 获取用户会话列表
func (am *AuthManager) GetUserSessions(ctx context.Context, userID uint64) ([]*Session, error) {
	sessions, err := am.sessionStore.ListUserSessions(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to get user sessions")
	}

	return sessions, nil
}

// TerminateSession 终止会话
func (am *AuthManager) TerminateSession(ctx context.Context, sessionID string) error {
	if err := am.sessionStore.DeleteSession(ctx, sessionID); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to terminate session")
	}

	return nil
}

// ==================== 权限检查 ====================

// HasPermission 检查用户是否有权限
func (am *AuthManager) HasPermission(claims *UserClaims, permission Permission) bool {
	if claims == nil {
		return false
	}

	// 超级管理员拥有所有权限
	for _, role := range claims.Roles {
		if role == RoleSuperAdmin {
			return true
		}
	}

	// 检查权限列表
	for _, perm := range claims.Permissions {
		if perm == permission {
			return true
		}
	}

	return false
}

// HasAnyPermission 检查用户是否有任意权限
func (am *AuthManager) HasAnyPermission(claims *UserClaims, permissions []Permission) bool {
	for _, permission := range permissions {
		if am.HasPermission(claims, permission) {
			return true
		}
	}
	return false
}

// HasAllPermissions 检查用户是否有所有权限
func (am *AuthManager) HasAllPermissions(claims *UserClaims, permissions []Permission) bool {
	for _, permission := range permissions {
		if !am.HasPermission(claims, permission) {
			return false
		}
	}
	return true
}

// HasRole 检查用户是否有角色
func (am *AuthManager) HasRole(claims *UserClaims, role UserRole) bool {
	if claims == nil {
		return false
	}

	for _, r := range claims.Roles {
		if r == role {
			return true
		}
	}

	return false
}

// HasAnyRole 检查用户是否有任意角色
func (am *AuthManager) HasAnyRole(claims *UserClaims, roles []UserRole) bool {
	for _, role := range roles {
		if am.HasRole(claims, role) {
			return true
		}
	}
	return false
}

// GetUserPermissions 获取用户权限列表
func (am *AuthManager) GetUserPermissions(ctx context.Context, userID uint64) ([]Permission, error) {
	return am.userProvider.GetUserPermissions(ctx, userID)
}

// ==================== OAuth2.0集成 ====================

// GetOAuth2AuthURL 获取OAuth2认证URL
func (am *AuthManager) GetOAuth2AuthURL(ctx context.Context, provider OAuth2Provider, state string) (string, error) {
	config, ok := am.oauth2Config[provider]
	if !ok {
		return "", errors.New(errors.CodeInternalError,
			fmt.Sprintf("OAuth2 provider %s not configured", provider))
	}

	// 根据不同的提供商生成认证URL
	switch provider {
	case OAuth2Google:
		return am.getGoogleAuthURL(config, state)
	case OAuth2GitHub:
		return am.getGitHubAuthURL(config, state)
	case OAuth2Facebook:
		return am.getFacebookAuthURL(config, state)
	case OAuth2Microsoft:
		return am.getMicrosoftAuthURL(config, state)
	case OAuth2Apple:
		return am.getAppleAuthURL(config, state)
	default:
		return "", errors.New(errors.CodeInternalError,
			fmt.Sprintf("Unsupported OAuth2 provider: %s", provider))
	}
}

// HandleOAuth2Callback 处理OAuth2回调
func (am *AuthManager) HandleOAuth2Callback(ctx context.Context, provider OAuth2Provider, code, state string) (*AuthResult, error) {
	config, ok := am.oauth2Config[provider]
	if !ok {
		return nil, errors.New(errors.CodeInternalError,
			fmt.Sprintf("OAuth2 provider %s not configured", provider))
	}

	// 交换令牌
	accessToken, err := am.exchangeOAuth2Code(provider, config, code)
	if err != nil {
		return nil, err
	}

	// 获取用户信息
	userInfo, err := am.getOAuth2UserInfo(provider, accessToken)
	if err != nil {
		return nil, err
	}

	// 查找或创建用户
	user, err := am.findOrCreateOAuth2User(ctx, provider, userInfo)
	if err != nil {
		return nil, err
	}

	// 生成令牌对
	tokenPair, err := am.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		UserID:      user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Roles:       user.Roles,
		Permissions: am.getDefaultPermissions(user.Roles),
		TokenPair:   tokenPair,
	}, nil
}

// 以下为OAuth2具体实现（示例）
func (am *AuthManager) getGoogleAuthURL(config *OAuth2Config, state string) (string, error) {
	// TODO: 实现Google OAuth2认证URL生成
	return "", nil
}

func (am *AuthManager) exchangeOAuth2Code(provider OAuth2Provider, config *OAuth2Config, code string) (string, error) {
	// TODO: 实现OAuth2代码交换
	return "", nil
}

func (am *AuthManager) getOAuth2UserInfo(provider OAuth2Provider, accessToken string) (map[string]interface{}, error) {
	// TODO: 实现获取OAuth2用户信息
	return map[string]interface{}{}, nil
}

func (am *AuthManager) findOrCreateOAuth2User(ctx context.Context, provider OAuth2Provider, userInfo map[string]interface{}) (*User, error) {
	// TODO: 实现查找或创建OAuth2用户
	return &User{}, nil
}

// ==================== 辅助函数 ====================

// generateTokenID 生成令牌ID
func generateTokenID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", bytes)
}

// generateSessionID 生成会话ID
func generateSessionID() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	hash := sha256.Sum256(bytes)
	return fmt.Sprintf("sess_%x", hash[:16])
}

// ExtractTokenFromHeader 从请求头提取令牌
func (am *AuthManager) ExtractTokenFromHeader(c *gin.Context) (string, error) {
	authHeader := c.GetHeader(am.config.TokenHeader)
	if authHeader == "" {
		return "", errors.New(errors.CodeUnauthorized, "Authorization header is required")
	}

	// 检查令牌前缀
	prefix := am.config.TokenPrefix + " "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", errors.New(errors.CodeUnauthorized,
			fmt.Sprintf("Invalid token format, expected '%s' prefix", am.config.TokenPrefix))
	}

	token := strings.TrimPrefix(authHeader, prefix)
	if token == "" {
		return "", errors.New(errors.CodeUnauthorized, "Token is empty")
	}

	return token, nil
}

// GetUserFromContext 从上下文中获取用户信息
func GetUserFromContext(c *gin.Context) (*UserClaims, error) {
	claims, exists := c.Get("user_claims")
	if !exists {
		return nil, errors.New(errors.CodeUnauthorized, "User claims not found in context")
	}

	userClaims, ok := claims.(*UserClaims)
	if !ok {
		return nil, errors.New(errors.CodeInternalError, "Invalid user claims type")
	}

	return userClaims, nil
}

// SetUserToContext 设置用户信息到上下文
func SetUserToContext(c *gin.Context, claims *UserClaims) {
	c.Set("user_claims", claims)
	c.Set("user_id", claims.UserID)
	c.Set("username", claims.Username)
	c.Set("email", claims.Email)
	c.Set("roles", claims.Roles)
	c.Set("permissions", claims.Permissions)
}

// ==================== Gin中间件 ====================

// AuthMiddleware 认证中间件
func (am *AuthManager) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提取令牌
		token, err := am.ExtractTokenFromHeader(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 验证令牌
		claims, err := am.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 设置用户信息到上下文
		SetUserToContext(c, claims)

		// 更新会话活跃时间（如果有会话ID）
		if sessionID, exists := c.Get("session_id"); exists {
			if sid, ok := sessionID.(string); ok {
				_ = am.UpdateSessionActivity(c.Request.Context(), sid)
			}
		}

		c.Next()
	}
}

// RBACMiddleware RBAC权限中间件
func (am *AuthManager) RBACMiddleware(permission Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户声明
		claims, err := GetUserFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 检查权限
		if !am.HasPermission(claims, permission) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RoleMiddleware 角色中间件
func (am *AuthManager) RoleMiddleware(roles []UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户声明
		claims, err := GetUserFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 检查角色
		if !am.HasAnyRole(claims, roles) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient role privileges",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// TwoFactorAuthMiddleware 双因子认证中间件
func (am *AuthManager) TwoFactorAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户声明
		claims, err := GetUserFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 检查是否启用了双因子认证
		if claims.TwoFactorAuth {
			// 检查会话中是否有双因子认证标记
			if verified, exists := c.Get("2fa_verified"); !exists || !verified.(bool) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Two-factor authentication required",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// ==================== 配置验证 ====================

// ValidateConfig 验证配置
func (am *AuthManager) ValidateConfig() error {
	if am.config.JWTSecret == "" {
		return errors.New(errors.CodeValidationFailed, "JWT secret is required")
	}

	if len(am.config.JWTSecret) < 32 {
		return errors.New(errors.CodeValidationFailed, "JWT secret must be at least 32 characters")
	}

	if am.config.JWTExpire <= 0 {
		return errors.New(errors.CodeValidationFailed, "JWT expire time must be positive")
	}

	if am.config.RefreshExpire <= 0 {
		return errors.New(errors.CodeValidationFailed, "Refresh expire time must be positive")
	}

	if am.config.Issuer == "" {
		am.config.Issuer = "dgou-app"
	}

	if am.config.Audience == "" {
		am.config.Audience = "dgou-client"
	}

	return nil
}

// GetConfig 获取配置
func (am *AuthManager) GetConfig() *AuthConfig {
	return am.config
}

// Close 关闭认证管理器
func (am *AuthManager) Close() error {
	// 清理资源
	// 这里可以关闭数据库连接等
	return nil
}
