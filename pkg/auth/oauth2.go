package auth

import (
	"context"
	"dgou/pkg/errors"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuth2Manager OAuth2.0管理器
type OAuth2Manager struct {
	providers map[OAuth2Provider]*OAuth2ProviderConfig
	client    *http.Client
}

// OAuth2ProviderConfig OAuth2.0提供商配置
type OAuth2ProviderConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	RedirectURL  string
	Scopes       []string
}

// NewOAuth2Manager 创建OAuth2.0管理器
func NewOAuth2Manager() *OAuth2Manager {
	return &OAuth2Manager{
		providers: make(map[OAuth2Provider]*OAuth2ProviderConfig),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterProvider 注册OAuth2.0提供商
func (om *OAuth2Manager) RegisterProvider(provider OAuth2Provider, config *OAuth2ProviderConfig) error {
	if config == nil {
		return errors.New(errors.CodeValidationFailed, "OAuth2 provider config is required")
	}

	if config.ClientID == "" || config.ClientSecret == "" {
		return errors.New(errors.CodeValidationFailed, "OAuth2 client ID and secret are required")
	}

	om.providers[provider] = config
	return nil
}

// GetAuthURL 获取认证URL
func (om *OAuth2Manager) GetAuthURL(provider OAuth2Provider, state string) (string, error) {
	config, ok := om.providers[provider]
	if !ok {
		return "", errors.New(errors.CodeInternalError,
			fmt.Sprintf("OAuth2 provider %s not registered", provider))
	}

	// 构建认证URL
	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("redirect_uri", config.RedirectURL)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", strings.Join(config.Scopes, " "))

	authURL, err := url.Parse(config.AuthURL)
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to parse auth URL")
	}

	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

// ExchangeCode 交换代码获取令牌
func (om *OAuth2Manager) ExchangeCode(provider OAuth2Provider, code string) (*OAuth2Token, error) {
	config, ok := om.providers[provider]
	if !ok {
		return nil, errors.New(errors.CodeInternalError,
			fmt.Sprintf("OAuth2 provider %s not registered", provider))
	}

	// 准备请求数据
	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("client_secret", config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", config.RedirectURL)
	data.Set("grant_type", "authorization_code")

	// 发送请求
	req, err := http.NewRequest("POST", config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to create token request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := om.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to exchange code for token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.CodeUnauthorized,
			fmt.Sprintf("Failed to exchange code: %s", resp.Status))
	}

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to read token response")
	}

	var token OAuth2Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to parse token response")
	}

	return &token, nil
}

// GetUserInfo 获取用户信息
func (om *OAuth2Manager) GetUserInfo(provider OAuth2Provider, accessToken string) (*OAuth2UserInfo, error) {
	config, ok := om.providers[provider]
	if !ok {
		return nil, errors.New(errors.CodeInternalError,
			fmt.Sprintf("OAuth2 provider %s not registered", provider))
	}

	// 发送请求获取用户信息
	req, err := http.NewRequest("GET", config.UserInfoURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to create user info request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	resp, err := om.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to get user info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.CodeUnauthorized,
			fmt.Sprintf("Failed to get user info: %s", resp.Status))
	}

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to read user info response")
	}

	var userInfo OAuth2UserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to parse user info response")
	}

	return &userInfo, nil
}

// OAuth2Token OAuth2.0令牌
type OAuth2Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// OAuth2UserInfo OAuth2.0用户信息
type OAuth2UserInfo struct {
	ID            string                 `json:"id"`
	Username      string                 `json:"username"`
	Email         string                 `json:"email"`
	EmailVerified bool                   `json:"email_verified"`
	Name          string                 `json:"name"`
	GivenName     string                 `json:"given_name"`
	FamilyName    string                 `json:"family_name"`
	Picture       string                 `json:"picture"`
	Locale        string                 `json:"locale"`
	Provider      OAuth2Provider         `json:"provider"`
	RawData       map[string]interface{} `json:"raw_data,omitempty"`
}

// RefreshToken 刷新令牌
func (om *OAuth2Manager) RefreshToken(provider OAuth2Provider, refreshToken string) (*OAuth2Token, error) {
	config, ok := om.providers[provider]
	if !ok {
		return nil, errors.New(errors.CodeInternalError,
			fmt.Sprintf("OAuth2 provider %s not registered", provider))
	}

	// 准备请求数据
	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("client_secret", config.ClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	// 发送请求
	req, err := http.NewRequest("POST", config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to create refresh request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := om.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to refresh token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.CodeUnauthorized,
			fmt.Sprintf("Failed to refresh token: %s", resp.Status))
	}

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to read refresh response")
	}

	var token OAuth2Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to parse refresh response")
	}

	return &token, nil
}

// RevokeToken 吊销令牌
func (om *OAuth2Manager) RevokeToken(provider OAuth2Provider, token string) error {
	// 不同提供商的吊销端点不同，这里需要根据提供商实现
	// 这里只是一个示例
	switch provider {
	case OAuth2Google:
		return om.revokeGoogleToken(token)
	case OAuth2GitHub:
		return om.revokeGitHubToken(token)
	default:
		// 对于不支持吊销的提供商，返回成功
		return nil
	}
}

func (om *OAuth2Manager) revokeGoogleToken(token string) error {
	// Google令牌吊销端点
	revokeURL := "https://oauth2.googleapis.com/revoke"

	data := url.Values{}
	data.Set("token", token)

	req, err := http.NewRequest("POST", revokeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to create revoke request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := om.client.Do(req)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to revoke token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(errors.CodeInternalError,
			fmt.Sprintf("Failed to revoke token: %s", resp.Status))
	}

	return nil
}

func (om *OAuth2Manager) revokeGitHubToken(token string) error {
	// GitHub令牌吊销端点
	revokeURL := "https://api.github.com/applications/" + om.providers[OAuth2GitHub].ClientID + "/token"

	// GitHub使用HTTP Basic认证
	req, err := http.NewRequest("DELETE", revokeURL, nil)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to create revoke request")
	}

	req.SetBasicAuth(om.providers[OAuth2GitHub].ClientID, om.providers[OAuth2GitHub].ClientSecret)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := om.client.Do(req)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to revoke token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return errors.New(errors.CodeInternalError,
			fmt.Sprintf("Failed to revoke token: %s", resp.Status))
	}

	return nil
}

// ValidateIDToken 验证ID令牌
func (om *OAuth2Manager) ValidateIDToken(provider OAuth2Provider, idToken string) (*OAuth2UserInfo, error) {
	// 验证ID令牌的逻辑因提供商而异
	// 这里只是一个示例，实际需要根据提供商实现
	switch provider {
	case OAuth2Google:
		return om.validateGoogleIDToken(idToken)
	case OAuth2Apple:
		return om.validateAppleIDToken(idToken)
	default:
		return nil, errors.New(errors.CodeValidationFailed,
			"ID token validation not supported for this provider")
	}
}

func (om *OAuth2Manager) validateGoogleIDToken(idToken string) (*OAuth2UserInfo, error) {
	// Google ID令牌验证端点
	verifyURL := "https://oauth2.googleapis.com/tokeninfo"

	params := url.Values{}
	params.Set("id_token", idToken)

	resp, err := om.client.Get(verifyURL + "?" + params.Encode())
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to verify ID token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.CodeUnauthorized,
			fmt.Sprintf("Failed to verify ID token: %s", resp.Status))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to read ID token response")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(body, &claims); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to parse ID token claims")
	}

	// 验证受众
	if aud, ok := claims["aud"].(string); !ok || aud != om.providers[OAuth2Google].ClientID {
		return nil, errors.New(errors.CodeUnauthorized, "Invalid ID token audience")
	}

	// 构建用户信息
	userInfo := &OAuth2UserInfo{
		ID:            getString(claims, "sub"),
		Email:         getString(claims, "email"),
		EmailVerified: getBool(claims, "email_verified"),
		Name:          getString(claims, "name"),
		GivenName:     getString(claims, "given_name"),
		FamilyName:    getString(claims, "family_name"),
		Picture:       getString(claims, "picture"),
		Locale:        getString(claims, "locale"),
		Provider:      OAuth2Google,
		RawData:       claims,
	}

	return userInfo, nil
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

// GetDefaultProviders 获取默认的OAuth2.0提供商配置
func GetDefaultProviders() map[OAuth2Provider]*OAuth2ProviderConfig {
	return map[OAuth2Provider]*OAuth2ProviderConfig{
		OAuth2Google: {
			AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:    "https://oauth2.googleapis.com/token",
			UserInfoURL: "https://www.googleapis.com/oauth2/v3/userinfo",
			Scopes:      []string{"openid", "email", "profile"},
		},
		OAuth2GitHub: {
			AuthURL:     "https://github.com/login/oauth/authorize",
			TokenURL:    "https://github.com/login/oauth/access_token",
			UserInfoURL: "https://api.github.com/user",
			Scopes:      []string{"user:email", "read:user"},
		},
		OAuth2Facebook: {
			AuthURL:     "https://www.facebook.com/v12.0/dialog/oauth",
			TokenURL:    "https://graph.facebook.com/v12.0/oauth/access_token",
			UserInfoURL: "https://graph.facebook.com/v12.0/me",
			Scopes:      []string{"email", "public_profile"},
		},
		OAuth2Microsoft: {
			AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			UserInfoURL: "https://graph.microsoft.com/v1.0/me",
			Scopes:      []string{"User.Read", "openid", "email", "profile"},
		},
	}
}
