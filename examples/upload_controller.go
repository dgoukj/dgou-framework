// file: examples/upload_controller.go
package examples

import (
	"dgou/pkg/response"
	"dgou/pkg/upload"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// UploadController 上传控制器示例
type UploadController struct {
	uploadService *upload.UploadService
}

// NewUploadController 创建上传控制器
func NewUploadController(uploadService *upload.UploadService) *UploadController {
	return &UploadController{
		uploadService: uploadService,
	}
}

// UploadSingle 上传单个文件
func (c *UploadController) UploadSingle(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		response.BadRequest(ctx, "No file uploaded")
		return
	}

	req := &upload.UploadRequest{
		File:         file,
		UploaderID:   ctx.GetString("user_id"),
		UploaderIP:   ctx.ClientIP(),
		IsPublic:     ctx.DefaultQuery("public", "true") == "true",
		Category:     ctx.DefaultQuery("category", "default"),
		SubDirectory: ctx.DefaultQuery("subdir", ""),
		CustomPath:   ctx.Query("path"),
	}

	resp, err := c.uploadService.UploadSingle(ctx.Request.Context(), req)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	response.Success(ctx, gin.H{
		"file": resp.FileInfo,
		"url":  resp.FileInfo.URL,
	})
}

// GetFile 获取文件
func (c *UploadController) GetFile(ctx *gin.Context) {
	path := ctx.Param("path")
	if path == "" {
		response.BadRequest(ctx, "File path is required")
		return
	}

	// 检查文件是否存在
	exists, err := c.uploadService.FileExists(ctx.Request.Context(), path)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	if !exists {
		response.NotFound(ctx, "File not found")
		return
	}

	// 获取文件信息
	fileInfo, err := c.uploadService.GetFileInfo(ctx.Request.Context(), path)
	if err != nil {
		response.Error(ctx, err)
		return
	}

	// 如果是图片，直接返回URL重定向
	if strings.HasPrefix(fileInfo.MimeType, "image/") {
		url, err := c.uploadService.GetFileURL(ctx.Request.Context(), path, true)
		if err != nil {
			response.Error(ctx, err)
			return
		}

		ctx.Redirect(http.StatusFound, url)
		return
	}

	// 获取文件内容
	reader, err := c.uploadService.manager.GetFile(ctx.Request.Context(), path)
	if err != nil {
		response.Error(ctx, err)
		return
	}
	defer reader.Close()

	// 设置响应头
	ctx.Header("Content-Type", fileInfo.MimeType)
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.Name))
	ctx.Header("Content-Length", strconv.FormatInt(fileInfo.Size, 10))

	// 流式传输文件内容
	ctx.Stream(func(w io.Writer) bool {
		_, err := io.Copy(w, reader)
		return err == nil
	})
}

// RegisterRoutes 注册路由（业务方调用）
func (c *UploadController) RegisterRoutes(router *gin.RouterGroup) {
	uploadGroup := router.Group("/upload")
	{
		uploadGroup.POST("/single", c.UploadSingle)
		uploadGroup.GET("/files/:path", c.GetFile)
	}
}
