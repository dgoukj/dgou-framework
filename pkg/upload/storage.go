package upload

import (
	"context"
	"io"
	"mime/multipart"
	"time"
)

type StorageType string

const (
	Local StorageType = "local"
	OSS   StorageType = "oss"
	S3    StorageType = "s3"
)

type FileInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Path        string                 `json:"path"`
	URL         string                 `json:"url"`
	Size        int64                  `json:"size"`
	MimeType    string                 `json:"mime_type"`
	Extension   string                 `json:"extension"`
	MD5         string                 `json:"md5,omitempty"`
	StorageType StorageType            `json:"storage_type"`
	UploaderID  string                 `json:"uploader_id"`
	CreatedAt   time.Time              `json:"created_at"`
	IsPublic    bool                   `json:"is_public"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type Storage interface {
	Type() StorageType
	Put(ctx context.Context, file *FileInfo, reader io.Reader) error
	PutMultipart(ctx context.Context, file *FileInfo, header *multipart.FileHeader) error
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	GetURL(ctx context.Context, path string, public bool) (string, error)
	Delete(ctx context.Context, path string) error
	Stat(ctx context.Context, path string) (*FileInfo, error)
	Exists(ctx context.Context, path string) (bool, error)
	Copy(ctx context.Context, srcPath, dstPath string) error
	Move(ctx context.Context, srcPath, dstPath string) error
}
