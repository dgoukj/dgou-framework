## 文件上传组件 (pkg/upload)

### 特性
- ✅ **多存储后端**：支持本地存储和阿里云OSS，易于扩展其他存储
- ✅ **文件类型验证**：基于扩展名、MIME类型和文件类型的完整验证
- ✅ **病毒扫描集成**：支持病毒扫描接口，可集成ClamAV等扫描引擎
- ✅ **分片上传支持**：大文件分片上传，支持断点续传和进度查询
- ✅ **CDN集成**：支持CDN加速，自动生成CDN URL
- ✅ **安全性**：文件名安全检查，防止目录遍历攻击
- ✅ **高性能**：流式处理，内存高效，支持并发上传
- ✅ **可配置性**：灵活的配置选项，支持多种上传场景

### 快速开始

#### 1. 基本配置

```yaml
# config/config.yaml
upload:
  # 存储配置
  storage_type: "local"  # local 或 oss
  base_path: "./uploads" # 本地存储基础路径
  base_url: "http://localhost:8080/uploads" # 基础访问URL
  cdn_url: "" # CDN URL（可选）

  # OSS配置（如果使用OSS）
  access_key_id: ""
  access_key_secret: ""
  endpoint: "oss-cn-hangzhou.aliyuncs.com"
  bucket: "my-bucket"
  region: "cn-hangzhou"

  # 验证配置
  allowed_types: ["image", "document"] # 允许的文件类型
  allowed_extensions: ["jpg", "jpeg", "png", "pdf", "doc", "docx"]
  allowed_mime_types: ["image/jpeg", "image/png", "application/pdf"]
  max_file_size: 10485760 # 10MB
  min_file_size: 1024     # 1KB
  max_file_name_length: 255
  validate_virus: false   # 是否启用病毒扫描
  scan_timeout: 30        # 病毒扫描超时（秒）

  # 分片上传配置
  chunk_enabled: true     # 是否启用分片上传
  chunk_size: 5242880    # 分片大小（5MB）
  max_chunks: 1000       # 最大分片数
  temp_dir: "/tmp/uploads" # 临时目录
  cleanup_interval: 1800 # 清理间隔（秒）
  max_temp_file_age: 86400 # 临时文件最大年龄（秒）
  enable_resumable: true # 是否启用断点续传

  # 其他配置
  enable_virus_scan: false # 是否启用病毒扫描
  use_https: true        # 是否使用HTTPS
  enable_cdn: false      # 是否启用CDN
```

#### 2. 初始化上传管理器

```go
import (
    "dgou/pkg/upload"
    "dgou/pkg/config"
)

func main() {
    // 加载配置
    cfg := config.LoadConfig()

    // 初始化上传管理器
    manager, err := upload.InitUploadManager(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Stop()

    // 创建上传处理器
    handler := upload.NewUploadHandler(manager)

    // 在Gin中注册路由
    router := gin.Default()
    handler.RegisterRoutes(router.Group("/api"))
}
```

#### 3. 上传单个文件

```go
// 使用Gin处理文件上传
func uploadHandler(c *gin.Context) {
    // 获取文件
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
        return
    }

    // 获取上传管理器
    manager := upload.GetUploadManager()

    // 配置上传选项
    options := &upload.UploadOptions{
        UploaderID:   c.GetString("user_id"),
        UploaderIP:   c.ClientIP(),
        IsPublic:     true,
        Category:     "avatar",
        SubDirectory: "users",
    }

    // 上传文件
    fileInfo, err := manager.Upload(c.Request.Context(), file, options)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "File uploaded successfully",
        "file":    fileInfo,
        "url":     fileInfo.URL,
    })
}
```

#### 4. 上传多个文件

```go
func uploadMultipleHandler(c *gin.Context) {
    form, err := c.MultipartForm()
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form"})
        return
    }

    files := form.File["files"]
    manager := upload.GetUploadManager()
    options := &upload.UploadOptions{
        UploaderID: c.GetString("user_id"),
        UploaderIP: c.ClientIP(),
        Category:   "documents",
    }

    results := make([]gin.H, 0)
    errors := make([]string, 0)

    for _, file := range files {
        fileInfo, err := manager.Upload(c.Request.Context(), file, options)
        if err != nil {
            errors = append(errors, fmt.Sprintf("%s: %v", file.Filename, err))
            continue
        }

        results = append(results, gin.H{
            "filename": file.Filename,
            "url":      fileInfo.URL,
            "size":     fileInfo.Size,
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "success": results,
        "errors":  errors,
        "count":   len(results),
    })
}
```

### 高级用法

#### 1. 分片上传（大文件）

```javascript
// 前端JavaScript示例
const uploadLargeFile = async (file) => {
    const chunkSize = 5 * 1024 * 1024; // 5MB
    const totalChunks = Math.ceil(file.size / chunkSize);

    // 1. 开始上传会话
    const startResponse = await fetch('/api/upload/chunk/start', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            file_name: file.name,
            file_size: file.size,
            category: 'videos'
        })
    });

    const { upload_id, total_chunks, chunk_size } = await startResponse.json();

    // 2. 上传所有分片
    for (let chunkNumber = 1; chunkNumber <= totalChunks; chunkNumber++) {
        const start = (chunkNumber - 1) * chunkSize;
        const end = Math.min(start + chunkSize, file.size);
        const chunk = file.slice(start, end);

        const formData = new FormData();
        formData.append('chunk', chunk);

        await fetch(`/api/upload/chunk/${upload_id}/chunk/${chunkNumber}`, {
            method: 'POST',
            body: formData
        });

        // 更新进度
        const progress = (chunkNumber / totalChunks) * 100;
        console.log(`Upload progress: ${progress.toFixed(2)}%`);
    }

    // 3. 完成上传
    const completeResponse = await fetch(`/api/upload/chunk/${upload_id}/complete`, {
        method: 'POST'
    });

    const result = await completeResponse.json();
    console.log('Upload completed:', result);
};
```

```go
// Go后端处理
func handleChunkUpload(c *gin.Context) {
    // 处理器已内置，直接使用即可
}
```

#### 2. 文件验证和病毒扫描

```go
// 自定义验证器
func createCustomValidator() *upload.Validator {
    config := &upload.ValidationConfig{
        AllowedTypes: []upload.FileType{
            upload.FileTypeImage,
            upload.FileTypeDocument,
            upload.FileTypeVideo,
        },
        AllowedExtensions: []string{
            "jpg", "jpeg", "png", "gif", "pdf", "doc", "docx", "mp4", "avi",
        },
        MaxFileSize: 100 * 1024 * 1024, // 100MB
        ValidateVirus: true,
        ScanTimeout: 60,
    }

    return upload.NewValidator(config)
}

// 集成ClamAV病毒扫描
func integrateClamAV() {
    validator := createCustomValidator()

    // 重写ScanVirus方法
    type CustomValidator struct {
        *upload.Validator
    }

    customValidator := &CustomValidator{Validator: validator}

    // 实现ClamAV扫描
    func (v *CustomValidator) ScanVirus(filePath string) (bool, string, error) {
        // 调用ClamAV进行扫描
        // 这里需要安装并运行clamd服务
        cmd := exec.Command("clamdscan", "--no-summary", filePath)
        output, err := cmd.CombinedOutput()

        if err != nil {
            // 检查是否为病毒检测错误
            if strings.Contains(string(output), "FOUND") {
                return false, "Virus detected", nil
            }
            return false, "Scan failed", err
        }

        return true, "Clean", nil
    }
}
```

#### 3. CDN集成

```go
// 配置CDN
func configureCDN() {
    config := &upload.StorageConfig{
        Type:      upload.StorageTypeOSS,
        Endpoint:  "oss-cn-hangzhou.aliyuncs.com",
        Bucket:    "my-bucket",
        CDNURL:    "cdn.example.com", // CDN域名
        EnableCDN: true,
        UseHTTPS:  true,
    }

    // 使用CDN后，文件URL会自动使用CDN域名
    // 例如：https://cdn.example.com/images/avatar.jpg
}

// 动态切换CDN
func getFileURLWithCDN(manager *upload.UploadManager, path string, useCDN bool) (string, error) {
    // 临时启用/禁用CDN
    originalCDN := manager.config.StorageConfig.EnableCDN
    manager.config.StorageConfig.EnableCDN = useCDN

    url, err := manager.GetFileURL(context.Background(), path, true)

    // 恢复原设置
    manager.config.StorageConfig.EnableCDN = originalCDN

    return url, err
}
```

#### 4. 文件处理（缩略图、水印等）

```go
// 图片处理示例
func processImage(fileInfo *upload.FileInfo) error {
    // 读取图片
    reader, err := uploadManager.GetFile(context.Background(), fileInfo.Path)
    if err != nil {
        return err
    }
    defer reader.Close()

    // 解码图片
    img, _, err := image.Decode(reader)
    if err != nil {
        return err
    }

    // 生成缩略图
    thumbnail := resize.Thumbnail(200, 200, img, resize.Lanczos3)

    // 保存缩略图
    thumbnailPath := fmt.Sprintf("%s_thumb.jpg", strings.TrimSuffix(fileInfo.Path, filepath.Ext(fileInfo.Path)))

    thumbnailFile, err := os.Create(thumbnailPath)
    if err != nil {
        return err
    }
    defer thumbnailFile.Close()

    if err := jpeg.Encode(thumbnailFile, thumbnail, &jpeg.Options{Quality: 85}); err != nil {
        return err
    }

    // 上传缩略图到存储
    thumbnailReader, _ := os.Open(thumbnailPath)
    defer thumbnailReader.Close()
    defer os.Remove(thumbnailPath)

    thumbnailInfo := &upload.FileInfo{
        ID:          uuid.New().String(),
        Name:        fmt.Sprintf("%s_thumb%s", strings.TrimSuffix(fileInfo.Name, filepath.Ext(fileInfo.Name)), filepath.Ext(fileInfo.Name)),
        StorageName: fmt.Sprintf("%s_thumb%s", strings.TrimSuffix(fileInfo.StorageName, filepath.Ext(fileInfo.StorageName)), filepath.Ext(fileInfo.StorageName)),
        Path:        thumbnailPath,
        MimeType:    "image/jpeg",
        Size:        0, // 实际大小需要计算
    }

    return uploadManager.storage.Put(context.Background(), thumbnailInfo, thumbnailReader)
}
```

#### 5. 权限控制

```go
// 基于角色的文件访问控制
func checkFilePermission(c *gin.Context, filePath string) bool {
    userRole := c.GetString("user_role")

    // 解析文件路径获取分类
    parts := strings.Split(filePath, "/")
    if len(parts) == 0 {
        return false
    }

    category := parts[0]

    // 定义权限规则
    permissionRules := map[string][]string{
        "admin":   {"*"},                    // 管理员可以访问所有
        "user":    {"avatar", "documents"},  // 普通用户可以访问头像和文档
        "guest":   {"public"},               // 访客只能访问公开文件
    }

    // 检查权限
    allowedCategories, exists := permissionRules[userRole]
    if !exists {
        return false
    }

    // 检查通配符
    for _, allowed := range allowedCategories {
        if allowed == "*" || allowed == category {
            return true
        }
    }

    return false
}

// 私有文件访问
func servePrivateFile(c *gin.Context) {
    filePath := c.Param("path")

    // 检查权限
    if !checkFilePermission(c, filePath) {
        c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
        return
    }

    // 生成临时访问URL（带签名）
    manager := upload.GetUploadManager()
    url, err := manager.GetFileURL(c.Request.Context(), filePath, false)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 重定向到签名URL
    c.Redirect(http.StatusFound, url)
}
```

#### 6. 文件元数据管理

```go
// 添加上下文元数据
func uploadWithMetadata(c *gin.Context) {
    file, _ := c.FormFile("file")

    options := &upload.UploadOptions{
        UploaderID: c.GetString("user_id"),
        Metadata: map[string]interface{}{
            "upload_context": c.Query("context"),
            "tags":           strings.Split(c.Query("tags"), ","),
            "description":    c.Query("description"),
            "custom_field":   c.Query("custom_field"),
            "timestamp":      time.Now().Unix(),
        },
    }

    manager := upload.GetUploadManager()
    fileInfo, _ := manager.Upload(c.Request.Context(), file, options)

    // 保存元数据到数据库
    saveFileMetadataToDB(fileInfo)
}

// 文件搜索和过滤
func searchFiles(c *gin.Context) {
    query := c.Query("q")
    category := c.Query("category")
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")

    // 从数据库查询文件元数据
    files := queryFileMetadataFromDB(query, category, startDate, endDate)

    c.JSON(http.StatusOK, gin.H{
        "files": files,
        "count": len(files),
    })
}
```

### 配置说明

#### 存储配置

```yaml
upload:
  # 本地存储配置
  storage_type: "local"
  base_path: "./uploads"           # 存储根目录
  base_url: "http://localhost:8080/uploads"

  # OSS存储配置
  storage_type: "oss"
  access_key_id: "your-access-key-id"
  access_key_secret: "your-access-key-secret"
  endpoint: "oss-cn-hangzhou.aliyuncs.com"
  bucket: "your-bucket-name"
  region: "cn-hangzhou"
  use_https: true

  # CDN配置
  cdn_url: "https://cdn.example.com"
  enable_cdn: true
```

#### 验证配置

```yaml
upload:
  validation:
    # 文件类型限制
    allowed_types: ["image", "document", "video"]

    # 扩展名限制
    allowed_extensions: ["jpg", "png", "pdf", "mp4"]

    # MIME类型限制
    allowed_mime_types: ["image/jpeg", "image/png", "application/pdf", "video/mp4"]

    # 大小限制
    max_file_size: 104857600  # 100MB
    min_file_size: 1024       # 1KB

    # 文件名限制
    max_file_name_length: 255

    # 病毒扫描
    validate_virus: true
    scan_timeout: 30
```

#### 分片上传配置

```yaml
upload:
  chunk:
    enabled: true
    chunk_size: 5242880      # 5MB
    max_chunks: 1000
    temp_dir: "/tmp/uploads"
    cleanup_interval: 1800   # 30分钟
    max_temp_file_age: 86400 # 24小时
    enable_resumable: true   # 断点续传
```

### 最佳实践

#### 1. 安全性建议

```go
// 1. 文件名安全处理
func sanitizeFilename(filename string) string {
    // 移除路径分隔符
    filename = filepath.Base(filename)

    // 移除危险字符
    dangerousChars := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\x00"}
    for _, char := range dangerousChars {
        filename = strings.ReplaceAll(filename, char, "")
    }

    // 限制长度
    if len(filename) > 255 {
        ext := filepath.Ext(filename)
        name := filename[:255-len(ext)]
        filename = name + ext
    }

    return filename
}

// 2. 文件类型白名单
func validateFileTypeByContent(fileHeader *multipart.FileHeader) error {
    // 读取文件前512字节检测真实类型
    file, err := fileHeader.Open()
    if err != nil {
        return err
    }
    defer file.Close()

    buffer := make([]byte, 512)
    n, err := file.Read(buffer)
    if err != nil && err != io.EOF {
        return err
    }

    // 检测MIME类型
    mimeType := http.DetectContentType(buffer[:n])

    // 验证允许的MIME类型
    allowedMimes := []string{"image/jpeg", "image/png", "application/pdf"}
    for _, allowed := range allowedMimes {
        if mimeType == allowed {
            return nil
        }
    }

    return errors.New("File type not allowed")
}

// 3. 文件大小限制
func limitFileSize(c *gin.Context) {
    // 设置请求体大小限制
    c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100<<20) // 100MB

    // 在Gin中间件中限制
    router.MaxMultipartMemory = 8 << 20 // 8MB
}
```

#### 2. 性能优化

```go
// 1. 使用缓冲区池
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 32*1024) // 32KB缓冲区
    },
}

func uploadWithBufferPool(file io.Reader) error {
    buffer := bufferPool.Get().([]byte)
    defer bufferPool.Put(buffer)

    for {
        n, err := file.Read(buffer)
        if err != nil && err != io.EOF {
            return err
        }
        if n == 0 {
            break
        }
        // 处理数据...
    }
    return nil
}

// 2. 并发上传控制
type UploadLimiter struct {
    semaphore chan struct{}
}

func NewUploadLimiter(maxConcurrent int) *UploadLimiter {
    return &UploadLimiter{
        semaphore: make(chan struct{}, maxConcurrent),
    }
}

func (ul *UploadLimiter) Upload(file *multipart.FileHeader) error {
    ul.semaphore <- struct{}{}
    defer func() { <-ul.semaphore }()

    // 执行上传
    return doUpload(file)
}

// 3. 内存优化
func streamUpload(c *gin.Context) {
    // 使用流式处理，避免内存中保存整个文件
    file, _ := c.FormFile("file")
    src, _ := file.Open()
    defer src.Close()

    // 直接流式传输到存储
    dst, _ := os.CreateTemp("", "upload-*")
    defer dst.Close()
    defer os.Remove(dst.Name())

    if _, err := io.Copy(dst, src); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
}
```

#### 3. 错误处理和重试

```go
// 1. 上传重试机制
func uploadWithRetry(manager *upload.UploadManager, file *multipart.FileHeader, options *upload.UploadOptions, maxRetries int) (*upload.FileInfo, error) {
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        fileInfo, err := manager.Upload(context.Background(), file, options)
        if err == nil {
            return fileInfo, nil
        }

        lastErr = err

        // 指数退避
        delay := time.Duration(math.Pow(2, float64(i))) * time.Second
        time.Sleep(delay)

        // 重新打开文件（如果需要）
        if i < maxRetries-1 {
            // 重新获取文件句柄
        }
    }

    return nil, fmt.Errorf("upload failed after %d retries: %v", maxRetries, lastErr)
}

// 2. 监控和日志
func uploadWithMonitoring(c *gin.Context) {
    startTime := time.Now()

    file, _ := c.FormFile("file")

    // 记录上传开始
    logger.Info("Upload started",
        logger.String("filename", file.Filename),
        logger.Int64("filesize", file.Size),
        logger.String("client_ip", c.ClientIP()),
    )

    // 执行上传
    fileInfo, err := uploadManager.Upload(c.Request.Context(), file, &upload.UploadOptions{})

    duration := time.Since(startTime)

    if err != nil {
        // 记录失败
        logger.Error("Upload failed",
            logger.String("filename", file.Filename),
            logger.Duration("duration", duration),
            logger.ErrorField(err),
        )
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 记录成功
    logger.Info("Upload completed",
        logger.String("filename", file.Filename),
        logger.String("file_id", fileInfo.ID),
        logger.Int64("filesize", fileInfo.Size),
        logger.Duration("duration", duration),
        logger.Float64("speed_mbps", float64(fileInfo.Size)/duration.Seconds()/1024/1024),
    )

    c.JSON(http.StatusOK, gin.H{"file": fileInfo})
}
```

#### 4. 存储策略

```go
// 1. 多存储后端切换
type MultiStorageManager struct {
    primary     upload.Storage
    secondary   upload.Storage
    usePrimary  bool
    mu          sync.RWMutex
}

func (m *MultiStorageManager) Put(ctx context.Context, file *upload.FileInfo, src io.Reader) error {
    m.mu.RLock()
    storage := m.primary
    if !m.usePrimary {
        storage = m.secondary
    }
    m.mu.RUnlock()

    err := storage.Put(ctx, file, src)
    if err != nil && m.usePrimary {
        // 主存储失败，切换到备用存储
        m.mu.Lock()
        m.usePrimary = false
        m.mu.Unlock()

        logger.Warn("Primary storage failed, switching to secondary",
            logger.String("file_id", file.ID),
            logger.ErrorField(err),
        )

        return m.secondary.Put(ctx, file, src)
    }

    return err
}

// 2. 存储分层策略
type TieredStorage struct {
    hotStorage   upload.Storage  // 热存储（SSD/内存）
    warmStorage  upload.Storage  // 温存储（普通磁盘）
    coldStorage  upload.Storage  // 冷存储（对象存储/磁带）
}

func (t *TieredStorage) Put(ctx context.Context, file *upload.FileInfo, src io.Reader) error {
    // 根据文件类型和大小选择存储层
    if file.Size < 10*1024*1024 { // 小于10MB
        return t.hotStorage.Put(ctx, file, src)
    } else if file.Size < 100*1024*1024 { // 小于100MB
        return t.warmStorage.Put(ctx, file, src)
    } else {
        return t.coldStorage.Put(ctx, file, src)
    }
}

// 3. 自动清理旧文件
func autoCleanup(manager *upload.UploadManager) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        // 清理30天前的临时文件
        cleanupTempFiles(30 * 24 * time.Hour)

        // 清理过期的用户上传文件
        cleanupExpiredFiles()

        // 清理未完成的断点上传
        cleanupAbandonedUploads(7 * 24 * time.Hour)
    }
}
```

### 故障排除

#### 1. 上传失败处理

```go
// 常见错误处理
func handleUploadError(err error) string {
    if err == nil {
        return "Success"
    }

    // 检查错误类型
    if strings.Contains(err.Error(), "file size exceeds") {
        return "File too large. Please reduce file size."
    }

    if strings.Contains(err.Error(), "invalid file type") {
        return "File type not allowed. Please upload allowed file types only."
    }

    if strings.Contains(err.Error(), "virus detected") {
        return "File contains virus. Upload blocked for security."
    }

    if strings.Contains(err.Error(), "disk full") {
        return "Storage space full. Please contact administrator."
    }

    if strings.Contains(err.Error(), "network error") {
        return "Network error. Please check your connection and try again."
    }

    if strings.Contains(err.Error(), "timeout") {
        return "Upload timeout. Please try again with smaller file or better network."
    }

    return "Upload failed. Please try again later."
}

// 重试逻辑
func uploadWithSmartRetry(manager *upload.UploadManager, file *multipart.FileHeader, options *upload.UploadOptions) (*upload.FileInfo, error) {
    maxRetries := 3
    var lastErr error

    for attempt := 1; attempt <= maxRetries; attempt++ {
        fileInfo, err := manager.Upload(context.Background(), file, options)
        if err == nil {
            return fileInfo, nil
        }

        lastErr = err

        // 根据错误类型决定是否重试
        if shouldRetry(err) {
            // 等待重试
            delay := calculateRetryDelay(attempt)
            time.Sleep(delay)
            continue
        }

        // 不可重试的错误
        break
    }

    return nil, lastErr
}

func shouldRetry(err error) bool {
    // 网络错误、超时错误可以重试
    retryableErrors := []string{
        "network error",
        "timeout",
        "connection reset",
        "temporary failure",
    }

    errStr := strings.ToLower(err.Error())
    for _, retryable := range retryableErrors {
        if strings.Contains(errStr, retryable) {
            return true
        }
    }

    return false
}

func calculateRetryDelay(attempt int) time.Duration {
    // 指数退避 + 随机抖动
    baseDelay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
    jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
    return baseDelay + jitter
}
```

#### 2. 性能问题诊断

```go
// 性能监控
type UploadMetrics struct {
    TotalRequests      int64
    SuccessfulUploads  int64
    FailedUploads      int64
    TotalBytes         int64
    AverageDuration    time.Duration
    ErrorRate          float64
}

func monitorUploadPerformance() {
    metrics := &UploadMetrics{}
    startTime := time.Now()

    // 在中间件中收集指标
    router.Use(func(c *gin.Context) {
        if strings.Contains(c.Request.URL.Path, "/upload") {
            atomic.AddInt64(&metrics.TotalRequests, 1)
            uploadStart := time.Now()

            c.Next()

            duration := time.Since(uploadStart)

            if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
                atomic.AddInt64(&metrics.SuccessfulUploads, 1)
            } else {
                atomic.AddInt64(&metrics.FailedUploads, 1)
            }

            // 更新平均持续时间
            oldAvg := metrics.AverageDuration
            count := metrics.SuccessfulUploads + metrics.FailedUploads
            metrics.AverageDuration = time.Duration(
                (float64(oldAvg)*float64(count-1) + float64(duration)) / float64(count),
            )
        }
    })

    // 定期报告指标
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            total := metrics.TotalRequests
            success := metrics.SuccessfulUploads
            failed := metrics.FailedUploads

            errorRate := 0.0
            if total > 0 {
                errorRate = float64(failed) / float64(total) * 100
            }

            logger.Info("Upload performance metrics",
                logger.Int64("total_requests", total),
                logger.Int64("successful", success),
                logger.Int64("failed", failed),
                logger.Float64("error_rate", errorRate),
                logger.Duration("avg_duration", metrics.AverageDuration),
                logger.Int64("total_bytes", metrics.TotalBytes),
            )
        }
    }()
}
```

这个文件上传组件提供了完整的生产级解决方案，支持多种存储后端、文件验证、病毒扫描、分片上传等功能。您可以根据实际需求调整配置和使用方式。