// file: pkg/upload/manager.go
package upload

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// UploadConfig 上传配置
type UploadConfig struct {
	StorageType      StorageType       `mapstructure:"storage_type"`
	StorageConfig    StorageConfig     `mapstructure:"storage_config"`
	ValidationConfig ValidationConfig  `mapstructure:"validation_config"`
	ChunkConfig      ChunkUploadConfig `mapstructure:"chunk_config"`
	EnableVirusScan  bool              `mapstructure:"enable_virus_scan"`
}

// UploadManager 上传管理器
type UploadManager struct {
	config       *UploadConfig
	storage      Storage
	validator    *Validator
	chunkManager *ChunkUploadManager
	mu           sync.RWMutex
}

// UploadOptions 上传选项
type UploadOptions struct {
	UploaderID   string
	UploaderIP   string
	IsPublic     bool
	Category     string
	SubDirectory string
	CustomPath   string
	ExpiredAt    *time.Time
	Metadata     map[string]interface{}
}

// NewUploadManager 创建上传管理器
func NewUploadManager(config *UploadConfig) (*UploadManager, error) {
	if config == nil {
		return nil, errors.New(errors.CodeValidationFailed, "Upload config is required")
	}

	// 创建存储
	var storage Storage
	var err error

	switch config.StorageType {
	case StorageTypeLocal:
		storage, err = NewLocalStorage(&config.StorageConfig)
	case StorageTypeOSS:
		storage, err = NewOSSStorage(&config.StorageConfig)
	default:
		return nil, errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Unsupported storage type: %s", config.StorageType))
	}

	if err != nil {
		return nil, err
	}

	// 创建验证器
	validator := NewValidator(&config.ValidationConfig)

	// 创建分片上传管理器
	var chunkManager *ChunkUploadManager
	if config.ChunkConfig.Enabled {
		chunkManager, err = NewChunkUploadManager(&config.ChunkConfig)
		if err != nil {
			return nil, err
		}
	}

	manager := &UploadManager{
		config:       config,
		storage:      storage,
		validator:    validator,
		chunkManager: chunkManager,
	}

	logger.Info("Upload manager initialized",
		logger.String("storage_type", string(config.StorageType)),
		logger.Bool("virus_scan_enabled", config.EnableVirusScan),
		logger.Bool("chunk_upload_enabled", config.ChunkConfig.Enabled),
	)

	return manager, nil
}

// Upload 上传文件
func (m *UploadManager) Upload(ctx context.Context, header *multipart.FileHeader, options *UploadOptions) (*FileInfo, error) {
	startTime := time.Now()

	// 验证文件
	if err := m.validator.Validate(header); err != nil {
		return nil, err
	}

	// 生成文件信息
	fileInfo, err := m.generateFileInfo(header, options)
	if err != nil {
		return nil, err
	}

	// 设置上传者信息
	if options != nil {
		fileInfo.UploaderID = options.UploaderID
		fileInfo.UploaderIP = options.UploaderIP
		fileInfo.IsPublic = options.IsPublic
		fileInfo.ExpiredAt = options.ExpiredAt

		if options.Metadata != nil {
			fileInfo.Metadata = options.Metadata
		}
	}

	// 打开文件
	src, err := header.Open()
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to open uploaded file")
	}
	defer src.Close()

	// 如果是分片上传且文件较大，使用分片上传
	if m.chunkManager != nil && header.Size > m.config.ChunkConfig.ChunkSize {
		return m.uploadWithChunks(ctx, src, fileInfo, header.Size, options)
	}

	// 直接上传
	if err := m.storage.PutMultipart(ctx, fileInfo, header); err != nil {
		return nil, err
	}

	// 病毒扫描
	if m.config.EnableVirusScan {
		if err := m.scanVirus(ctx, fileInfo); err != nil {
			// 如果扫描失败，记录错误但不阻止上传
			logger.Error("Virus scan failed",
				logger.String("file_id", fileInfo.ID),
				logger.String("file_name", fileInfo.Name),
				logger.ErrorField(err),
			)
		}
	}

	duration := time.Since(startTime)
	logger.Info("File uploaded successfully",
		logger.String("file_id", fileInfo.ID),
		logger.String("file_name", fileInfo.Name),
		logger.String("storage_name", fileInfo.StorageName),
		logger.String("path", fileInfo.Path),
		logger.Int64("size", fileInfo.Size),
		logger.String("storage_type", string(fileInfo.StorageType)),
		logger.Duration("duration", duration),
	)

	return fileInfo, nil
}

// uploadWithChunks 使用分片上传
func (m *UploadManager) uploadWithChunks(ctx context.Context, src io.Reader, fileInfo *FileInfo, fileSize int64, options *UploadOptions) (*FileInfo, error) {
	// 计算文件哈希
	hash, err := m.calculateFileHash(src)
	if err != nil {
		return nil, err
	}

	// 创建上传会话
	session, err := m.chunkManager.CreateUploadSession(fileInfo.Name, fileSize, hash, options.Metadata)
	if err != nil {
		return nil, err
	}

	// 重置读取位置
	if seeker, ok := src.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError,
				"Failed to seek to file start")
		}
	}

	// 分片上传
	chunkNumber := 1
	buffer := make([]byte, m.config.ChunkConfig.ChunkSize)

	for {
		n, err := src.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, errors.Wrap(err, errors.CodeInternalError,
				"Failed to read file chunk")
		}

		if n == 0 {
			break
		}

		chunkData := buffer[:n]
		chunkHash := calculateChunkHash(chunkData)

		// 上传分片
		if err := m.chunkManager.UploadChunk(session.UploadID, chunkNumber,
			strings.NewReader(string(chunkData)), int64(n), chunkHash); err != nil {
			return nil, err
		}

		chunkNumber++

		if err == io.EOF {
			break
		}
	}

	// 完成上传
	completedSession, err := m.chunkManager.CompleteUpload(session.UploadID)
	if err != nil {
		return nil, err
	}

	// 获取合并后的文件路径
	mergedFilePath, ok := completedSession.Metadata["merged_file_path"].(string)
	if !ok {
		return nil, errors.New(errors.CodeInternalError,
			"Failed to get merged file path")
	}

	// 打开合并后的文件
	mergedFile, err := os.Open(mergedFilePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to open merged file: %s", mergedFilePath))
	}
	defer mergedFile.Close()

	// 上传到最终存储
	fileInfo.Size = fileSize
	fileInfo.MD5 = hash

	if err := m.storage.Put(ctx, fileInfo, mergedFile); err != nil {
		return nil, err
	}

	// 清理合并文件
	if err := os.Remove(mergedFilePath); err != nil {
		logger.Warn("Failed to delete merged file",
			logger.String("file_path", mergedFilePath),
			logger.ErrorField(err),
		)
	}

	logger.Info("File uploaded with chunks",
		logger.String("file_id", fileInfo.ID),
		logger.String("file_name", fileInfo.Name),
		logger.Int64("file_size", fileSize),
		logger.Int("total_chunks", completedSession.TotalChunks),
		logger.String("upload_id", session.UploadID),
	)

	return fileInfo, nil
}

// GetFile 获取文件
func (m *UploadManager) GetFile(ctx context.Context, path string) (io.ReadCloser, error) {
	return m.storage.Get(ctx, path)
}

// GetFileInfo 获取文件信息
func (m *UploadManager) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	return m.storage.Stat(ctx, path)
}

// GetFileURL 获取文件URL
func (m *UploadManager) GetFileURL(ctx context.Context, path string, public bool) (string, error) {
	return m.storage.GetURL(ctx, path, public)
}

// DeleteFile 删除文件
func (m *UploadManager) DeleteFile(ctx context.Context, path string) error {
	return m.storage.Delete(ctx, path)
}

// CopyFile 复制文件
func (m *UploadManager) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	return m.storage.Copy(ctx, srcPath, dstPath)
}

// MoveFile 移动文件
func (m *UploadManager) MoveFile(ctx context.Context, srcPath, dstPath string) error {
	return m.storage.Move(ctx, srcPath, dstPath)
}

// FileExists 检查文件是否存在
func (m *UploadManager) FileExists(ctx context.Context, path string) (bool, error) {
	return m.storage.Exists(ctx, path)
}

// generateFileInfo 生成文件信息
func (m *UploadManager) generateFileInfo(header *multipart.FileHeader, options *UploadOptions) (*FileInfo, error) {
	// 生成文件ID
	fileID := generateFileID()

	// 原始文件名
	originalName := header.Filename

	// 生成存储文件名
	storageName := m.generateStorageName(originalName, fileID)

	// 生成存储路径
	storagePath := m.generateStoragePath(storageName, options)

	// 获取MIME类型
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType([]byte(originalName))
	}

	// 获取文件扩展名
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(originalName)), ".")

	// 获取URL
	url, err := m.storage.GetURL(context.Background(), storagePath, true)
	if err != nil {
		// 如果获取URL失败，使用默认URL
		url = storagePath
	}

	return &FileInfo{
		ID:          fileID,
		Name:        originalName,
		StorageName: storageName,
		Path:        storagePath,
		URL:         url,
		Size:        header.Size,
		MimeType:    mimeType,
		Extension:   extension,
		StorageType: m.storage.Type(),
		CreatedAt:   time.Now(),
		IsPublic:    options != nil && options.IsPublic,
	}, nil
}

// generateStorageName 生成存储文件名
func (m *UploadManager) generateStorageName(originalName, fileID string) string {
	// 获取文件扩展名
	ext := filepath.Ext(originalName)

	// 生成唯一文件名
	timestamp := time.Now().Format("20060102_150405")
	randomStr := fileID[:8]

	return fmt.Sprintf("%s_%s%s", timestamp, randomStr, ext)
}

// generateStoragePath 生成存储路径
func (m *UploadManager) generateStoragePath(storageName string, options *UploadOptions) string {
	var path string

	if options != nil && options.CustomPath != "" {
		// 使用自定义路径
		path = options.CustomPath
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
		path += storageName
	} else {
		// 使用分类和子目录
		category := "default"
		subDir := time.Now().Format("2006/01/02")

		if options != nil {
			if options.Category != "" {
				category = options.Category
			}
			if options.SubDirectory != "" {
				subDir = options.SubDirectory
			}
		}

		path = fmt.Sprintf("%s/%s/%s", category, subDir, storageName)
	}

	// 清理路径
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "//", "/")

	return path
}

// calculateFileHash 计算文件哈希
func (m *UploadManager) calculateFileHash(reader io.Reader) (string, error) {
	// 这里实现文件哈希计算
	// 实际项目中可能需要重新读取文件
	return "", nil
}

// calculateChunkHash 计算分片哈希
func calculateChunkHash(data []byte) string {
	// 这里实现分片哈希计算
	return ""
}

// scanVirus 扫描病毒
func (m *UploadManager) scanVirus(ctx context.Context, fileInfo *FileInfo) error {
	if !m.config.EnableVirusScan {
		return nil
	}

	// 获取文件进行扫描
	reader, err := m.storage.Get(ctx, fileInfo.Path)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to get file for virus scan")
	}
	defer reader.Close()

	// 创建临时文件
	tempFile, err := os.CreateTemp("", "virus_scan_*.tmp")
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to create temp file for virus scan")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 复制文件内容
	if _, err := io.Copy(tempFile, reader); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to copy file for virus scan")
	}

	// 扫描病毒
	isClean, result, err := m.validator.ScanVirus(tempFile.Name())
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			"Virus scan failed")
	}

	// 更新文件信息
	fileInfo.IsVirusScanned = true
	fileInfo.IsVirusFree = isClean
	fileInfo.ScanResult = result

	if !isClean {
		// 如果检测到病毒，删除文件
		if err := m.storage.Delete(ctx, fileInfo.Path); err != nil {
			logger.Error("Failed to delete infected file",
				logger.String("file_id", fileInfo.ID),
				logger.String("file_path", fileInfo.Path),
				logger.ErrorField(err),
			)
		}

		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("File contains virus: %s", result))
	}

	logger.Info("Virus scan completed",
		logger.String("file_id", fileInfo.ID),
		logger.String("file_name", fileInfo.Name),
		logger.Bool("is_clean", isClean),
		logger.String("scan_result", result),
	)

	return nil
}

// Stop 停止上传管理器
func (m *UploadManager) Stop() {
	if m.chunkManager != nil {
		m.chunkManager.Stop()
	}
}

// InitUploadManager 初始化上传管理器
func InitUploadManager(cfg *config.Config) (*UploadManager, error) {
	// 将字符串类型的AllowedTypes转换为FileType
	var allowedTypes []FileType
	for _, t := range cfg.Upload.AllowedTypes {
		allowedTypes = append(allowedTypes, FileType(t))
	}

	uploadConfig := &UploadConfig{
		StorageType: StorageType(cfg.Upload.StorageType),
		StorageConfig: StorageConfig{
			Type:            StorageType(cfg.Upload.StorageType),
			BasePath:        cfg.Upload.BasePath,
			BaseURL:         cfg.Upload.BaseURL,
			CDNURL:          cfg.Upload.CDNURL,
			AccessKeyID:     cfg.Upload.AccessKeyID,
			AccessKeySecret: cfg.Upload.AccessKeySecret,
			Endpoint:        cfg.Upload.Endpoint,
			Bucket:          cfg.Upload.Bucket,
			Region:          cfg.Upload.Region,
			MaxFileSize:     cfg.Upload.MaxFileSize,
			UseHTTPS:        cfg.Upload.UseHTTPS,
			EnableCDN:       cfg.Upload.EnableCDN,
		},
		ValidationConfig: ValidationConfig{
			AllowedTypes:      allowedTypes,
			AllowedExtensions: cfg.Upload.AllowedExtensions,
			AllowedMimeTypes:  cfg.Upload.AllowedMimeTypes,
			MaxFileSize:       cfg.Upload.MaxFileSize,
			MinFileSize:       cfg.Upload.MinFileSize,
			MaxFileNameLength: cfg.Upload.MaxFileNameLength,
			ValidateVirus:     cfg.Upload.ValidateVirus,
			ScanTimeout:       cfg.Upload.ScanTimeout,
		},
		ChunkConfig: ChunkUploadConfig{
			Enabled:         cfg.Upload.ChunkEnabled,
			ChunkSize:       cfg.Upload.ChunkSize,
			MaxChunks:       cfg.Upload.MaxChunks,
			TempDir:         cfg.Upload.TempDir,
			CleanupInterval: cfg.Upload.CleanupInterval,
			MaxTempFileAge:  cfg.Upload.MaxTempFileAge,
			EnableResumable: cfg.Upload.EnableResumable,
		},
		EnableVirusScan: cfg.Upload.EnableVirusScan,
	}

	return NewUploadManager(uploadConfig)
}
