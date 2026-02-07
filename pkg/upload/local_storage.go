// file: pkg/upload/local_storage.go
package upload

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LocalStorage 本地存储
type LocalStorage struct {
	config *StorageConfig
}

// NewLocalStorage 创建本地存储
func NewLocalStorage(config *StorageConfig) (*LocalStorage, error) {
	// 创建存储目录
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to create storage directory")
	}

	// 创建临时目录
	tempDir := filepath.Join(config.BasePath, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to create temp directory")
	}

	// 创建缩略图目录
	thumbDir := filepath.Join(config.BasePath, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to create thumbnails directory")
	}

	logger.Info("Local storage initialized",
		logger.String("base_path", config.BasePath),
		logger.String("base_url", config.BaseURL),
	)

	return &LocalStorage{
		config: config,
	}, nil
}

// Type 返回存储类型
func (s *LocalStorage) Type() StorageType {
	return StorageTypeLocal
}

// Put 上传文件
func (s *LocalStorage) Put(ctx context.Context, file *FileInfo, src io.Reader) error {
	// 构建完整路径
	fullPath := s.getFullPath(file.Path)

	// 创建目录
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create directory: %s", dir))
	}

	// 创建文件
	dst, err := os.Create(fullPath)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create file: %s", fullPath))
	}
	defer dst.Close()

	// 计算文件哈希
	md5Hash := md5.New()
	sha256Hash := sha256.New()
	multiWriter := io.MultiWriter(dst, md5Hash, sha256Hash)

	// 复制文件内容
	written, err := io.Copy(multiWriter, src)
	if err != nil {
		// 删除已创建的文件
		os.Remove(fullPath)
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to write file content")
	}

	// 更新文件信息
	file.Size = written
	file.MD5 = hex.EncodeToString(md5Hash.Sum(nil))
	file.SHA256 = hex.EncodeToString(sha256Hash.Sum(nil))
	file.CreatedAt = time.Now()

	// 设置文件权限
	if err := os.Chmod(fullPath, 0644); err != nil {
		logger.Warn("Failed to set file permissions",
			logger.String("path", fullPath),
			logger.ErrorField(err),
		)
	}

	logger.Debug("File uploaded to local storage",
		logger.String("file_id", file.ID),
		logger.String("file_name", file.Name),
		logger.String("storage_name", file.StorageName),
		logger.String("path", file.Path),
		logger.Int64("size", written),
	)

	return nil
}

// PutMultipart 上传multipart文件
func (s *LocalStorage) PutMultipart(ctx context.Context, file *FileInfo, header *multipart.FileHeader) error {
	// 打开文件
	src, err := header.Open()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to open multipart file")
	}
	defer src.Close()

	// 调用Put方法
	return s.Put(ctx, file, src)
}

// Get 获取文件
func (s *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := s.getFullPath(path)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("File not found: %s", path))
		}
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to open file: %s", path))
	}

	return file, nil
}

// GetURL 获取文件访问URL
func (s *LocalStorage) GetURL(ctx context.Context, path string, public bool) (string, error) {
	// 检查文件是否存在
	fullPath := s.getFullPath(path)
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return "", errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("File not found: %s", path))
		}
		return "", errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to stat file: %s", path))
	}

	// 构建URL
	baseURL := strings.TrimSuffix(s.config.BaseURL, "/")
	fileURL := fmt.Sprintf("%s/%s", baseURL, path)

	// 如果需要CDN URL
	if s.config.EnableCDN && s.config.CDNURL != "" {
		cdnURL := strings.TrimSuffix(s.config.CDNURL, "/")
		fileURL = fmt.Sprintf("%s/%s", cdnURL, path)
	}

	return fileURL, nil
}

// Delete 删除文件
func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := s.getFullPath(path)

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("File not found: %s", path))
		}
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to stat file: %s", path))
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to delete file: %s", path))
	}

	// 尝试删除空目录
	dir := filepath.Dir(fullPath)
	s.cleanupEmptyDir(dir)

	logger.Debug("File deleted from local storage",
		logger.String("path", path),
		logger.String("full_path", fullPath),
	)

	return nil
}

// Stat 获取文件信息
func (s *LocalStorage) Stat(ctx context.Context, path string) (*FileInfo, error) {
	fullPath := s.getFullPath(path)

	// 获取文件信息
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("File not found: %s", path))
		}
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to stat file: %s", path))
	}

	// 计算文件哈希
	md5Hash, sha256Hash, err := s.calculateFileHashes(fullPath)
	if err != nil {
		return nil, err
	}

	// 获取MIME类型
	mimeType := getMimeType(fullPath)

	// 获取URL
	url, err := s.GetURL(ctx, path, true)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		ID:          generateFileID(),
		Name:        filepath.Base(path),
		StorageName: filepath.Base(path),
		Path:        path,
		URL:         url,
		Size:        fileInfo.Size(),
		MimeType:    mimeType,
		Extension:   strings.TrimPrefix(filepath.Ext(path), "."),
		MD5:         md5Hash,
		SHA256:      sha256Hash,
		StorageType: s.Type(),
		CreatedAt:   fileInfo.ModTime(),
	}, nil
}

// Exists 检查文件是否存在
func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := s.getFullPath(path)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to check file existence: %s", path))
	}

	return true, nil
}

// Copy 复制文件
func (s *LocalStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath := s.getFullPath(srcPath)
	dstFullPath := s.getFullPath(dstPath)

	// 检查源文件是否存在
	if _, err := os.Stat(srcFullPath); err != nil {
		if os.IsNotExist(err) {
			return errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("Source file not found: %s", srcPath))
		}
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to stat source file: %s", srcPath))
	}

	// 创建目标目录
	dir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create directory: %s", dir))
	}

	// 复制文件
	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to open source file: %s", srcPath))
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstFullPath)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create destination file: %s", dstPath))
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		// 删除已创建的目标文件
		os.Remove(dstFullPath)
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to copy file content")
	}

	// 设置文件权限
	if err := os.Chmod(dstFullPath, 0644); err != nil {
		logger.Warn("Failed to set file permissions",
			logger.String("path", dstFullPath),
			logger.ErrorField(err),
		)
	}

	logger.Debug("File copied in local storage",
		logger.String("src_path", srcPath),
		logger.String("dst_path", dstPath),
	)

	return nil
}

// Move 移动文件
func (s *LocalStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath := s.getFullPath(srcPath)
	dstFullPath := s.getFullPath(dstPath)

	// 检查源文件是否存在
	if _, err := os.Stat(srcFullPath); err != nil {
		if os.IsNotExist(err) {
			return errors.New(errors.CodeResourceNotFound,
				fmt.Sprintf("Source file not found: %s", srcPath))
		}
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to stat source file: %s", srcPath))
	}

	// 创建目标目录
	dir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to create directory: %s", dir))
	}

	// 移动文件
	if err := os.Rename(srcFullPath, dstFullPath); err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to move file: %s -> %s", srcPath, dstPath))
	}

	logger.Debug("File moved in local storage",
		logger.String("src_path", srcPath),
		logger.String("dst_path", dstPath),
	)

	return nil
}

// getFullPath 获取完整路径
func (s *LocalStorage) getFullPath(path string) string {
	// 清理路径，防止目录遍历攻击
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		cleanPath = strings.ReplaceAll(cleanPath, "..", "")
	}

	return filepath.Join(s.config.BasePath, cleanPath)
}

// cleanupEmptyDir 清理空目录
func (s *LocalStorage) cleanupEmptyDir(dir string) {
	// 检查是否为根目录
	basePath := filepath.Clean(s.config.BasePath)
	if dir == basePath || !strings.HasPrefix(dir, basePath) {
		return
	}

	// 尝试读取目录
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}

	// 如果目录为空，删除它并递归清理父目录
	if len(files) == 0 {
		if err := os.Remove(dir); err == nil {
			parentDir := filepath.Dir(dir)
			s.cleanupEmptyDir(parentDir)
		}
	}
}

// calculateFileHashes 计算文件哈希值
func (s *LocalStorage) calculateFileHashes(filePath string) (string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to open file for hashing: %s", filePath))
	}
	defer file.Close()

	md5Hash := md5.New()
	sha256Hash := sha256.New()

	// 使用多重写入器同时计算两个哈希
	multiWriter := io.MultiWriter(md5Hash, sha256Hash)

	if _, err := io.Copy(multiWriter, file); err != nil {
		return "", "", errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to read file for hashing: %s", filePath))
	}

	return hex.EncodeToString(md5Hash.Sum(nil)),
		hex.EncodeToString(sha256Hash.Sum(nil)), nil
}

// getMimeType 获取文件MIME类型
func getMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	// 常见MIME类型映射
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".zip":  "application/zip",
		".rar":  "application/x-rar-compressed",
		".7z":   "application/x-7z-compressed",
		".txt":  "text/plain",
		".csv":  "text/csv",
		".html": "text/html",
		".htm":  "text/html",
		".json": "application/json",
		".xml":  "application/xml",
		".mp3":  "audio/mpeg",
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}

	return "application/octet-stream"
}

// generateFileID 生成文件ID
func generateFileID() string {
	return uuid.New().String()
}
