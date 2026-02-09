// file: pkg/async/config_helper.go
package async

import (
	"dgou/pkg/config"
	"sync"
)

var (
	validators       []func(*config.Config) error
	shutdownHandlers []func() error
	validatorMutex   sync.RWMutex
	shutdownMutex    sync.RWMutex
)

// RegisterValidator 注册配置验证器
func RegisterValidator(validator func(*config.Config) error) {
	validatorMutex.Lock()
	defer validatorMutex.Unlock()
	validators = append(validators, validator)
}

// RegisterShutdownHandler 注册优雅关闭处理器
func RegisterShutdownHandler(handler func() error) {
	shutdownMutex.Lock()
	defer shutdownMutex.Unlock()
	shutdownHandlers = append(shutdownHandlers, handler)
}

// GetValidators 获取所有验证器
func GetValidators() []func(*config.Config) error {
	validatorMutex.RLock()
	defer validatorMutex.RUnlock()
	result := make([]func(*config.Config) error, len(validators))
	copy(result, validators)
	return result
}

// GetShutdownHandlers 获取所有关闭处理器
func GetShutdownHandlers() []func() error {
	shutdownMutex.RLock()
	defer shutdownMutex.RUnlock()
	result := make([]func() error, len(shutdownHandlers))
	copy(result, shutdownHandlers)
	return result
}
