// file: pkg/upload/chunk_upload.go
package upload

import (
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ChunkUploadConfig 分片上传配置
type ChunkUploadConfig struct {
	Enabled         bool          `mapstructure:"enabled"`           // 是否启用分片上传
	ChunkSize       int64         `mapstructure:"chunk_size"`        // 分片大小（字节）
	MaxChunks       int           `mapstructure:"max_chunks"`        // 最大分片数
	TempDir         string        `mapstructure:"temp_dir"`          // 临时目录
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`  // 清理间隔
	MaxTempFileAge  time.Duration `mapstructure:"max_temp_file_age"` // 临时文件最大年龄
	EnableResumable bool          `mapstructure:"enable_resumable"`  // 是否启用断点续传
}

// ChunkInfo 分片信息
type ChunkInfo struct {
	UploadID     string    `json:"upload_id"`      // 上传ID
	ChunkNumber  int       `json:"chunk_number"`   // 分片序号
	TotalChunks  int       `json:"total_chunks"`   // 总分片数
	ChunkSize    int64     `json:"chunk_size"`     // 分片大小
	FileSize     int64     `json:"file_size"`      // 文件总大小
	FileName     string    `json:"file_name"`      // 文件名
	FileHash     string    `json:"file_hash"`      // 文件哈希
	ChunkHash    string    `json:"chunk_hash"`     // 分片哈希
	UploadedAt   time.Time `json:"uploaded_at"`    // 上传时间
	Completed    bool      `json:"completed"`      // 是否完成
	TempFilePath string    `json:"temp_file_path"` // 临时文件路径
}

// ChunkUploadManager 分片上传管理器
type ChunkUploadManager struct {
	config    *ChunkUploadConfig
	tempDir   string
	uploads   map[string]*ChunkUploadSession
	mu        sync.RWMutex
	cleanupCh chan struct{}
}

// ChunkUploadSession 分片上传会话
type ChunkUploadSession struct {
	UploadID    string                 `json:"upload_id"`
	FileName    string                 `json:"file_name"`
	FileSize    int64                  `json:"file_size"`
	TotalChunks int                    `json:"total_chunks"`
	ChunkSize   int64                  `json:"chunk_size"`
	FileHash    string                 `json:"file_hash"`
	Chunks      map[int]*ChunkInfo     `json:"chunks"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Completed   bool                   `json:"completed"`
	mu          sync.RWMutex           `json:"-"`
}

// NewChunkUploadManager 创建分片上传管理器
func NewChunkUploadManager(config *ChunkUploadConfig) (*ChunkUploadManager, error) {
	if config == nil {
		config = &ChunkUploadConfig{
			Enabled:         true,
			ChunkSize:       5 * 1024 * 1024, // 5MB
			MaxChunks:       1000,
			TempDir:         "/tmp/upload_chunks",
			CleanupInterval: 30 * time.Minute,
			MaxTempFileAge:  24 * time.Hour,
			EnableResumable: true,
		}
	}

	// 创建临时目录
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to create chunk upload temp directory")
	}

	manager := &ChunkUploadManager{
		config:    config,
		tempDir:   config.TempDir,
		uploads:   make(map[string]*ChunkUploadSession),
		cleanupCh: make(chan struct{}),
	}

	// 启动清理协程
	if config.CleanupInterval > 0 {
		go manager.startCleanup()
	}

	logger.Info("Chunk upload manager initialized",
		logger.Bool("enabled", config.Enabled),
		logger.Int64("chunk_size", config.ChunkSize),
		logger.String("temp_dir", config.TempDir),
	)

	return manager, nil
}

// CreateUploadSession 创建上传会话
func (m *ChunkUploadManager) CreateUploadSession(filename string, fileSize int64, fileHash string, metadata map[string]interface{}) (*ChunkUploadSession, error) {
	if !m.config.Enabled {
		return nil, errors.New(errors.CodeValidationFailed, "Chunk upload is disabled")
	}

	// 计算分片数
	totalChunks := int((fileSize + m.config.ChunkSize - 1) / m.config.ChunkSize)
	if totalChunks > m.config.MaxChunks {
		return nil, errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Too many chunks: %d, maximum allowed: %d", totalChunks, m.config.MaxChunks))
	}

	uploadID := generateUploadID()

	session := &ChunkUploadSession{
		UploadID:    uploadID,
		FileName:    filename,
		FileSize:    fileSize,
		TotalChunks: totalChunks,
		ChunkSize:   m.config.ChunkSize,
		FileHash:    fileHash,
		Chunks:      make(map[int]*ChunkInfo),
		Metadata:    metadata,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 保存会话
	m.mu.Lock()
	m.uploads[uploadID] = session
	m.mu.Unlock()

	// 保存会话信息到文件
	if err := m.saveSession(session); err != nil {
		logger.Warn("Failed to save session to file",
			logger.String("upload_id", uploadID),
			logger.ErrorField(err),
		)
	}

	logger.Debug("Upload session created",
		logger.String("upload_id", uploadID),
		logger.String("file_name", filename),
		logger.Int64("file_size", fileSize),
		logger.Int("total_chunks", totalChunks),
	)

	return session, nil
}

// GetUploadSession 获取上传会话
func (m *ChunkUploadManager) GetUploadSession(uploadID string) (*ChunkUploadSession, error) {
	m.mu.RLock()
	sess, exists := m.uploads[uploadID] // 修改变量名避免冲突
	m.mu.RUnlock()

	if !exists {
		// 尝试从文件加载
		session, err := m.loadSession(uploadID)
		if err != nil {
			return nil, errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("Upload session not found: %s", uploadID))
		}

		// 添加到内存
		m.mu.Lock()
		m.uploads[uploadID] = session
		m.mu.Unlock()

		return session, nil
	}

	return sess, nil
}

// UploadChunk 上传分片
func (m *ChunkUploadManager) UploadChunk(uploadID string, chunkNumber int, chunkData io.Reader, chunkSize int64, chunkHash string) error {
	session, err := m.GetUploadSession(uploadID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 检查分片是否已上传
	if _, exists := session.Chunks[chunkNumber]; exists {
		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Chunk %d already uploaded", chunkNumber))
	}

	// 检查分片序号
	if chunkNumber < 1 || chunkNumber > session.TotalChunks {
		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Invalid chunk number: %d, expected 1-%d", chunkNumber, session.TotalChunks))
	}

	// 计算分片偏移量
	offset := int64(chunkNumber-1) * session.ChunkSize
	remaining := session.FileSize - offset
	expectedChunkSize := session.ChunkSize
	if remaining < session.ChunkSize {
		expectedChunkSize = remaining
	}

	// 检查分片大小
	if chunkSize != expectedChunkSize {
		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Invalid chunk size: %d, expected: %d", chunkSize, expectedChunkSize))
	}

	// 创建临时文件
	tempDir := filepath.Join(m.tempDir, uploadID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create temp directory: %s", tempDir))
	}

	tempFilePath := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.tmp", chunkNumber))
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create temp file: %s", tempFilePath))
	}
	defer tempFile.Close()

	// 保存分片数据
	if _, err := io.Copy(tempFile, chunkData); err != nil {
		os.Remove(tempFilePath)
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to write chunk data: %d", chunkNumber))
	}

	// 记录分片信息
	chunkInfo := &ChunkInfo{
		UploadID:     uploadID,
		ChunkNumber:  chunkNumber,
		TotalChunks:  session.TotalChunks,
		ChunkSize:    chunkSize,
		FileSize:     session.FileSize,
		FileName:     session.FileName,
		FileHash:     session.FileHash,
		ChunkHash:    chunkHash,
		UploadedAt:   time.Now(),
		Completed:    false,
		TempFilePath: tempFilePath,
	}

	session.Chunks[chunkNumber] = chunkInfo
	session.UpdatedAt = time.Now()

	// 保存会话信息
	m.saveSession(session)

	logger.Debug("Chunk uploaded",
		logger.String("upload_id", uploadID),
		logger.Int("chunk_number", chunkNumber),
		logger.Int64("chunk_size", chunkSize),
		logger.Int("uploaded_chunks", len(session.Chunks)),
		logger.Int("total_chunks", session.TotalChunks),
	)

	return nil
}

// CompleteUpload 完成上传
func (m *ChunkUploadManager) CompleteUpload(uploadID string) (*ChunkUploadSession, error) {
	session, err := m.GetUploadSession(uploadID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 检查是否所有分片都已上传
	if len(session.Chunks) != session.TotalChunks {
		return nil, errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Not all chunks uploaded: %d/%d", len(session.Chunks), session.TotalChunks))
	}

	// 检查分片序号连续性
	for i := 1; i <= session.TotalChunks; i++ {
		if _, exists := session.Chunks[i]; !exists {
			return nil, errors.New(errors.CodeValidationFailed,
				fmt.Sprintf("Missing chunk: %d", i))
		}
	}

	session.Completed = true
	session.UpdatedAt = time.Now()

	// 保存会话信息
	m.saveSession(session)

	// 合并分片
	mergedFilePath, err := m.mergeChunks(session)
	if err != nil {
		return nil, err
	}

	// 更新会话中的临时文件路径
	for _, chunk := range session.Chunks {
		chunk.Completed = true
	}

	// 添加合并后的文件路径到元数据
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["merged_file_path"] = mergedFilePath

	logger.Info("Upload completed",
		logger.String("upload_id", uploadID),
		logger.String("file_name", session.FileName),
		logger.Int64("file_size", session.FileSize),
		logger.String("merged_file_path", mergedFilePath),
	)

	return session, nil
}

// GetUploadProgress 获取上传进度
func (m *ChunkUploadManager) GetUploadProgress(uploadID string) (map[string]interface{}, error) {
	session, err := m.GetUploadSession(uploadID)
	if err != nil {
		return nil, err
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	progress := map[string]interface{}{
		"upload_id":       session.UploadID,
		"file_name":       session.FileName,
		"file_size":       session.FileSize,
		"total_chunks":    session.TotalChunks,
		"uploaded_chunks": len(session.Chunks),
		"chunk_size":      session.ChunkSize,
		"completed":       session.Completed,
		"progress":        float64(len(session.Chunks)) / float64(session.TotalChunks) * 100,
		"created_at":      session.CreatedAt,
		"updated_at":      session.UpdatedAt,
	}

	// 上传的分片列表
	uploadedChunks := make([]int, 0, len(session.Chunks))
	for chunkNumber := range session.Chunks {
		uploadedChunks = append(uploadedChunks, chunkNumber)
	}
	progress["uploaded_chunk_numbers"] = uploadedChunks

	return progress, nil
}

// AbortUpload 中止上传
func (m *ChunkUploadManager) AbortUpload(uploadID string) error {
	_, err := m.GetUploadSession(uploadID)
	if err != nil {
		return err
	}

	// 清理临时文件
	tempDir := filepath.Join(m.tempDir, uploadID)
	if err := os.RemoveAll(tempDir); err != nil {
		logger.Warn("Failed to cleanup temp directory",
			logger.String("upload_id", uploadID),
			logger.String("temp_dir", tempDir),
			logger.ErrorField(err),
		)
	}

	// 删除会话文件
	sessionFile := filepath.Join(m.tempDir, fmt.Sprintf("%s.session", uploadID))
	os.Remove(sessionFile)

	// 从内存中删除
	m.mu.Lock()
	delete(m.uploads, uploadID)
	m.mu.Unlock()

	logger.Debug("Upload aborted",
		logger.String("upload_id", uploadID),
	)

	return nil
}

// mergeChunks 合并分片
func (m *ChunkUploadManager) mergeChunks(session *ChunkUploadSession) (string, error) {
	// 创建合并后的文件
	mergedDir := filepath.Join(m.tempDir, "merged")
	if err := os.MkdirAll(mergedDir, 0755); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError,
			"Failed to create merged directory")
	}

	mergedFilePath := filepath.Join(mergedDir, fmt.Sprintf("%s_%s", session.UploadID, session.FileName))
	mergedFile, err := os.Create(mergedFilePath)
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create merged file: %s", mergedFilePath))
	}
	defer mergedFile.Close()

	// 按顺序合并分片
	for i := 1; i <= session.TotalChunks; i++ {
		chunk := session.Chunks[i]

		chunkFile, err := os.Open(chunk.TempFilePath)
		if err != nil {
			return "", errors.Wrap(err, errors.CodeInternalError,
				fmt.Sprintf("Failed to open chunk file: %s", chunk.TempFilePath))
		}

		if _, err := io.Copy(mergedFile, chunkFile); err != nil {
			chunkFile.Close()
			return "", errors.Wrap(err, errors.CodeInternalError,
				fmt.Sprintf("Failed to merge chunk: %d", i))
		}

		chunkFile.Close()

		// 删除临时分片文件
		if err := os.Remove(chunk.TempFilePath); err != nil {
			logger.Warn("Failed to delete chunk file",
				logger.String("upload_id", session.UploadID),
				logger.Int("chunk_number", i),
				logger.String("chunk_file", chunk.TempFilePath),
				logger.ErrorField(err),
			)
		}
	}

	// 清理会话目录
	tempDir := filepath.Join(m.tempDir, session.UploadID)
	if err := os.RemoveAll(tempDir); err != nil {
		logger.Warn("Failed to cleanup session directory",
			logger.String("upload_id", session.UploadID),
			logger.String("temp_dir", tempDir),
			logger.ErrorField(err),
		)
	}

	return mergedFilePath, nil
}

// saveSession 保存会话到文件
func (m *ChunkUploadManager) saveSession(session *ChunkUploadSession) error {
	sessionFile := filepath.Join(m.tempDir, fmt.Sprintf("%s.session", session.UploadID))

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to marshal session data")
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to write session file: %s", sessionFile))
	}

	return nil
}

// loadSession 从文件加载会话
func (m *ChunkUploadManager) loadSession(uploadID string) (*ChunkUploadSession, error) {
	sessionFile := filepath.Join(m.tempDir, fmt.Sprintf("%s.session", uploadID))

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("Session file not found: %s", sessionFile))
		}
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to read session file: %s", sessionFile))
	}

	var session ChunkUploadSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to unmarshal session data")
	}

	// 重新初始化互斥锁
	session.mu = sync.RWMutex{}

	return &session, nil
}

// startCleanup 启动清理协程
func (m *ChunkUploadManager) startCleanup() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpired()
		case <-m.cleanupCh:
			return
		}
	}
}

// cleanupExpired 清理过期数据
func (m *ChunkUploadManager) cleanupExpired() {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 清理过期的会话
	for uploadID, session := range m.uploads {
		// 检查会话是否过期（24小时未更新）
		if now.Sub(session.UpdatedAt) > m.config.MaxTempFileAge {
			// 中止上传
			m.abortSession(uploadID, session)
			delete(m.uploads, uploadID)
		}
	}

	// 清理临时目录
	entries, err := os.ReadDir(m.tempDir)
	if err != nil {
		logger.Error("Failed to read temp directory",
			logger.String("temp_dir", m.tempDir),
			logger.ErrorField(err),
		)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(m.tempDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// 检查目录是否过期
			if now.Sub(info.ModTime()) > m.config.MaxTempFileAge {
				if err := os.RemoveAll(dirPath); err != nil {
					logger.Warn("Failed to cleanup expired directory",
						logger.String("dir_path", dirPath),
						logger.ErrorField(err),
					)
				} else {
					logger.Debug("Cleaned up expired directory",
						logger.String("dir_path", dirPath),
					)
				}
			}
		}
	}
}

// abortSession 中止会话并清理资源
func (m *ChunkUploadManager) abortSession(uploadID string, session *ChunkUploadSession) {
	// 清理临时文件
	tempDir := filepath.Join(m.tempDir, uploadID)
	if err := os.RemoveAll(tempDir); err != nil {
		logger.Warn("Failed to cleanup temp directory",
			logger.String("upload_id", uploadID),
			logger.String("temp_dir", tempDir),
			logger.ErrorField(err),
		)
	}

	// 删除会话文件
	sessionFile := filepath.Join(m.tempDir, fmt.Sprintf("%s.session", uploadID))
	os.Remove(sessionFile)

	logger.Debug("Cleaned up expired session",
		logger.String("upload_id", uploadID),
		logger.String("file_name", session.FileName),
		logger.Duration("age", time.Since(session.UpdatedAt)),
	)
}

// Stop 停止清理协程
func (m *ChunkUploadManager) Stop() {
	close(m.cleanupCh)
}

// generateUploadID 生成上传ID
func generateUploadID() string {
	return fmt.Sprintf("upload_%d_%s", time.Now().UnixNano(), strings.ReplaceAll(uuid.New().String(), "-", ""))
}
