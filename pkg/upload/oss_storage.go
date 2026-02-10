// file: pkg/upload/oss_storage.go
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
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSStorage OSS存储
type OSSStorage struct {
	config *StorageConfig
	client *oss.Client
	bucket *oss.Bucket
}

// NewOSSStorage 创建OSS存储
func NewOSSStorage(config *StorageConfig) (*OSSStorage, error) {
	if config.AccessKeyID == "" || config.AccessKeySecret == "" {
		return nil, errors.New(errors.CodeValidationFailed,
			"OSS access key ID and secret are required")
	}

	if config.Endpoint == "" {
		return nil, errors.New(errors.CodeValidationFailed,
			"OSS endpoint is required")
	}

	if config.Bucket == "" {
		return nil, errors.New(errors.CodeValidationFailed,
			"OSS bucket is required")
	}

	// 创建OSS客户端
	client, err := oss.New(config.Endpoint, config.AccessKeyID, config.AccessKeySecret)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to create OSS client")
	}

	// 检查Bucket是否存在 - 使用 GetBucketInfo
	_, err = client.GetBucketInfo(config.Bucket)
	if err != nil {
		if ossErr, ok := err.(oss.ServiceError); ok {
			if ossErr.StatusCode == 404 {
				return nil, errors.New(errors.CodeValidationFailed,
					fmt.Sprintf("OSS bucket does not exist: %s", config.Bucket))
			}
		}
		return nil, errors.Wrap(err, errors.CodeInternalError,
			"Failed to check bucket existence")
	}

	// 获取Bucket
	bucket, err := client.Bucket(config.Bucket)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to get OSS bucket: %s", config.Bucket))
	}

	logger.Info("OSS storage initialized",
		logger.String("endpoint", config.Endpoint),
		logger.String("bucket", config.Bucket),
		logger.String("region", config.Region),
	)

	return &OSSStorage{
		config: config,
		client: client,
		bucket: bucket,
	}, nil
}

// Type 返回存储类型
func (s *OSSStorage) Type() StorageType {
	return StorageTypeOSS
}

// Put 上传文件到OSS
func (s *OSSStorage) Put(ctx context.Context, file *FileInfo, src io.Reader) error {
	options := []oss.Option{
		oss.ContentType(file.MimeType),
		oss.ContentLength(file.Size),
	}

	// 计算文件哈希
	md5Hash := md5.New()
	sha256Hash := sha256.New()

	// 创建多重读取器
	var reader io.Reader = src
	if file.Size > 0 {
		// 如果知道文件大小，使用TeeReader计算哈希
		reader = io.TeeReader(src, io.MultiWriter(md5Hash, sha256Hash))
	}

	// 上传到OSS
	err := s.bucket.PutObject(file.Path, reader, options...)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to upload file to OSS: %s", file.Path))
	}

	// 更新文件信息
	file.MD5 = hex.EncodeToString(md5Hash.Sum(nil))
	file.SHA256 = hex.EncodeToString(sha256Hash.Sum(nil))
	file.CreatedAt = time.Now()

	// 获取文件URL
	url, err := s.GetURL(ctx, file.Path, file.IsPublic)
	if err == nil {
		file.URL = url
	}

	logger.Debug("File uploaded to OSS",
		logger.String("file_id", file.ID),
		logger.String("file_name", file.Name),
		logger.String("storage_name", file.StorageName),
		logger.String("path", file.Path),
		logger.Int64("size", file.Size),
	)

	return nil
}

// PutMultipart 上传multipart文件到OSS
func (s *OSSStorage) PutMultipart(ctx context.Context, file *FileInfo, header *multipart.FileHeader) error {
	// 打开文件
	src, err := header.Open()
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to open multipart file")
	}
	defer src.Close()

	file.Size = header.Size

	// 调用Put方法
	return s.Put(ctx, file, src)
}

// Get 从OSS获取文件
func (s *OSSStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	body, err := s.bucket.GetObject(path)
	if err != nil {
		if ossErr, ok := err.(oss.ServiceError); ok {
			if ossErr.StatusCode == 404 {
				return nil, errors.New(errors.CodeResourceNotFound,
					fmt.Sprintf("File not found in OSS: %s", path))
			}
		}
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to get file from OSS: %s", path))
	}

	return body, nil
}

// GetURL 获取OSS文件访问URL
func (s *OSSStorage) GetURL(ctx context.Context, path string, public bool) (string, error) {
	// 检查文件是否存在
	exist, err := s.bucket.IsObjectExist(path)
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to check file existence in OSS: %s", path))
	}

	if !exist {
		return "", errors.New(errors.CodeResourceNotFound,
			fmt.Sprintf("File not found in OSS: %s", path))
	}

	var fileURL string

	if public {
		// 公开文件，直接返回URL
		protocol := "http"
		if s.config.UseHTTPS {
			protocol = "https"
		}

		domain := s.config.Endpoint
		if s.config.EnableCDN && s.config.CDNURL != "" {
			domain = s.config.CDNURL
		}

		fileURL = fmt.Sprintf("%s://%s/%s", protocol, domain, path)
	} else {
		// 私有文件，生成签名URL
		signedURL, err := s.bucket.SignURL(path, oss.HTTPGet, 3600) // 1小时有效
		if err != nil {
			return "", errors.Wrap(err, errors.CodeInternalError,
				fmt.Sprintf("Failed to generate signed URL: %s", path))
		}
		fileURL = signedURL
	}

	return fileURL, nil
}

// Delete 从OSS删除文件
func (s *OSSStorage) Delete(ctx context.Context, path string) error {
	err := s.bucket.DeleteObject(path)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to delete file from OSS: %s", path))
	}

	logger.Debug("File deleted from OSS",
		logger.String("path", path),
	)

	return nil
}

// Stat 获取OSS文件信息
func (s *OSSStorage) Stat(ctx context.Context, path string) (*FileInfo, error) {
	// 获取文件元数据
	props, err := s.bucket.GetObjectMeta(path)
	if err != nil {
		if ossErr, ok := err.(oss.ServiceError); ok {
			if ossErr.StatusCode == 404 {
				return nil, errors.New(errors.CodeResourceNotFound,
					fmt.Sprintf("File not found in OSS: %s", path))
			}
		}
		return nil, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to get file metadata from OSS: %s", path))
	}

	// 获取文件大小
	size := int64(0)
	if contentLength, ok := props["Content-Length"]; ok && len(contentLength) > 0 {
		fmt.Sscanf(contentLength[0], "%d", &size)
	}

	// 获取MIME类型
	mimeType := ""
	if contentType, ok := props["Content-Type"]; ok && len(contentType) > 0 {
		mimeType = contentType[0]
	}

	// 获取ETag（通常是MD5）
	md5Hash := ""
	if eTag, ok := props["ETag"]; ok && len(eTag) > 0 {
		md5Hash = strings.Trim(eTag[0], "\"")
	}

	// 获取最后修改时间
	var createdAt time.Time
	if lastModified, ok := props["Last-Modified"]; ok && len(lastModified) > 0 {
		parsedTime, err := time.Parse(time.RFC1123, lastModified[0])
		if err == nil {
			createdAt = parsedTime
		}
	}

	if createdAt.IsZero() {
		createdAt = time.Now()
	}

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
		Size:        size,
		MimeType:    mimeType,
		Extension:   strings.TrimPrefix(filepath.Ext(path), "."),
		MD5:         md5Hash,
		StorageType: StorageTypeOSS,
		CreatedAt:   createdAt,
	}, nil
}

// Exists 检查OSS文件是否存在
func (s *OSSStorage) Exists(ctx context.Context, path string) (bool, error) {
	exist, err := s.bucket.IsObjectExist(path)
	if err != nil {
		return false, errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to check file existence in OSS: %s", path))
	}

	return exist, nil
}

// Copy 在OSS中复制文件
func (s *OSSStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	_, err := s.bucket.CopyObject(srcPath, dstPath)
	if err != nil {
		if ossErr, ok := err.(oss.ServiceError); ok {
			if ossErr.StatusCode == 404 {
				return errors.New(errors.CodeResourceNotFound,
					fmt.Sprintf("Source file not found in OSS: %s", srcPath))
			}
		}
		return errors.Wrap(err, errors.CodeInternalError,
			fmt.Sprintf("Failed to copy file in OSS: %s -> %s", srcPath, dstPath))
	}

	logger.Debug("File copied in OSS",
		logger.String("src_path", srcPath),
		logger.String("dst_path", dstPath),
	)

	return nil
}

// Move 在OSS中移动文件
func (s *OSSStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	// 先复制
	if err := s.Copy(ctx, srcPath, dstPath); err != nil {
		return err
	}

	// 再删除源文件
	if err := s.Delete(ctx, srcPath); err != nil {
		// 如果删除失败，记录错误但继续
		logger.Error("Failed to delete source file after move",
			logger.String("src_path", srcPath),
			logger.String("dst_path", dstPath),
			logger.ErrorField(err),
		)
	}

	logger.Debug("File moved in OSS",
		logger.String("src_path", srcPath),
		logger.String("dst_path", dstPath),
	)

	return nil
}
