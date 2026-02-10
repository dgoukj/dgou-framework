// file: pkg/upload/storage.go
package upload

import (
	"context"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StorageType 存储类型
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeOSS   StorageType = "oss"
)

// FileType 文件类型
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeDocument FileType = "document"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeOther    FileType = "other"
)

// StorageConfig 存储配置
type StorageConfig struct {
	Type            StorageType `mapstructure:"type"`
	BasePath        string      `mapstructure:"base_path"`
	BaseURL         string      `mapstructure:"base_url"`
	CDNURL          string      `mapstructure:"cdn_url"`
	AccessKeyID     string      `mapstructure:"access_key_id"`
	AccessKeySecret string      `mapstructure:"access_key_secret"`
	Endpoint        string      `mapstructure:"endpoint"`
	Bucket          string      `mapstructure:"bucket"`
	Region          string      `mapstructure:"region"`
	MaxFileSize     int64       `mapstructure:"max_file_size"`
	UseHTTPS        bool        `mapstructure:"use_https"`
	EnableCDN       bool        `mapstructure:"enable_cdn"`
}

// ValidationConfig 验证配置
type ValidationConfig struct {
	AllowedTypes      []FileType `mapstructure:"allowed_types"`
	AllowedExtensions []string   `mapstructure:"allowed_extensions"`
	AllowedMimeTypes  []string   `mapstructure:"allowed_mime_types"`
	MaxFileSize       int64      `mapstructure:"max_file_size"`
	MinFileSize       int64      `mapstructure:"min_file_size"`
	MaxFileNameLength int        `mapstructure:"max_file_name_length"`
	ValidateVirus     bool       `mapstructure:"validate_virus"`
	ScanTimeout       int        `mapstructure:"scan_timeout"`
}

// FileInfo 文件信息
type FileInfo struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	StorageName    string                 `json:"storage_name"`
	Path           string                 `json:"path"`
	URL            string                 `json:"url"`
	FullURL        string                 `json:"full_url"`
	Size           int64                  `json:"size"`
	MimeType       string                 `json:"mime_type"`
	Extension      string                 `json:"extension"`
	MD5            string                 `json:"md5,omitempty"`
	SHA256         string                 `json:"sha256,omitempty"`
	StorageType    StorageType            `json:"storage_type"`
	UploaderID     string                 `json:"uploader_id,omitempty"`
	UploaderIP     string                 `json:"uploader_ip,omitempty"`
	IsPublic       bool                   `json:"is_public"`
	IsVirusScanned bool                   `json:"is_virus_scanned"`
	IsVirusFree    bool                   `json:"is_virus_free"`
	ScanResult     string                 `json:"scan_result,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	ExpiredAt      *time.Time             `json:"expired_at,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
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

// generateFileID 生成文件ID（公共函数）
func generateFileID() string {
	return uuid.New().String()
}

// 文件类型检测辅助函数
func detectFileTypeByMime(mimeType string) FileType {
	switch {
	case mimeType == "":
		return FileTypeOther
	case strings.HasPrefix(mimeType, "image/"):
		return FileTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return FileTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return FileTypeAudio
	case mimeType == "application/pdf" ||
		strings.HasPrefix(mimeType, "application/msword") ||
		strings.HasPrefix(mimeType, "application/vnd.ms-excel") ||
		strings.HasPrefix(mimeType, "application/vnd.ms-powerpoint") ||
		strings.HasPrefix(mimeType, "application/vnd.openxmlformats"):
		return FileTypeDocument
	case mimeType == "application/zip" ||
		mimeType == "application/x-rar-compressed" ||
		mimeType == "application/x-tar" ||
		mimeType == "application/gzip":
		return FileTypeArchive
	default:
		return FileTypeOther
	}
}

// 文件类型检测辅助函数（通过扩展名）
func detectFileTypeByExt(ext string) FileType {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	imageExts := []string{"jpg", "jpeg", "png", "gif", "bmp", "webp", "svg"}
	videoExts := []string{"mp4", "avi", "mov", "wmv", "flv", "mkv"}
	audioExts := []string{"mp3", "wav", "ogg", "flac", "aac"}
	documentExts := []string{"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt"}
	archiveExts := []string{"zip", "rar", "7z", "tar", "gz"}

	for _, e := range imageExts {
		if e == ext {
			return FileTypeImage
		}
	}

	for _, e := range videoExts {
		if e == ext {
			return FileTypeVideo
		}
	}

	for _, e := range audioExts {
		if e == ext {
			return FileTypeAudio
		}
	}

	for _, e := range documentExts {
		if e == ext {
			return FileTypeDocument
		}
	}

	for _, e := range archiveExts {
		if e == ext {
			return FileTypeArchive
		}
	}

	return FileTypeOther
}
