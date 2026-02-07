package auth

import (
	"dgou/pkg/config"
	"dgou/pkg/logger"
	"sync"
)

var (
	// globalAuth 全局认证管理器
	globalAuth *AuthManager
	// authOnce 确保单例初始化
	authOnce sync.Once
)

// InitAuth 初始化认证（单例模式）
func InitAuth(cfg *config.Config, userProvider UserProvider) (*AuthManager, error) {
	var initErr error

	authOnce.Do(func() {
		// 创建认证配置
		authConfig := &AuthConfig{
			Type:          AuthTypeJWT,
			JWTSecret:     cfg.JWT.Secret,
			JWTExpire:     cfg.JWT.Expire,
			RefreshExpire: cfg.JWT.Refresh,
			Issuer:        cfg.JWT.Issuer,
			Audience:      cfg.JWT.Audience,
			Enable2FA:     false,
			EnableRBAC:    true,
			TokenHeader:   "Authorization",
			TokenPrefix:   "Bearer",
		}

		// 创建认证管理器
		authManager, err := NewAuthManager(authConfig, userProvider)
		if err != nil {
			initErr = err
			return
		}

		globalAuth = authManager

		// 验证配置
		if err := authManager.ValidateConfig(); err != nil {
			initErr = err
			return
		}

		logger.Info("Authentication initialized successfully",
			logger.String("type", string(authConfig.Type)),
			logger.Bool("enable_2fa", authConfig.Enable2FA),
			logger.Bool("enable_rbac", authConfig.EnableRBAC),
		)
	})

	return globalAuth, initErr
}

// GetAuth 获取全局认证管理器
func GetAuth() *AuthManager {
	if globalAuth == nil {
		logger.Error("Authentication not initialized, please call InitAuth first")
		return nil
	}
	return globalAuth
}

// CloseAuth 关闭认证管理器
func CloseAuth() error {
	if globalAuth == nil {
		return nil
	}
	return globalAuth.Close()
}

// 快捷方法
var (
	// HashPassword 哈希密码
	HashPassword = HashPassword

	// VerifyPassword 验证密码
	VerifyPassword = VerifyPassword

	// GenerateRandomPassword 生成随机密码
	GenerateRandomPassword = GenerateRandomPassword

	// GenerateAPIKey 生成API密钥
	GenerateAPIKey = GenerateAPIKey
)
