// file: pkg/upload/service.go
package upload

import (
	"bytes"
	"context"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"mime/multipart"
	"os"
	"time"
)

// UploadService 上传服务
type UploadService struct {
	manager *UploadManager
}

// NewUploadService 创建上传服务
func NewUploadService(manager *UploadManager) *UploadService {
	return &UploadService{
		manager: manager,
	}
}

// UploadRequest 上传请求
type UploadRequest struct {
	File         *multipart.FileHeader  `json:"-"` // 文件
	UploaderID   string                 `json:"uploader_id,omitempty"`
	UploaderIP   string                 `json:"uploader_ip,omitempty"`
	IsPublic     bool                   `json:"is_public"`
	Category     string                 `json:"category,omitempty"`
	SubDirectory string                 `json:"sub_directory,omitempty"`
	CustomPath   string                 `json:"custom_path,omitempty"`
	ExpiredAt    *time.Time             `json:"expired_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// UploadResponse 上传响应
type UploadResponse struct {
	FileInfo *FileInfo `json:"file_info"`
	Message  string    `json:"message"`
}

// UploadSingle 上传单个文件
func (s *UploadService) UploadSingle(ctx context.Context, req *UploadRequest) (*UploadResponse, error) {
	if req.File == nil {
		return nil, errors.New(errors.CodeValidationFailed, "File is required")
	}

	// 验证文件
	if err := s.manager.validator.Validate(req.File); err != nil {
		return nil, err
	}

	// 转换为上传选项
	options := &UploadOptions{
		UploaderID:   req.UploaderID,
		UploaderIP:   req.UploaderIP,
		IsPublic:     req.IsPublic,
		Category:     req.Category,
		SubDirectory: req.SubDirectory,
		CustomPath:   req.CustomPath,
		ExpiredAt:    req.ExpiredAt,
		Metadata:     req.Metadata,
	}

	// 上传文件
	fileInfo, err := s.manager.Upload(ctx, req.File, options)
	if err != nil {
		return nil, err
	}

	return &UploadResponse{
		FileInfo: fileInfo,
		Message:  "File uploaded successfully",
	}, nil
}

// UploadMultiple 上传多个文件
func (s *UploadService) UploadMultiple(ctx context.Context, files []*multipart.FileHeader, options *UploadOptions) ([]*FileInfo, []error) {
	var fileInfos []*FileInfo
	var errors []error

	for _, file := range files {
		fileInfo, err := s.manager.Upload(ctx, file, options)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %v", file.Filename, err))
			continue
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	return fileInfos, errors
}

// ChunkUploadStart 开始分片上传
func (s *UploadService) ChunkUploadStart(ctx context.Context, req *ChunkUploadStartRequest) (*ChunkUploadStartResponse, error) {
	if s.manager.chunkManager == nil {
		return nil, errors.New(errors.CodeValidationFailed, "Chunk upload is disabled")
	}

	session, err := s.manager.chunkManager.CreateUploadSession(
		req.FileName,
		req.FileSize,
		req.FileHash,
		req.Metadata,
	)
	if err != nil {
		return nil, err
	}

	return &ChunkUploadStartResponse{
		UploadID:    session.UploadID,
		FileName:    session.FileName,
		FileSize:    session.FileSize,
		TotalChunks: session.TotalChunks,
		ChunkSize:   session.ChunkSize,
		CreatedAt:   session.CreatedAt,
	}, nil
}

// UploadChunk 上传分片
func (s *UploadService) UploadChunk(ctx context.Context, req *ChunkUploadRequest) error {
	if s.manager.chunkManager == nil {
		return errors.New(errors.CodeValidationFailed, "Chunk upload is disabled")
	}

	// 使用 bytes.NewReader 包装 []byte
	reader := bytes.NewReader(req.ChunkData)
	return s.manager.chunkManager.UploadChunk(
		req.UploadID,
		req.ChunkNumber,
		reader,
		req.ChunkSize,
		req.ChunkHash,
	)
}

// CompleteChunkUpload 完成分片上传
func (s *UploadService) CompleteChunkUpload(ctx context.Context, req *CompleteChunkUploadRequest) (*FileInfo, error) {
	if s.manager.chunkManager == nil {
		return nil, errors.New(errors.CodeValidationFailed, "Chunk upload is disabled")
	}

	// 完成上传
	session, err := s.manager.chunkManager.CompleteUpload(req.UploadID)
	if err != nil {
		return nil, err
	}

	// 获取合并后的文件
	mergedFilePath, ok := session.Metadata["merged_file_path"].(string)
	if !ok {
		return nil, errors.New(errors.CodeInternalError, "Failed to get merged file path")
	}

	// 打开合并后的文件
	mergedFile, err := os.Open(mergedFilePath)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to open merged file: %s", mergedFilePath))
	}
	defer mergedFile.Close()
	defer os.Remove(mergedFilePath)

	// 生成文件信息
	options := &UploadOptions{
		UploaderID:   req.UploaderID,
		UploaderIP:   req.UploaderIP,
		IsPublic:     req.IsPublic,
		Category:     req.Category,
		SubDirectory: req.SubDirectory,
		CustomPath:   req.CustomPath,
		Metadata:     session.Metadata,
	}

	fileInfo, err := s.manager.generateFileInfo(&multipart.FileHeader{
		Filename: session.FileName,
		Size:     session.FileSize,
		Header:   make(map[string][]string),
	}, options)
	if err != nil {
		return nil, err
	}

	// 上传到最终存储
	if err := s.manager.storage.Put(ctx, fileInfo, mergedFile); err != nil {
		return nil, err
	}

	// 清理上传会话
	s.manager.chunkManager.AbortUpload(req.UploadID)

	logger.Info("Chunk upload completed",
		logger.String("upload_id", req.UploadID),
		logger.String("file_name", session.FileName),
		logger.Int64("file_size", session.FileSize),
		logger.String("file_path", fileInfo.Path),
	)

	return fileInfo, nil
}

// GetUploadProgress 获取上传进度
func (s *UploadService) GetUploadProgress(ctx context.Context, uploadID string) (map[string]interface{}, error) {
	if s.manager.chunkManager == nil {
		return nil, errors.New(errors.CodeValidationFailed, "Chunk upload is disabled")
	}

	return s.manager.chunkManager.GetUploadProgress(uploadID)
}

// GetFileInfo 获取文件信息
func (s *UploadService) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	return s.manager.GetFileInfo(ctx, path)
}

// GetFileURL 获取文件URL
func (s *UploadService) GetFileURL(ctx context.Context, path string, public bool) (string, error) {
	return s.manager.GetFileURL(ctx, path, public)
}

// DeleteFile 删除文件
func (s *UploadService) DeleteFile(ctx context.Context, path string) error {
	return s.manager.DeleteFile(ctx, path)
}

// CopyFile 复制文件
func (s *UploadService) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	return s.manager.CopyFile(ctx, srcPath, dstPath)
}

// MoveFile 移动文件
func (s *UploadService) MoveFile(ctx context.Context, srcPath, dstPath string) error {
	return s.manager.MoveFile(ctx, srcPath, dstPath)
}

// FileExists 检查文件是否存在
func (s *UploadService) FileExists(ctx context.Context, path string) (bool, error) {
	return s.manager.FileExists(ctx, path)
}

// 请求/响应结构体
type ChunkUploadStartRequest struct {
	FileName     string                 `json:"file_name"`
	FileSize     int64                  `json:"file_size"`
	FileHash     string                 `json:"file_hash,omitempty"`
	Category     string                 `json:"category,omitempty"`
	SubDirectory string                 `json:"sub_directory,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type ChunkUploadStartResponse struct {
	UploadID    string    `json:"upload_id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	TotalChunks int       `json:"total_chunks"`
	ChunkSize   int64     `json:"chunk_size"`
	CreatedAt   time.Time `json:"created_at"`
}

type ChunkUploadRequest struct {
	UploadID    string `json:"upload_id"`
	ChunkNumber int    `json:"chunk_number"`
	ChunkData   []byte `json:"chunk_data"`
	ChunkSize   int64  `json:"chunk_size"`
	ChunkHash   string `json:"chunk_hash,omitempty"`
}

type CompleteChunkUploadRequest struct {
	UploadID     string `json:"upload_id"`
	UploaderID   string `json:"uploader_id,omitempty"`
	UploaderIP   string `json:"uploader_ip,omitempty"`
	IsPublic     bool   `json:"is_public"`
	Category     string `json:"category,omitempty"`
	SubDirectory string `json:"sub_directory,omitempty"`
	CustomPath   string `json:"custom_path,omitempty"`
}
