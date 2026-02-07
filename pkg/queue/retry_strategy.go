// file: pkg/queue/retry_strategy.go
package queue

import (
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"math"
	"strings"
	"time"
)

// RetryStrategy 重试策略
type RetryStrategy string

const (
	RetryStrategyFixed       RetryStrategy = "fixed"       // 固定间隔重试
	RetryStrategyLinear      RetryStrategy = "linear"      // 线性增长重试
	RetryStrategyExponential RetryStrategy = "exponential" // 指数退避重试
	RetryStrategyFibonacci   RetryStrategy = "fibonacci"   // 斐波那契重试
)

// RetryConfig 重试配置
type RetryConfig struct {
	Enabled       bool          `mapstructure:"enabled"`        // 是否启用重试
	MaxRetries    int           `mapstructure:"max_retries"`    // 最大重试次数
	InitialDelay  time.Duration `mapstructure:"initial_delay"`  // 初始延迟
	MaxDelay      time.Duration `mapstructure:"max_delay"`      // 最大延迟
	Strategy      RetryStrategy `mapstructure:"strategy"`       // 重试策略
	BackoffFactor float64       `mapstructure:"backoff_factor"` // 退避因子
	Jitter        bool          `mapstructure:"jitter"`         // 是否添加抖动
	JitterFactor  float64       `mapstructure:"jitter_factor"`  // 抖动因子
}

// RetryManager 重试管理器
type RetryManager struct {
	config *RetryConfig // 配置
}

// RetryResult 重试结果
type RetryResult struct {
	Success    bool          `json:"success"`     // 是否成功
	Attempts   int           `json:"attempts"`    // 尝试次数
	TotalDelay time.Duration `json:"total_delay"` // 总延迟
	LastError  error         `json:"last_error"`  // 最后错误
}

// NewRetryConfig 创建重试配置
func NewRetryConfig() *RetryConfig {
	return &RetryConfig{
		Enabled:       true,
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      60 * time.Second,
		Strategy:      RetryStrategyExponential,
		BackoffFactor: 2.0,
		Jitter:        true,
		JitterFactor:  0.1,
	}
}

// NewRetryManager 创建重试管理器
func NewRetryManager(config *RetryConfig) *RetryManager {
	if config == nil {
		config = NewRetryConfig()
	}

	return &RetryManager{
		config: config,
	}
}

// Execute 执行带重试的操作
func (rm *RetryManager) Execute(operation func() error) (*RetryResult, error) {
	if !rm.config.Enabled {
		// 不启用重试，直接执行
		err := operation()
		return &RetryResult{
			Success:   err == nil,
			Attempts:  1,
			LastError: err,
		}, err
	}

	var lastErr error
	totalDelay := time.Duration(0)

	for attempt := 0; attempt <= rm.config.MaxRetries; attempt++ {
		// 执行操作
		err := operation()

		if err == nil {
			// 操作成功
			return &RetryResult{
				Success:    true,
				Attempts:   attempt + 1,
				TotalDelay: totalDelay,
			}, nil
		}

		lastErr = err

		// 如果是最后一次尝试，直接返回
		if attempt == rm.config.MaxRetries {
			logger.Warn("Max retries reached",
				logger.Int("max_retries", rm.config.MaxRetries),
				logger.ErrorField(err),
			)
			break
		}

		// 计算下一次重试的延迟
		delay := rm.calculateDelay(attempt)
		totalDelay += delay

		logger.Warn("Operation failed, retrying...",
			logger.Int("attempt", attempt+1),
			logger.Int("max_retries", rm.config.MaxRetries),
			logger.Duration("delay", delay),
			logger.ErrorField(err),
		)

		// 等待重试延迟
		time.Sleep(delay)
	}

	return &RetryResult{
		Success:    false,
		Attempts:   rm.config.MaxRetries + 1,
		TotalDelay: totalDelay,
		LastError:  lastErr,
	}, lastErr
}

// calculateDelay 计算重试延迟
func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch rm.config.Strategy {
	case RetryStrategyFixed:
		delay = rm.config.InitialDelay

	case RetryStrategyLinear:
		delay = rm.config.InitialDelay * time.Duration(attempt+1)

	case RetryStrategyExponential:
		delay = time.Duration(float64(rm.config.InitialDelay) *
			math.Pow(rm.config.BackoffFactor, float64(attempt)))

	case RetryStrategyFibonacci:
		delay = rm.config.InitialDelay * time.Duration(fibonacci(attempt+1))

	default:
		delay = rm.config.InitialDelay
	}

	// 限制最大延迟
	if delay > rm.config.MaxDelay {
		delay = rm.config.MaxDelay
	}

	// 添加抖动（如果启用）
	if rm.config.Jitter {
		delay = rm.addJitter(delay, rm.config.JitterFactor)
	}

	return delay
}

// addJitter 添加抖动
func (rm *RetryManager) addJitter(delay time.Duration, jitterFactor float64) time.Duration {
	if jitterFactor <= 0 {
		return delay
	}

	// 生成随机抖动
	jitter := float64(delay) * jitterFactor
	randomJitter := time.Duration(float64(delay) - jitter + (2 * jitter * rand.Float64()))

	return randomJitter
}

// fibonacci 计算斐波那契数列
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}

	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

// ShouldRetry 判断是否应该重试
func (rm *RetryManager) ShouldRetry(err error, retryCount int) bool {
	if !rm.config.Enabled {
		return false
	}

	if retryCount >= rm.config.MaxRetries {
		return false
	}

	// 检查错误类型，某些错误不应该重试
	if errors.Is(err, errors.CodeValidationFailed) {
		// 验证错误不应该重试
		return false
	}

	if errors.Is(err, errors.CodeResourceNotFound) {
		// 资源不存在错误不应该重试
		return false
	}

	// 检查错误消息中是否包含不应重试的关键字
	errorMsg := err.Error()
	nonRetryableErrors := []string{
		"invalid",
		"not found",
		"unauthorized",
		"forbidden",
		"permission denied",
	}

	for _, keyword := range nonRetryableErrors {
		if containsIgnoreCase(errorMsg, keyword) {
			return false
		}
	}

	return true
}

// containsIgnoreCase 检查字符串是否包含子串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	// 简单实现，实际项目中可能需要更复杂的处理
	return len(s) >= len(substr) &&
		strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// GetRetryDelay 获取重试延迟
func (rm *RetryManager) GetRetryDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return 0
	}

	return rm.calculateDelay(retryCount - 1)
}

// UpdateRetryHeaders 更新消息重试头信息
func (rm *RetryManager) UpdateRetryHeaders(msg *Message, retryCount int) {
	if msg.Headers == nil {
		msg.Headers = make(map[string]interface{})
	}

	msg.RetryCount = retryCount
	msg.MaxRetries = rm.config.MaxRetries
	msg.RetryDelay = rm.GetRetryDelay(retryCount)

	msg.Headers["x-retry-count"] = retryCount
	msg.Headers["x-max-retries"] = rm.config.MaxRetries
	msg.Headers["x-retry-delay"] = msg.RetryDelay.String()
	msg.Headers["x-retry-strategy"] = string(rm.config.Strategy)
	msg.Headers["x-last-retry-time"] = time.Now().Format(time.RFC3339)

	// 计算下一次重试时间
	if retryCount < rm.config.MaxRetries {
		nextRetryTime := time.Now().Add(msg.RetryDelay)
		msg.Headers["x-next-retry-time"] = nextRetryTime.Format(time.RFC3339)
	}
}

// GetRetryInfo 获取重试信息
func (rm *RetryManager) GetRetryInfo(retryCount int) map[string]interface{} {
	nextDelay := rm.GetRetryDelay(retryCount)
	remainingRetries := rm.config.MaxRetries - retryCount

	return map[string]interface{}{
		"retry_count":       retryCount,
		"max_retries":       rm.config.MaxRetries,
		"remaining_retries": remainingRetries,
		"next_delay":        nextDelay.String(),
		"strategy":          rm.config.Strategy,
		"enabled":           rm.config.Enabled,
	}
}
