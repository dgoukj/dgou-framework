package auth

import (
	"crypto/rand"
	"crypto/sha1"
	"dgou/pkg/errors"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// TwoFactorManager 双因子认证管理器
type TwoFactorManager struct {
	issuer string // 签发者名称
}

// NewTwoFactorManager 创建双因子认证管理器
func NewTwoFactorManager(issuer string) *TwoFactorManager {
	if issuer == "" {
		issuer = "dgou-app"
	}

	return &TwoFactorManager{
		issuer: issuer,
	}
}

// GenerateTOTPSecret 生成TOTP密钥
func (tfm *TwoFactorManager) GenerateTOTPSecret(username string) (string, string, error) {
	// 生成随机密钥
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      tfm.issuer,
		AccountName: username,
		Period:      30, // 30秒有效期
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})

	if err != nil {
		return "", "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate TOTP secret")
	}

	// 返回密钥和URL
	return key.Secret(), key.URL(), nil
}

// VerifyTOTPCode 验证TOTP代码
func (tfm *TwoFactorManager) VerifyTOTPCode(secret, code string) (bool, error) {
	// 验证TOTP代码
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1, // 允许前后1个周期的时间偏移
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})

	if err != nil {
		return false, errors.Wrap(err, errors.CodeInternalError, "Failed to validate TOTP code")
	}

	return valid, nil
}

// GenerateBackupCodes 生成备份代码
func (tfm *TwoFactorManager) GenerateBackupCodes(count int) ([]string, []string, error) {
	if count <= 0 {
		count = 10
	}

	var codes []string
	var hashedCodes []string

	for i := 0; i < count; i++ {
		// 生成随机代码
		code, err := generateRandomCode(10)
		if err != nil {
			return nil, nil, err
		}

		// 哈希代码（用于存储）
		hashed, err := hashBackupCode(code)
		if err != nil {
			return nil, nil, err
		}

		codes = append(codes, code)
		hashedCodes = append(hashedCodes, hashed)
	}

	return codes, hashedCodes, nil
}

// VerifyBackupCode 验证备份代码
func (tfm *TwoFactorManager) VerifyBackupCode(code string, hashedCodes []string) (bool, error) {
	for _, hashed := range hashedCodes {
		if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(code)); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// generateRandomCode 生成随机代码
func generateRandomCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate random code")
	}

	// 使用base32编码，避免混淆字符
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	code := encoder.EncodeToString(bytes)

	// 截取指定长度
	if len(code) > length {
		code = code[:length]
	}

	// 转换为大写
	code = strings.ToUpper(code)

	return code, nil
}

// hashBackupCode 哈希备份代码
func hashBackupCode(code string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to hash backup code")
	}

	return string(hash), nil
}

// GenerateRecoveryCodes 生成恢复代码
func (tfm *TwoFactorManager) GenerateRecoveryCodes(count int) ([]string, error) {
	var codes []string

	for i := 0; i < count; i++ {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}

	return codes, nil
}

// generateRecoveryCode 生成恢复代码
func generateRecoveryCode() (string, error) {
	// 生成16字节的随机数据
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate recovery code")
	}

	// 使用SHA1哈希并取前8个字符
	hash := sha1.Sum(bytes)
	code := fmt.Sprintf("%x", hash[:8])

	return strings.ToUpper(code), nil
}

// ValidateRecoveryCode 验证恢复代码
func (tfm *TwoFactorManager) ValidateRecoveryCode(code string, storedHash string) (bool, error) {
	// 哈希输入的代码
	hash := sha1.Sum([]byte(code))
	hashedCode := fmt.Sprintf("%x", hash[:8])

	// 比较哈希值
	return strings.EqualFold(hashedCode, storedHash), nil
}

// GetQRCodeData 获取二维码数据
func (tfm *TwoFactorManager) GetQRCodeData(username, secret string) (string, error) {
	// 生成TOTP Key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      tfm.issuer,
		AccountName: username,
		Secret:      []byte(secret),
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})

	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to generate QR code data")
	}

	// 返回OTP URL，前端可以使用这个URL生成二维码
	return key.URL(), nil
}
