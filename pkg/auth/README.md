# Dgou Framework - 生产级 Go Gin 脚手架
## 8. 认证组件 (pkg/auth)

### 特性
- ✅ JWT v5 安全实现（支持HS256签名）
- ✅ Token刷新机制（双Token方案）
- ✅ 完整的RBAC权限控制系统
- ✅ 双因子认证支持（TOTP、短信、邮箱）
- ✅ OAuth2.0集成（Google、GitHub、Microsoft等）
- ✅ API密钥认证
- ✅ 会话管理
- ✅ 令牌吊销和黑名单
- ✅ 密码哈希和验证

### 快速开始

#### 基本配置

```yaml
# config/config.yaml
auth:
    type: jwt
    jwt_secret: "your-super-secret-jwt-key-at-least-32-characters-long"
    jwt_expire: 60
    refresh_expire: 7
    issuer: "dgou-app"
    audience: "dgou-client"
    enable_2fa: false
    enable_rbac: true
```
#### 用户提供者接口实现
```go

package services

import (
    "context"
    "dgou/pkg/auth"
    "dgou/pkg/database"
)

type UserService struct {
    db *database.DB
}

func (us *UserService) GetUserByID(ctx context.Context, userID uint64) (*auth.User, error) {
    var user auth.User
    if err := us.db.First(&user, userID).Error; err != nil {
        return nil, err
    }
    return &user, nil
}

func (us *UserService) VerifyCredentials(ctx context.Context, username, password string) (*auth.User, error) {
    var user auth.User
    if err := us.db.Where("username = ? OR email = ?", username, username).First(&user).Error; err != nil {
        return nil, auth.ErrInvalidCredentials
    }
    
    if err := auth.VerifyPassword(user.PasswordHash, password); err != nil {
        return nil, err
    }
    
    return &user, nil
}

// 实现其他接口方法...
```
#### 初始化认证
```go

import (
    "dgou/pkg/auth"
    "dgou/pkg/config"
    "your-app/services"
)

func main() {
    cfg := config.LoadConfig()
    
    // 创建用户服务
    userService := &services.UserService{}
    
    // 初始化认证
    authManager, err := auth.InitAuth(cfg, userService)
    if err != nil {
        log.Fatal(err)
    }
    
    // 注册OAuth2提供商
    oauth2Manager := auth.NewOAuth2Manager()
    googleConfig := &auth.OAuth2ProviderConfig{
         ClientID:     cfg.OAuth2.Google.ClientID,
         ClientSecret: cfg.OAuth2.Google.ClientSecret,
         AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
         TokenURL:     "https://oauth2.googleapis.com/token",
         UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
         RedirectURL:  cfg.OAuth2.Google.RedirectURL,
         Scopes:       cfg.OAuth2.Google.Scopes,
    }
    oauth2Manager.RegisterProvider(auth.OAuth2Google, googleConfig)
}
```
#### 认证路由示例
```go

package api

import (
    "dgou/pkg/auth"
    "dgou/pkg/response"
    "net/http"
    
    "github.com/gin-gonic/gin"
)

type AuthController struct {
    authManager *auth.AuthManager
}

func NewAuthController(authManager *auth.AuthManager) *AuthController {
    return &AuthController{
         authManager: authManager,
    }
}

// Login 用户名密码登录
func (ac *AuthController) Login(c *gin.Context) {
    var req struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request")
        return
    }

    ipAddress := c.ClientIP()
    userAgent := c.Request.UserAgent()
    
    authResult, err := ac.authManager.Authenticate(c.Request.Context(), req.Username, req.Password, ipAddress, userAgent)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    // 检查是否需要双因子认证
    if authResult.Metadata != nil && authResult.Metadata["requires_2fa"].(bool) {
        response.Success(c, gin.H{
             "requires_2fa": true,
             "method":       authResult.Metadata["2fa_method"],
             "user_id":      authResult.UserID,
        })
        return
    }
    
    response.Success(c, authResult)
}

// Login2FA 双因子认证登录
func (ac *AuthController) Login2FA(c *gin.Context) {
    var req struct {
        UserID uint64 `json:"user_id" binding:"required"`
        Code   string `json:"code" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request")
        return
    }
    
    ipAddress := c.ClientIP()
    userAgent := c.Request.UserAgent()
    
    // 这里需要根据实际情况获取用户名和密码
    // 实际项目中可能需要修改接口设计
    authResult, err := ac.authManager.AuthenticateWith2FA(c.Request.Context(), "", "", req.Code, ipAddress, userAgent)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    response.Success(c, authResult)
}

// RefreshToken 刷新令牌
func (ac *AuthController) RefreshToken(c *gin.Context) {
    var req struct {
        RefreshToken string `json:"refresh_token" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request")
        return
    }
    
    tokenPair, err := ac.authManager.RefreshToken(c.Request.Context(), req.RefreshToken)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    response.Success(c, tokenPair)
}

// Logout 登出
func (ac *AuthController) Logout(c *gin.Context) {
    // 从上下文中获取用户ID
    userClaims, err := auth.GetUserFromContext(c)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    // 吊销用户所有令牌
    if err := ac.authManager.RevokeAllUserTokens(c.Request.Context(), userClaims.UserID); err != nil {
        response.InternalServerError(c, "Failed to logout")
        return
    }
    
    response.Success(c, gin.H{
         "message": "Logged out successfully",
    })
}

// GetProfile 获取用户资料
func (ac *AuthController) GetProfile(c *gin.Context) {
    userClaims, err := auth.GetUserFromContext(c)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    response.Success(c, gin.H{
         "user_id":    userClaims.UserID,
         "username":   userClaims.Username,
         "email":      userClaims.Email,
         "roles":      userClaims.Roles,
         "permissions": userClaims.Permissions,
    })
}
```
#### 权限控制示例
```go

// 需要认证的路由
func setupAuthRoutes(router *gin.Engine, authManager *auth.AuthManager) {
    authGroup := router.Group("/api")
    
    // 使用认证中间件
    authGroup.Use(authManager.AuthMiddleware())
    
    // 用户相关路由
    userGroup := authGroup.Group("/users")
    {
        // 所有用户都可以访问自己的资料
        userGroup.GET("/profile", userController.GetProfile)
        
        // 需要user:read权限
        userGroup.GET("", userController.ListUsers,
        authManager.RBACMiddleware(auth.PermissionUserRead))
        
        // 需要user:create权限
        userGroup.POST("", userController.CreateUser,
        authManager.RBACMiddleware(auth.PermissionUserCreate))
        
        // 需要user:update权限
        userGroup.PUT("/:id", userController.UpdateUser,
        authManager.RBACMiddleware(auth.PermissionUserUpdate))
    
        // 需要user:delete权限
        userGroup.DELETE("/:id", userController.DeleteUser,
        authManager.RBACMiddleware(auth.PermissionUserDelete))
    }

    // 文章相关路由
    articleGroup := authGroup.Group("/articles")
    {
        // 所有用户都可以查看文章
        articleGroup.GET("/:id", articleController.GetArticle)
        
        // 需要article:create权限
        articleGroup.POST("", articleController.CreateArticle,
        authManager.RBACMiddleware(auth.PermissionArticleCreate))
        
        // 需要article:update权限
        articleGroup.PUT("/:id", articleController.UpdateArticle,
        authManager.RBACMiddleware(auth.PermissionArticleUpdate))
        
        // 需要article:delete权限
        articleGroup.DELETE("/:id", articleController.DeleteArticle,
        authManager.RBACMiddleware(auth.PermissionArticleDelete))
    }

    // 管理员路由
    adminGroup := authGroup.Group("/admin")
    {
        // 需要admin角色
        adminGroup.Use(authManager.RoleMiddleware([]auth.UserRole{auth.RoleAdmin, auth.RoleSuperAdmin}))
        
        adminGroup.GET("/dashboard", adminController.Dashboard)
        adminGroup.GET("/users", adminController.ListAllUsers)
        
        // 需要super_admin角色
        superAdminGroup := adminGroup.Group("/system")
        superAdminGroup.Use(authManager.RoleMiddleware([]auth.UserRole{auth.RoleSuperAdmin}))
        {
            superAdminGroup.GET("/config", adminController.GetSystemConfig)
            superAdminGroup.PUT("/config", adminController.UpdateSystemConfig)
        }
    }
}

// 双因子认证路由
func setup2FARoutes(router *gin.Engine, authManager *auth.AuthManager) {
    authGroup := router.Group("/api/2fa")
    authGroup.Use(authManager.AuthMiddleware())
    
    // 启用双因子认证
    authGroup.POST("/enable", func(c *gin.Context) {
        userClaims, _ := auth.GetUserFromContext(c)
        
        // 生成TOTP密钥
        twoFactorManager := auth.NewTwoFactorManager("dgou-app")
        secret, qrURL, err := twoFactorManager.GenerateTOTPSecret(userClaims.Username)
        if err != nil {
            response.InternalServerError(c, "Failed to generate TOTP secret")
            return
        }
    
        // 生成备份代码
        backupCodes, hashedCodes, err := twoFactorManager.GenerateBackupCodes(10)
        if err != nil {
            response.InternalServerError(c, "Failed to generate backup codes")
            return
        }
    
        // 保存到数据库
        // TODO: 保存secret和hashedCodes到用户记录
    
        response.Success(c, gin.H{
             "secret":       secret,
             "qr_url":       qrURL,
             "backup_codes": backupCodes, // 只返回一次，用户需要安全保存
        })
    })

    // 验证并启用双因子认证
    authGroup.POST("/verify", func(c *gin.Context) {
        var req struct {
            Code string `json:"code" binding:"required"`
        }
        
        if err := c.ShouldBindJSON(&req); err != nil {
            response.BadRequest(c, "Invalid request")
            return
        }
        
        userClaims, _ := auth.GetUserFromContext(c)
        
        // 从数据库获取用户和TOTP密钥
             // TODO: 获取用户和TOTP密钥
        
        twoFactorManager := auth.NewTwoFactorManager("dgou-app")
        valid, err := twoFactorManager.VerifyTOTPCode(secret, req.Code)
        if err != nil || !valid {
            response.BadRequest(c, "Invalid verification code")
            return
        }
        
        // 启用双因子认证
        // TODO: 更新用户记录，启用双因子认证
        
        response.Success(c, gin.H{
          "message": "Two-factor authentication enabled successfully",
        })
    })

    // 禁用双因子认证
    authGroup.POST("/disable", func(c *gin.Context) {
        // 需要验证密码或其他安全措施
        var req struct {
            Password string `json:"password" binding:"required"`
        }
        
        if err := c.ShouldBindJSON(&req); err != nil {
            response.BadRequest(c, "Invalid request")
            return
        }
        
        userClaims, _ := auth.GetUserFromContext(c)
        
        // 验证密码
        // TODO: 验证用户密码
        
        // 禁用双因子认证
        // TODO: 更新用户记录，禁用双因子认证
        
        response.Success(c, gin.H{
             "message": "Two-factor authentication disabled successfully",
        })
    })
}
```
#### OAuth2.0集成示例
```go

// OAuth2路由
func setupOAuth2Routes(router *gin.Engine, authManager *auth.AuthManager, oauth2Manager *auth.OAuth2Manager) {
    // OAuth2认证开始
    router.GET("/auth/:provider", func(c *gin.Context) {
        provider := auth.OAuth2Provider(c.Param("provider"))
        
        // 生成状态令牌，防止CSRF攻击
        state, err := generateStateToken()
        if err != nil {
            response.InternalServerError(c, "Failed to generate state token")
            return
        }
        
        // 保存状态到会话
             // TODO: 保存state到会话
        
        // 获取认证URL
        authURL, err := oauth2Manager.GetAuthURL(provider, state)
        if err != nil {
            response.InternalServerError(c, "Failed to get auth URL")
            return
        }
        
        // 重定向到OAuth2提供商
        c.Redirect(http.StatusFound, authURL)
    })
    
    // OAuth2回调
    router.GET("/auth/:provider/callback", func(c *gin.Context) {
        provider := auth.OAuth2Provider(c.Param("provider"))
        code := c.Query("code")
        state := c.Query("state")
        
        // 验证状态令牌
             // TODO: 从会话中获取并验证state
        
        // 交换代码获取令牌
        token, err := oauth2Manager.ExchangeCode(provider, code)
        if err != nil {
            response.Unauthorized(c, "Failed to exchange code for token")
            return
        }
        
        // 获取用户信息
        userInfo, err := oauth2Manager.GetUserInfo(provider, token.AccessToken)
        if err != nil {
            response.Unauthorized(c, "Failed to get user info")
            return
        }
        
        // 查找或创建用户
        user, err := findOrCreateOAuth2User(c.Request.Context(), provider, userInfo)
        if err != nil {
            response.InternalServerError(c, "Failed to find or create user")
            return
        }
        
        // 生成JWT令牌
        tokenPair, err := authManager.GenerateTokenPair(user)
        if err != nil {
            response.InternalServerError(c, "Failed to generate token")
            return
        }
        
        // 重定向到前端，携带令牌
        // 或者设置cookie，根据你的应用架构决定
        frontendURL := fmt.Sprintf("%s?token=%s", frontendCallbackURL, tokenPair.AccessToken)
        c.Redirect(http.StatusFound, frontendURL)
    })
}

func generateStateToken() (string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil
}
```
#### API密钥认证示例
```go

// API密钥路由
func setupAPIKeyRoutes(router *gin.Engine, apiKeyManager *auth.APIKeyManager) {
    apiGroup := router.Group("/api/keys")
    
    // 使用JWT认证中间件
    apiGroup.Use(authManager.AuthMiddleware())

    // 生成API密钥
    apiGroup.POST("", func(c *gin.Context) {
    var req struct {
        Name        string               `json:"name" binding:"required"`
        Permissions []auth.Permission    `json:"permissions"`
        ExpiresIn   *time.Duration       `json:"expires_in"` // 可选，例如：720h (30天)
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request")
        return
    }

    userClaims, _ := auth.GetUserFromContext(c)

    // 生成API密钥
    apiKey, fullKey, err := apiKeyManager.GenerateAPIKey(
        c.Request.Context(),
        userClaims.UserID,
        req.Name,
        req.Permissions,
        req.ExpiresIn,
    )

    if err != nil {
        response.InternalServerError(c, "Failed to generate API key")
        return
    }

    // 返回API密钥（只显示一次）
    response.Success(c, gin.H{
        "api_key": apiKey,
        "full_key": fullKey, // 注意：这个只返回一次！
        "warning": "Save this API key now! It will not be shown again.",
        })
    })

    // 列出API密钥
    apiGroup.GET("", func(c *gin.Context) {
        userClaims, _ := auth.GetUserFromContext(c)
        
        keys, err := apiKeyManager.ListUserAPIKeys(c.Request.Context(), userClaims.UserID)
        if err != nil {
            response.InternalServerError(c, "Failed to list API keys")
            return
        }
    
        response.Success(c, keys)
    })

    // 吊销API密钥
    apiGroup.DELETE("/:id", func(c *gin.Context) {
        keyID := c.Param("id")
        
        if err := apiKeyManager.RevokeAPIKey(c.Request.Context(), keyID); err != nil {
            response.InternalServerError(c, "Failed to revoke API key")
            return
        }
        
        response.Success(c, gin.H{
            "message": "API key revoked successfully",
        })
    })
}
    
// API路由使用API密钥认证
func setupAPIRoutes(router *gin.Engine, apiKeyManager *auth.APIKeyManager) {
    apiGroup := router.Group("/api/v1")
    
    // 使用API密钥认证中间件
    apiGroup.Use(apiKeyManager.APIKeyMiddleware())
    
    // 公共API端点
    apiGroup.GET("/health", func(c *gin.Context) {
        response.Success(c, gin.H{
            "status": "ok",
            "timestamp": time.Now().Unix(),
        })
    })
    
    apiGroup.GET("/data", func(c *gin.Context) {
        // 获取API密钥信息
        apiKeyInfo, exists := c.Get("api_key_info")
        if !exists {
            response.Unauthorized(c, "API key information not found")
            return
        }
        
        keyInfo := apiKeyInfo.(*auth.APIKey)
        
        // 检查权限
        hasPermission := false
        for _, perm := range keyInfo.Permissions {
            if perm == auth.PermissionArticleRead {
                hasPermission = true
                break
            }
        }
        
        if !hasPermission {
            response.Forbidden(c, "Insufficient permissions")
            return
        }
        
        // 返回数据
        data := fetchDataForAPIKey(keyInfo.ID)
        response.Success(c, data)
    })
}
```
#### 高级特性
##### 自定义用户声明
```go

// 添加自定义声明
claims := &auth.UserClaims{
     UserID:   user.ID,
     Username: user.Username,
     Email:    user.Email,
     Roles:    user.Roles,
     CustomClaims: map[string]interface{}{
       "department": "engineering",
       "team":       "backend",
       "employee_id": "EMP-12345",
    },
}

// 在中间件中访问自定义声明
func CustomMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userClaims, err := auth.GetUserFromContext(c)
        if err != nil {
             c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
        return
        }
        
        // 访问自定义声明
        if department, ok := userClaims.CustomClaims["department"].(string); ok {
        c.Set("user_department", department)
        }
        
        c.Next()
    }
}
```
##### 令牌黑名单
```go

// 吊销令牌
func revokeTokenHandler(c *gin.Context) {
    token, err := authManager.ExtractTokenFromHeader(c)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    if err := authManager.RevokeToken(c.Request.Context(), token, "user_logout"); err != nil {
        response.InternalServerError(c, "Failed to revoke token")
        return
    }
    
    response.Success(c, gin.H{
         "message": "Token revoked successfully",
    })
}

// 批量吊销用户所有令牌
func revokeAllUserTokensHandler(c *gin.Context) {
    userClaims, err := auth.GetUserFromContext(c)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    if err := authManager.RevokeAllUserTokens(c.Request.Context(), userClaims.UserID); err != nil {
        response.InternalServerError(c, "Failed to revoke tokens")
        return
    }
    
    response.Success(c, gin.H{
         "message": "All tokens revoked successfully",
    })
}
```
##### 会话管理
```go

// 获取用户所有会话
func listSessionsHandler(c *gin.Context) {
    userClaims, err := auth.GetUserFromContext(c)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }
    
    sessions, err := authManager.GetUserSessions(c.Request.Context(), userClaims.UserID)
    if err != nil {
        response.InternalServerError(c, "Failed to get sessions")
        return
    }
    
    response.Success(c, sessions)
}

// 终止特定会话
func terminateSessionHandler(c *gin.Context) {
    sessionID := c.Param("id")
    
    if err := authManager.TerminateSession(c.Request.Context(), sessionID); err != nil {
        response.InternalServerError(c, "Failed to terminate session")
        return
    }
    
    response.Success(c, gin.H{
         "message": "Session terminated successfully",
    })
}

// 终止除当前会话外的所有会话
func terminateOtherSessionsHandler(c *gin.Context) {
    userClaims, err := auth.GetUserFromContext(c)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }

    sessions, err := authManager.GetUserSessions(c.Request.Context(), userClaims.UserID)
    if err != nil {
        response.InternalServerError(c, "Failed to get sessions")
        return
    }

    currentSessionID, _ := c.Get("session_id")

    for _, session := range sessions {
        if session.ID != currentSessionID {
            _ = authManager.TerminateSession(c.Request.Context(), session.ID)
        }
    }

    response.Success(c, gin.H{
         "message": "Other sessions terminated successfully",
    })
}
```
##### 密码策略
```go

// 密码验证器
type PasswordValidator struct {
    MinLength     int
    RequireUpper  bool
    RequireLower  bool
    RequireNumber bool
    RequireSpecial bool
}

func NewPasswordValidator() *PasswordValidator {
    return &PasswordValidator{
         MinLength:     8,
         RequireUpper:  true,
         RequireLower:  true,
         RequireNumber: true,
         RequireSpecial: true,
    }
}

func (pv *PasswordValidator) Validate(password string) error {
    if len(password) < pv.MinLength {
        return fmt.Errorf("password must be at least %d characters long", pv.MinLength)
    }
    
    if pv.RequireUpper && !containsUpper(password) {
        return errors.New("password must contain at least one uppercase letter")
    }
    
    if pv.RequireLower && !containsLower(password) {
        return errors.New("password must contain at least one lowercase letter")
    }
    
    if pv.RequireNumber && !containsNumber(password) {
        return errors.New("password must contain at least one number")
    }
    
    if pv.RequireSpecial && !containsSpecial(password) {
        return errors.New("password must contain at least one special character")
    }
    
    return nil
}

// 密码重置
func resetPasswordHandler(c *gin.Context) {
    var req struct {
        CurrentPassword string `json:"current_password" binding:"required"`
        NewPassword     string `json:"new_password" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request")
        return
    }

    userClaims, err := auth.GetUserFromContext(c)
    if err != nil {
        response.Unauthorized(c, err.Error())
        return
    }

    // 验证当前密码
    user, err := userService.GetUserByID(c.Request.Context(), userClaims.UserID)
    if err != nil {
        response.InternalServerError(c, "Failed to get user")
        return
    }

    if err := auth.VerifyPassword(user.PasswordHash, req.CurrentPassword); err != nil {
        response.BadRequest(c, "Current password is incorrect")
        return
    }

    // 验证新密码强度
    validator := NewPasswordValidator()
    if err := validator.Validate(req.NewPassword); err != nil {
        response.BadRequest(c, err.Error())
        return
    }

    // 哈希新密码
    newHash, err := auth.HashPassword(req.NewPassword)
    if err != nil {
        response.InternalServerError(c, "Failed to hash password")
        return
    }

    // 更新密码
    if err := userService.UpdatePassword(c.Request.Context(), userClaims.UserID, newHash); err != nil {
        response.InternalServerError(c, "Failed to update password")
        return
    }

    // 吊销所有现有令牌（安全最佳实践）
    _ = authManager.RevokeAllUserTokens(c.Request.Context(), userClaims.UserID)
    
    response.Success(c, gin.H{
         "message": "Password updated successfully. Please login again.",
    })
}
```
#### 安全最佳实践
##### JWT安全配置
```go

// 使用强密钥
jwtSecret := generateStrongSecret(64)

// 设置合理的过期时间
jwtExpire := 15 * time.Minute     // 访问令牌：15分钟
refreshExpire := 7 * 24 * time.Hour // 刷新令牌：7天

// 使用HTTPS传输
// 启用HttpOnly和Secure Cookie
```
##### 防止令牌泄露
```go

// 令牌轮换
func rotateTokens(refreshToken string) (*auth.TokenPair, error) {
    // 验证刷新令牌
    // 生成新的访问令牌和刷新令牌
    // 吊销旧的刷新令牌
    // 返回新的令牌对
}

// 令牌绑定（绑定到IP、User-Agent等）
func bindTokenToContext(token string, ipAddress, userAgent string) bool {
    // 在令牌中存储上下文信息
    // 验证时检查上下文是否匹配
    // 不匹配则拒绝访问
}
```
##### 速率限制
```go

// 登录尝试限制
func loginRateLimitMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        key := fmt.Sprintf("login_attempts:%s", ip)
        
        attempts, _ := cache.Get(c.Request.Context(), key)
        if attempts >= 5 {
            c.JSON(http.StatusTooManyRequests, gin.H{
                 "error": "Too many login attempts. Please try again later.",
            })
            c.Abort()
            return
        }
        
        cache.Increment(c.Request.Context(), key, 1)
        cache.Expire(c.Request.Context(), key, 15*time.Minute)
        
        c.Next()
    }
}
```
#### 故障排除
##### 令牌验证失败
```go

// 检查令牌格式
token := strings.TrimPrefix(authHeader, "Bearer ")

// 检查令牌是否过期
claims, err := authManager.ParseToken(token)
if err != nil {
    if strings.Contains(err.Error(), "token is expired") {
        return errors.New("Token has expired. Please refresh or login again.")
    }
    return errors.New("Invalid token")
}

// 检查令牌是否被吊销
revoked, err := authManager.IsTokenRevoked(token)
if err != nil || revoked {
    return errors.New("Token has been revoked")
}
```
##### 权限检查失败
```go

// 检查用户角色
userClaims := GetUserFromContext(c)
if !authManager.HasRole(userClaims, auth.RoleAdmin) {
    return errors.New("Insufficient role privileges")
}

// 检查具体权限
if !authManager.HasPermission(userClaims, auth.PermissionUserDelete) {
    return errors.New("Insufficient permissions")
}

// 检查多个权限
requiredPermissions := []auth.Permission{
    auth.PermissionUserRead,
    auth.PermissionUserUpdate,
}

if !authManager.HasAllPermissions(userClaims, requiredPermissions) {
    return errors.New("Missing required permissions")
}
```
##### 双因子认证问题
```go

// 处理TOTP代码验证
func verifyTOTPCode(secret, code string) error {
    // 验证代码
    valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
         Period:    30,
         Skew:      1, // 允许前后一个周期
         Digits:    otp.DigitsSix,
         Algorithm: otp.AlgorithmSHA1,
    })
    
    if err != nil {
        return errors.Wrap(err, "Failed to validate TOTP code")
    }
    
    if !valid {
        return errors.New("Invalid TOTP code")
    }
    
    return nil
}

// 处理备份代码
func verifyBackupCode(code string, hashedCodes []string) (bool, error) {
    for _, hashed := range hashedCodes {
        if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(code)); err == nil {
            return true, nil
        }
    }
    return false, nil
}
```
这个认证组件提供了完整的生产级认证解决方案，支持多种认证方式和安全特性。您可以根据实际需求选择使用不同的认证策略。

