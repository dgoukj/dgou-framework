package upload

import (
	"context"
	"github.com/dgoukj/dgou-framework/pkg/logger"
	pkgErrors "github.com/pkg/errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Config struct {
	MaxFileSize  int64
	AllowedExts  []string
	AllowedMimes []string
}

type Manager struct {
	storage   Storage
	validator *Validator
	config    Config
	logger    *logger.Logger
}

func NewManager(storage Storage, cfg Config, log *logger.Logger) *Manager {
	valCfg := &ValidationConfig{
		MaxFileSize:       cfg.MaxFileSize,
		AllowedExtensions: cfg.AllowedExts,
		AllowedMimeTypes:  cfg.AllowedMimes,
		MaxFileNameLength: 255,
	}
	return &Manager{
		storage:   storage,
		validator: NewValidator(valCfg, log),
		config:    cfg,
		logger:    log,
	}
}

func (m *Manager) Upload(ctx context.Context, file *multipart.FileHeader, opts ...UploadOption) (*FileInfo, error) {
	if err := m.validator.Validate(file); err != nil {
		return nil, err
	}
	opt := &UploadOptions{IsPublic: true}
	for _, o := range opts {
		o(opt)
	}
	info := &FileInfo{
		ID:          uuid.New().String(),
		Name:        file.Filename,
		Path:        m.generatePath(file.Filename, opt),
		Size:        file.Size,
		MimeType:    file.Header.Get("Content-Type"),
		Extension:   strings.TrimPrefix(filepath.Ext(file.Filename), "."),
		StorageType: m.storage.Type(),
		IsPublic:    opt.IsPublic,
		CreatedAt:   time.Now(),
		UploaderID:  opt.UploaderID,
		Metadata:    opt.Metadata,
	}
	if err := m.storage.PutMultipart(ctx, info, file); err != nil {
		return nil, err
	}
	return info, nil
}

func (m *Manager) Delete(ctx context.Context, path string) error {
	return m.storage.Delete(ctx, path)
}

func (m *Manager) GetURL(ctx context.Context, path string, public bool) (string, error) {
	return m.storage.GetURL(ctx, path, public)
}

func (m *Manager) generatePath(filename string, opt *UploadOptions) string {
	dir := time.Now().Format("2006/01/02")
	if opt.Category != "" {
		dir = opt.Category + "/" + dir
	}
	return dir + "/" + uuid.New().String() + "_" + filename
}

type UploadOptions struct {
	UploaderID string
	IsPublic   bool
	Category   string
	Metadata   map[string]interface{}
	MimeType   string
}

type UploadOption func(*UploadOptions)

func WithUploaderID(id string) UploadOption {
	return func(o *UploadOptions) { o.UploaderID = id }
}
func WithPublic(public bool) UploadOption {
	return func(o *UploadOptions) { o.IsPublic = public }
}
func WithCategory(category string) UploadOption {
	return func(o *UploadOptions) { o.Category = category }
}
func WithMetadata(md map[string]interface{}) UploadOption {
	return func(o *UploadOptions) { o.Metadata = md }
}

// WithMimeType 设置 MIME 类型
func WithMimeType(mime string) UploadOption {
	return func(o *UploadOptions) { o.MimeType = mime }
}

// UploadReader 直接上传 io.Reader 流
func (m *Manager) UploadReader(ctx context.Context, filename string, reader io.Reader, size int64, opts ...UploadOption) (*FileInfo, error) {
	// 验证大小
	if m.config.MaxFileSize > 0 && size > m.config.MaxFileSize {
		return nil, pkgErrors.Errorf("file size exceeds maximum allowed size: %d bytes", m.config.MaxFileSize)
	}

	opt := &UploadOptions{IsPublic: true}
	for _, o := range opts {
		o(opt)
	}

	// 生成路径
	path := m.generatePath(filename, opt)

	info := &FileInfo{
		ID:          uuid.New().String(),
		Name:        filename,
		Path:        path,
		Size:        size,
		MimeType:    opt.MimeType,
		Extension:   strings.TrimPrefix(filepath.Ext(filename), "."),
		StorageType: m.storage.Type(),
		IsPublic:    opt.IsPublic,
		CreatedAt:   time.Now(),
		UploaderID:  opt.UploaderID,
		Metadata:    opt.Metadata,
	}
	if info.MimeType == "" {
		info.MimeType = "application/octet-stream"
	}

	if err := m.storage.Put(ctx, info, reader); err != nil {
		return nil, err
	}
	return info, nil
}
