// file: pkg/upload/validator.go
package upload

import (
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// FileType 文件类型
type FileType string

const (
	FileTypeImage    FileType = "image"    // 图片
	FileTypeDocument FileType = "document" // 文档
	FileTypeVideo    FileType = "video"    // 视频
	FileTypeAudio    FileType = "audio"    // 音频
	FileTypeArchive  FileType = "archive"  // 压缩包
	FileTypeOther    FileType = "other"    // 其他
)

// ValidationConfig 验证配置
type ValidationConfig struct {
	AllowedTypes      []FileType `mapstructure:"allowed_types"`        // 允许的文件类型
	AllowedExtensions []string   `mapstructure:"allowed_extensions"`   // 允许的扩展名
	AllowedMimeTypes  []string   `mapstructure:"allowed_mime_types"`   // 允许的MIME类型
	MaxFileSize       int64      `mapstructure:"max_file_size"`        // 最大文件大小（字节）
	MinFileSize       int64      `mapstructure:"min_file_size"`        // 最小文件大小（字节）
	MaxFileNameLength int        `mapstructure:"max_file_name_length"` // 最大文件名长度
	ValidateVirus     bool       `mapstructure:"validate_virus"`       // 是否验证病毒
	ScanTimeout       int        `mapstructure:"scan_timeout"`         // 病毒扫描超时（秒）
}

// Validator 文件验证器
type Validator struct {
	config *ValidationConfig
}

// NewValidator 创建文件验证器
func NewValidator(config *ValidationConfig) *Validator {
	if config == nil {
		config = &ValidationConfig{
			MaxFileSize:       10 * 1024 * 1024, // 10MB
			MaxFileNameLength: 255,
		}
	}

	return &Validator{
		config: config,
	}
}

// Validate 验证文件
func (v *Validator) Validate(header *multipart.FileHeader) error {
	if header == nil {
		return errors.New(errors.CodeValidationFailed, "File header is nil")
	}

	// 验证文件大小
	if err := v.validateSize(header.Size); err != nil {
		return err
	}

	// 验证文件名
	if err := v.validateName(header.Filename); err != nil {
		return err
	}

	// 验证文件扩展名
	if err := v.validateExtension(header.Filename); err != nil {
		return err
	}

	// 验证MIME类型
	if err := v.validateMimeType(header); err != nil {
		return err
	}

	// 验证文件类型
	if err := v.validateFileType(header.Filename, header.Header.Get("Content-Type")); err != nil {
		return err
	}

	return nil
}

// validateSize 验证文件大小
func (v *Validator) validateSize(size int64) error {
	if size <= 0 {
		return errors.New(errors.CodeValidationFailed, "File size must be greater than 0")
	}

	if v.config.MaxFileSize > 0 && size > v.config.MaxFileSize {
		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("File size exceeds maximum allowed size: %d bytes", v.config.MaxFileSize))
	}

	if v.config.MinFileSize > 0 && size < v.config.MinFileSize {
		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("File size is less than minimum required size: %d bytes", v.config.MinFileSize))
	}

	return nil
}

// validateName 验证文件名
func (v *Validator) validateName(filename string) error {
	if filename == "" {
		return errors.New(errors.CodeValidationFailed, "Filename is empty")
	}

	// 检查文件名长度
	if v.config.MaxFileNameLength > 0 && len(filename) > v.config.MaxFileNameLength {
		return errors.New(errors.CodeValidationFailed,
			fmt.Sprintf("Filename exceeds maximum length: %d characters", v.config.MaxFileNameLength))
	}

	// 检查危险字符
	dangerousChars := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerousChars {
		if strings.Contains(filename, char) {
			return errors.New(errors.CodeValidationFailed,
				fmt.Sprintf("Filename contains dangerous character: %s", char))
		}
	}

	// 检查空字节注入
	if strings.Contains(filename, "\x00") {
		return errors.New(errors.CodeValidationFailed, "Filename contains null byte")
	}

	return nil
}

// validateExtension 验证文件扩展名
func (v *Validator) validateExtension(filename string) error {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

	// 如果没有配置允许的扩展名，允许所有扩展名
	if len(v.config.AllowedExtensions) == 0 {
		return nil
	}

	// 检查扩展名是否在允许列表中
	for _, allowedExt := range v.config.AllowedExtensions {
		if strings.ToLower(allowedExt) == ext {
			return nil
		}
	}

	return errors.New(errors.CodeValidationFailed,
		fmt.Sprintf("File extension not allowed: %s", ext))
}

// validateMimeType 验证MIME类型
func (v *Validator) validateMimeType(header *multipart.FileHeader) error {
	mimeType := header.Header.Get("Content-Type")

	// 如果没有配置允许的MIME类型，允许所有MIME类型
	if len(v.config.AllowedMimeTypes) == 0 {
		return nil
	}

	// 检查MIME类型是否在允许列表中
	for _, allowedMimeType := range v.config.AllowedMimeTypes {
		if mimeType == allowedMimeType {
			return nil
		}
	}

	return errors.New(errors.CodeValidationFailed,
		fmt.Sprintf("MIME type not allowed: %s", mimeType))
}

// validateFileType 验证文件类型
func (v *Validator) validateFileType(filename, mimeType string) error {
	// 如果没有配置允许的文件类型，允许所有文件类型
	if len(v.config.AllowedTypes) == 0 {
		return nil
	}

	fileType := v.detectFileType(filename, mimeType)

	// 检查文件类型是否在允许列表中
	for _, allowedType := range v.config.AllowedTypes {
		if fileType == allowedType {
			return nil
		}
	}

	return errors.New(errors.CodeValidationFailed,
		fmt.Sprintf("File type not allowed: %s", fileType))
}

// detectFileType 检测文件类型
func (v *Validator) detectFileType(filename, mimeType string) FileType {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

	// 图片类型
	imageExts := []string{"jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "ico", "tiff"}
	imageMimes := []string{"image/jpeg", "image/png", "image/gif", "image/bmp", "image/webp", "image/svg+xml", "image/x-icon", "image/tiff"}

	// 文档类型
	docExts := []string{"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "csv", "rtf", "odt", "ods", "odp"}
	docMimes := []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"text/plain", "text/csv", "application/rtf", "application/vnd.oasis.opendocument.text",
		"application/vnd.oasis.opendocument.spreadsheet", "application/vnd.oasis.opendocument.presentation"}

	// 视频类型
	videoExts := []string{"mp4", "avi", "mov", "wmv", "flv", "mkv", "webm", "m4v", "mpg", "mpeg"}
	videoMimes := []string{"video/mp4", "video/x-msvideo", "video/quicktime", "video/x-ms-wmv",
		"video/x-flv", "video/x-matroska", "video/webm", "video/x-m4v", "video/mpeg"}

	// 音频类型
	audioExts := []string{"mp3", "wav", "ogg", "flac", "aac", "wma", "m4a"}
	audioMimes := []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/flac", "audio/aac", "audio/x-ms-wma", "audio/mp4"}

	// 压缩包类型
	archiveExts := []string{"zip", "rar", "7z", "tar", "gz", "bz2", "xz"}
	archiveMimes := []string{"application/zip", "application/x-rar-compressed", "application/x-7z-compressed",
		"application/x-tar", "application/gzip", "application/x-bzip2", "application/x-xz"}

	// 根据扩展名判断
	if contains(imageExts, ext) || contains(imageMimes, mimeType) {
		return FileTypeImage
	}

	if contains(docExts, ext) || contains(docMimes, mimeType) {
		return FileTypeDocument
	}

	if contains(videoExts, ext) || contains(videoMimes, mimeType) {
		return FileTypeVideo
	}

	if contains(audioExts, ext) || contains(audioMimes, mimeType) {
		return FileTypeAudio
	}

	if contains(archiveExts, ext) || contains(archiveMimes, mimeType) {
		return FileTypeArchive
	}

	return FileTypeOther
}

// contains 检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ScanVirus 扫描病毒
func (v *Validator) ScanVirus(filePath string) (bool, string, error) {
	if !v.config.ValidateVirus {
		return true, "Virus scan disabled", nil
	}

	// 这里可以集成第三方病毒扫描服务
	// 例如：ClamAV, VirusTotal API, 阿里云安全服务等

	// 由于病毒扫描通常需要调用外部服务，这里提供框架接口
	// 实际使用时需要实现具体的扫描逻辑

	logger.Debug("Virus scan requested",
		logger.String("file_path", filePath),
	)

	// 模拟扫描结果
	// 实际项目中应该调用真实的病毒扫描服务
	return true, "Clean", nil
}

// GetFileType 获取文件类型
func (v *Validator) GetFileType(filename, mimeType string) FileType {
	return v.detectFileType(filename, mimeType)
}
