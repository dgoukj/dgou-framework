// file: pkg/upload/init.go
package upload

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/logger"
	"mime/multipart"
	"sync"
	"time"
)

var (
	// defaultManager 默认上传管理器实例
	defaultManager *UploadManager
	// defaultService 默认上传服务实例
	defaultService *UploadService
	// managerMutex 用于保护默认管理器实例
	managerMutex sync.RWMutex
	// serviceMutex 用于保护默认服务实例
	serviceMutex sync.RWMutex
	// isInitialized 标记上传组件是否已初始化
	isInitialized bool
	// initMutex 用于保护初始化过程
	initMutex sync.Mutex
)

// Init 初始化上传组件（使用默认配置）
func Init() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if isInitialized {
		logger.Info("Upload component already initialized")
		return nil
	}

	// 获取应用配置
	cfg := config.GetConfig()
	if cfg == nil {
		logger.Warn("Config not found, using default configuration")
		cfg = &config.Config{
			Upload: config.UploadConfig{
				StorageType:       "local",
				BasePath:          "./uploads",
				BaseURL:           "http://localhost:8080/uploads",
				MaxFileSize:       10 * 1024 * 1024, // 10MB
				AllowedTypes:      []string{"image", "document"},
				AllowedMimeTypes:  []string{"image/jpeg", "image/png", "application/pdf"},
				AllowedExtensions: []string{".jpg", ".jpeg", ".png", ".pdf", ".doc", ".docx"},
				ChunkEnabled:      true,
				ChunkSize:         5 * 1024 * 1024, // 5MB
				MaxChunks:         1000,
				TempDir:           "/tmp/upload_chunks",
				CleanupInterval:   30 * time.Minute,
				MaxTempFileAge:    24 * time.Hour,
				EnableResumable:   true,
				EnableVirusScan:   false,
				EnableCDN:         false,
				CDNURL:            "",
				AccessKeyID:       "",
				AccessKeySecret:   "",
				Endpoint:          "",
				Bucket:            "",
				Region:            "",
				UseHTTPS:          false,
				MinFileSize:       0,
				MaxFileNameLength: 255,
				ValidateVirus:     false,
				ScanTimeout:       30,
			},
		}
	}

	return InitWithConfig(cfg)
}

// InitWithConfig 使用自定义配置初始化上传组件
func InitWithConfig(cfg *config.Config) error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if isInitialized {
		logger.Info("Upload component already initialized, reinitializing")
		// 先停止已有的管理器
		if defaultManager != nil {
			defaultManager.Stop()
		}
	}

	// 创建上传管理器
	manager, err := InitUploadManager(cfg)
	if err != nil {
		logger.Error("Failed to initialize upload manager",
			logger.ErrorField(err),
		)
		return err
	}

	// 创建上传服务
	service := NewUploadService(manager)

	// 设置默认实例
	SetDefaultManager(manager)
	SetDefaultService(service)
	isInitialized = true

	logger.Info("Upload component initialized successfully",
		logger.String("storage_type", string(manager.storage.Type())),
		logger.Bool("virus_scan_enabled", manager.config.EnableVirusScan),
		logger.Bool("chunk_upload_enabled", manager.config.ChunkConfig.Enabled),
	)

	return nil
}

// SetDefaultManager 设置默认上传管理器
func SetDefaultManager(manager *UploadManager) {
	managerMutex.Lock()
	defer managerMutex.Unlock()
	defaultManager = manager
}

// SetDefaultService 设置默认上传服务
func SetDefaultService(service *UploadService) {
	serviceMutex.Lock()
	defer serviceMutex.RUnlock()
	defaultService = service
}

// GetDefaultManager 获取默认上传管理器
func GetDefaultManager() *UploadManager {
	managerMutex.RLock()
	defer managerMutex.RUnlock()
	return defaultManager
}

// GetDefaultService 获取默认上传服务
func GetDefaultService() *UploadService {
	serviceMutex.RLock()
	defer serviceMutex.RUnlock()
	return defaultService
}

// IsInitialized 检查上传组件是否已初始化
func IsInitialized() bool {
	managerMutex.RLock()
	defer managerMutex.RUnlock()
	return defaultManager != nil && isInitialized
}

// UploadSingle 上传单个文件（使用默认服务）
func UploadSingle(ctx context.Context, req *UploadRequest) (*UploadResponse, error) {
	service := GetDefaultService()
	if service == nil {
		return nil, NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.UploadSingle(ctx, req)
}

// UploadMultiple 上传多个文件（使用默认服务）
func UploadMultiple(ctx context.Context, files []*multipart.FileHeader, options *UploadOptions) ([]*FileInfo, []error) {
	service := GetDefaultService()
	if service == nil {
		return nil, []error{NewError("Upload service not initialized, please call upload.Init() first")}
	}
	return service.UploadMultiple(ctx, files, options)
}

// ChunkUploadStart 开始分片上传（使用默认服务）
func ChunkUploadStart(ctx context.Context, req *ChunkUploadStartRequest) (*ChunkUploadStartResponse, error) {
	service := GetDefaultService()
	if service == nil {
		return nil, NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.ChunkUploadStart(ctx, req)
}

// UploadChunk 上传分片（使用默认服务）
func UploadChunk(ctx context.Context, req *ChunkUploadRequest) error {
	service := GetDefaultService()
	if service == nil {
		return NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.UploadChunk(ctx, req)
}

// CompleteChunkUpload 完成分片上传（使用默认服务）
func CompleteChunkUpload(ctx context.Context, req *CompleteChunkUploadRequest) (*FileInfo, error) {
	service := GetDefaultService()
	if service == nil {
		return nil, NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.CompleteChunkUpload(ctx, req)
}

// GetUploadProgress 获取上传进度（使用默认服务）
func GetUploadProgress(ctx context.Context, uploadID string) (map[string]interface{}, error) {
	service := GetDefaultService()
	if service == nil {
		return nil, NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.GetUploadProgress(ctx, uploadID)
}

// GetFileInfo 获取文件信息（使用默认服务）
func GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	service := GetDefaultService()
	if service == nil {
		return nil, NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.GetFileInfo(ctx, path)
}

// GetFileURL 获取文件URL（使用默认服务）
func GetFileURL(ctx context.Context, path string, public bool) (string, error) {
	service := GetDefaultService()
	if service == nil {
		return "", NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.GetFileURL(ctx, path, public)
}

// DeleteFile 删除文件（使用默认服务）
func DeleteFile(ctx context.Context, path string) error {
	service := GetDefaultService()
	if service == nil {
		return NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.DeleteFile(ctx, path)
}

// CopyFile 复制文件（使用默认服务）
func CopyFile(ctx context.Context, srcPath, dstPath string) error {
	service := GetDefaultService()
	if service == nil {
		return NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.CopyFile(ctx, srcPath, dstPath)
}

// MoveFile 移动文件（使用默认服务）
func MoveFile(ctx context.Context, srcPath, dstPath string) error {
	service := GetDefaultService()
	if service == nil {
		return NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.MoveFile(ctx, srcPath, dstPath)
}

// FileExists 检查文件是否存在（使用默认服务）
func FileExists(ctx context.Context, path string) (bool, error) {
	service := GetDefaultService()
	if service == nil {
		return false, NewError("Upload service not initialized, please call upload.Init() first")
	}
	return service.FileExists(ctx, path)
}

// Stop 停止上传组件
func Stop() error {
	initMutex.Lock()
	defer initMutex.Unlock()

	if !isInitialized {
		logger.Info("Upload component not initialized")
		return nil
	}

	manager := GetDefaultManager()
	if manager == nil {
		isInitialized = false
		return nil
	}

	// 停止管理器
	manager.Stop()

	// 清理状态
	managerMutex.Lock()
	serviceMutex.Lock()
	defaultManager = nil
	defaultService = nil
	isInitialized = false
	serviceMutex.Unlock()
	managerMutex.Unlock()

	logger.Info("Upload component stopped successfully")
	return nil
}

// GetStorageType 获取当前存储类型
func GetStorageType() string {
	manager := GetDefaultManager()
	if manager == nil {
		return string(StorageTypeLocal)
	}
	return string(manager.storage.Type())
}

// IsChunkUploadEnabled 检查分片上传是否启用
func IsChunkUploadEnabled() bool {
	manager := GetDefaultManager()
	if manager == nil {
		return false
	}
	return manager.config.ChunkConfig.Enabled
}

// IsVirusScanEnabled 检查病毒扫描是否启用
func IsVirusScanEnabled() bool {
	manager := GetDefaultManager()
	if manager == nil {
		return false
	}
	return manager.config.EnableVirusScan
}

// GetMaxFileSize 获取最大文件大小限制
func GetMaxFileSize() int64 {
	manager := GetDefaultManager()
	if manager == nil {
		return 10 * 1024 * 1024 // 10MB默认值
	}
	return manager.config.ValidationConfig.MaxFileSize
}

// GetAllowedTypes 获取允许的文件类型
func GetAllowedTypes() []string {
	manager := GetDefaultManager()
	if manager == nil {
		return []string{string(FileTypeImage), string(FileTypeDocument)}
	}
	// 将FileType转换为字符串
	var allowedTypes []string
	for _, ft := range manager.config.ValidationConfig.AllowedTypes {
		allowedTypes = append(allowedTypes, string(ft))
	}
	return allowedTypes
}

// NewError 创建上传组件错误
func NewError(message string) error {
	return &UploadError{
		Message: message,
	}
}

// UploadError 上传组件错误
type UploadError struct {
	Message string
}

// Error 实现error接口
func (e *UploadError) Error() string {
	return e.Message
}

// 初始化检查
func init() {
	// 注册配置验证器
	config.RegisterValidator(func(cfg *config.Config) error {
		// 验证上传配置
		if cfg.Upload.BasePath == "" && cfg.Upload.StorageType == "local" {
			logger.Warn("Upload base path not configured, using default")
		}

		// 验证OSS配置
		if cfg.Upload.StorageType == "oss" {
			if cfg.Upload.AccessKeyID == "" || cfg.Upload.AccessKeySecret == "" {
				logger.Warn("OSS access key not configured, OSS storage will not work")
			}
			if cfg.Upload.Bucket == "" {
				logger.Warn("OSS bucket not configured")
			}
		}

		return nil
	})

	// 注册优雅关闭处理器
	config.RegisterShutdownHandler(func() error {
		return Stop()
	})

	logger.Info("Upload component package initialized")
}
