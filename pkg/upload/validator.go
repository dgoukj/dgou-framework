package upload

import (
	"dgou/pkg/logger"
	pkgErrors "github.com/pkg/errors"
	"go.uber.org/zap"
	"mime/multipart"
	"path/filepath"
	"strings"
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

// ValidationConfig 验证配置
type ValidationConfig struct {
	AllowedTypes      []FileType
	AllowedExtensions []string
	AllowedMimeTypes  []string
	MaxFileSize       int64
	MinFileSize       int64
	MaxFileNameLength int
	ValidateVirus     bool
	ScanTimeout       int
}

// Validator 文件验证器
type Validator struct {
	config *ValidationConfig
	logger *logger.Logger
}

func NewValidator(cfg *ValidationConfig, log *logger.Logger) *Validator {
	if cfg == nil {
		cfg = &ValidationConfig{
			MaxFileSize:       10 * 1024 * 1024,
			MaxFileNameLength: 255,
		}
	}
	return &Validator{
		config: cfg,
		logger: log,
	}
}

// Validate 验证文件
func (v *Validator) Validate(header *multipart.FileHeader) error {
	if header == nil {
		return pkgErrors.New("file header is nil")
	}
	if err := v.validateSize(header.Size); err != nil {
		return err
	}
	if err := v.validateName(header.Filename); err != nil {
		return err
	}
	if err := v.validateExtension(header.Filename); err != nil {
		return err
	}
	if err := v.validateMimeType(header); err != nil {
		return err
	}
	if err := v.validateFileType(header.Filename, header.Header.Get("Content-Type")); err != nil {
		return err
	}
	return nil
}

func (v *Validator) validateSize(size int64) error {
	if size <= 0 {
		return pkgErrors.New("file size must be greater than 0")
	}
	if v.config.MaxFileSize > 0 && size > v.config.MaxFileSize {
		return pkgErrors.Errorf("file size exceeds maximum allowed size: %d bytes", v.config.MaxFileSize)
	}
	if v.config.MinFileSize > 0 && size < v.config.MinFileSize {
		return pkgErrors.Errorf("file size is less than minimum required size: %d bytes", v.config.MinFileSize)
	}
	return nil
}

func (v *Validator) validateName(filename string) error {
	if filename == "" {
		return pkgErrors.New("filename is empty")
	}
	if v.config.MaxFileNameLength > 0 && len(filename) > v.config.MaxFileNameLength {
		return pkgErrors.Errorf("filename exceeds maximum length: %d characters", v.config.MaxFileNameLength)
	}
	dangerousChars := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerousChars {
		if strings.Contains(filename, char) {
			return pkgErrors.Errorf("filename contains dangerous character: %s", char)
		}
	}
	if strings.Contains(filename, "\x00") {
		return pkgErrors.New("filename contains null byte")
	}
	return nil
}

func (v *Validator) validateExtension(filename string) error {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if len(v.config.AllowedExtensions) == 0 {
		return nil
	}
	for _, allowed := range v.config.AllowedExtensions {
		if strings.ToLower(allowed) == ext {
			return nil
		}
	}
	return pkgErrors.Errorf("file extension not allowed: %s", ext)
}

func (v *Validator) validateMimeType(header *multipart.FileHeader) error {
	mime := header.Header.Get("Content-Type")
	if len(v.config.AllowedMimeTypes) == 0 {
		return nil
	}
	for _, allowed := range v.config.AllowedMimeTypes {
		if mime == allowed {
			return nil
		}
	}
	return pkgErrors.Errorf("MIME type not allowed: %s", mime)
}

func (v *Validator) validateFileType(filename, mimeType string) error {
	if len(v.config.AllowedTypes) == 0 {
		return nil
	}
	ft := detectFileType(filename, mimeType)
	for _, allowed := range v.config.AllowedTypes {
		if ft == allowed {
			return nil
		}
	}
	return pkgErrors.Errorf("file type not allowed: %s", ft)
}

// detectFileType 检测文件类型
func detectFileType(filename, mimeType string) FileType {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if t := detectByExt(ext); t != FileTypeOther {
		return t
	}
	return detectByMime(mimeType)
}

func detectByExt(ext string) FileType {
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

func detectByMime(mime string) FileType {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return FileTypeImage
	case strings.HasPrefix(mime, "video/"):
		return FileTypeVideo
	case strings.HasPrefix(mime, "audio/"):
		return FileTypeAudio
	case mime == "application/pdf" ||
		strings.HasPrefix(mime, "application/msword") ||
		strings.HasPrefix(mime, "application/vnd.ms-excel") ||
		strings.HasPrefix(mime, "application/vnd.ms-powerpoint") ||
		strings.HasPrefix(mime, "application/vnd.openxmlformats"):
		return FileTypeDocument
	case mime == "application/zip" ||
		mime == "application/x-rar-compressed" ||
		mime == "application/x-tar" ||
		mime == "application/gzip":
		return FileTypeArchive
	default:
		return FileTypeOther
	}
}

// ScanVirus 病毒扫描（仅占位，需集成真实服务）
func (v *Validator) ScanVirus(filePath string) (bool, string, error) {
	if !v.config.ValidateVirus {
		return true, "Virus scan disabled", nil
	}
	// 调用真实扫描服务，此处模拟
	v.logger.Debug("Virus scan requested", zap.String("file_path", filePath))
	return true, "Clean", nil
}
