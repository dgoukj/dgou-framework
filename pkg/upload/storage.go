// file: pkg/upload/storage.go
package upload

import (
	"context"
	"io"
	"mime/multipart"
	"time"
)

// StorageType 存储类型
type StorageType string

const (
	StorageTypeLocal StorageType = "local" // 本地存储
	StorageTypeOSS   StorageType = "oss"   // OSS存储
)

// FileInfo 文件信息
type FileInfo struct {
	ID             string                 `json:"id"`                    // 文件ID
	Name           string                 `json:"name"`                  // 原始文件名
	StorageName    string                 `json:"storage_name"`          // 存储文件名
	Path           string                 `json:"path"`                  // 存储路径
	URL            string                 `json:"url"`                   // 访问URL
	FullURL        string                 `json:"full_url"`              // 完整访问URL（带CDN）
	Size           int64                  `json:"size"`                  // 文件大小（字节）
	MimeType       string                 `json:"mime_type"`             // MIME类型
	Extension      string                 `json:"extension"`             // 文件扩展名
	MD5            string                 `json:"md5,omitempty"`         // MD5值
	SHA256         string                 `json:"sha256,omitempty"`      // SHA256值
	StorageType    StorageType            `json:"storage_type"`          // 存储类型
	UploaderID     string                 `json:"uploader_id,omitempty"` // 上传者ID
	UploaderIP     string                 `json:"uploader_ip,omitempty"` // 上传者IP
	IsPublic       bool                   `json:"is_public"`             // 是否公开
	IsVirusScanned bool                   `json:"is_virus_scanned"`      // 是否已扫描病毒
	IsVirusFree    bool                   `json:"is_virus_free"`         // 是否无病毒
	ScanResult     string                 `json:"scan_result,omitempty"` // 扫描结果
	CreatedAt      time.Time              `json:"created_at"`            // 创建时间
	ExpiredAt      *time.Time             `json:"expired_at,omitempty"`  // 过期时间
	Metadata       map[string]interface{} `json:"metadata,omitempty"`    // 元数据
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type            StorageType `mapstructure:"type"`              // 存储类型
	BasePath        string      `mapstructure:"base_path"`         // 基础路径（本地存储）
	BaseURL         string      `mapstructure:"base_url"`          // 基础URL
	CDNURL          string      `mapstructure:"cdn_url"`           // CDN URL
	AccessKeyID     string      `mapstructure:"access_key_id"`     // OSS AccessKey ID
	AccessKeySecret string      `mapstructure:"access_key_secret"` // OSS AccessKey Secret
	Endpoint        string      `mapstructure:"endpoint"`          // OSS Endpoint
	Bucket          string      `mapstructure:"bucket"`            // OSS Bucket
	Region          string      `mapstructure:"region"`            // OSS Region
	MaxFileSize     int64       `mapstructure:"max_file_size"`     // 最大文件大小（字节）
	UseHTTPS        bool        `mapstructure:"use_https"`         // 是否使用HTTPS
	EnableCDN       bool        `mapstructure:"enable_cdn"`        // 是否启用CDN
}

// Storage 存储接口
type Storage interface {
	// Type 返回存储类型
	Type() StorageType

	// Put 上传文件
	Put(ctx context.Context, file *FileInfo, src io.Reader) error

	// PutMultipart 上传multipart文件
	PutMultipart(ctx context.Context, file *FileInfo, header *multipart.FileHeader) error

	// Get 获取文件
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// GetURL 获取文件访问URL
	GetURL(ctx context.Context, path string, public bool) (string, error)

	// Delete 删除文件
	Delete(ctx context.Context, path string) error

	// Stat 获取文件信息
	Stat(ctx context.Context, path string) (*FileInfo, error)

	// Exists 检查文件是否存在
	Exists(ctx context.Context, path string) (bool, error)

	// Copy 复制文件
	Copy(ctx context.Context, srcPath, dstPath string) error

	// Move 移动文件
	Move(ctx context.Context, srcPath, dstPath string) error
}
