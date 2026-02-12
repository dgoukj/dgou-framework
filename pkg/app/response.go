package app

import (
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Response 统一响应结构体
type Response struct {
	RequestID string      `json:"request_id,omitempty"`
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// Pagination 分页信息
type Pagination struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int   `json:"total_page"`
}

// PageResponse 分页响应
type PageResponse struct {
	List       interface{} `json:"list"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	var requestID string
	if id, exists := c.Get("request_id"); exists {
		requestID = id.(string)
	}
	c.JSON(http.StatusOK, Response{
		RequestID: requestID,
		Code:      http.StatusOK,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().Unix(),
	})
}

// SuccessWithPagination 成功响应（带分页）
func SuccessWithPagination(c *gin.Context, data interface{}, page, pageSize int, total int64) {
	totalPage := int((total + int64(pageSize) - 1) / int64(pageSize))
	pagination := &Pagination{
		Page:      page,
		PageSize:  pageSize,
		Total:     total,
		TotalPage: totalPage,
	}
	Success(c, PageResponse{List: data, Pagination: pagination})
}

// Error 错误响应（需传入 logger 实例）
func Error(c *gin.Context, log *logger.Logger, err error) {
	var requestID string
	if id, exists := c.Get("request_id"); exists {
		requestID = id.(string)
	}
	// 记录错误日志
	logError(c, log, err)

	switch e := err.(type) {
	case *errors.Error:
		resp := Response{
			RequestID: requestID,
			Code:      int(e.Code),
			Message:   e.Message,
			Timestamp: time.Now().Unix(),
		}
		// 开发环境返回详细错误信息
		if gin.Mode() == gin.DebugMode && (e.Details != nil || e.Stack != nil) {
			resp.Data = map[string]interface{}{
				"details": e.Details,
				"stack":   e.Stack,
			}
		}
		c.JSON(e.HTTPCode, resp)
	default:
		c.JSON(http.StatusInternalServerError, Response{
			RequestID: requestID,
			Code:      http.StatusInternalServerError,
			Message:   "Internal server error",
			Timestamp: time.Now().Unix(),
		})
	}
}

// ==================== HTTP错误快捷方法（需传入 logger） ====================

// HTTPError HTTP错误响应
func HTTPError(c *gin.Context, log *logger.Logger, code int, message string) {
	Error(c, log, &errors.Error{
		Code:     errors.ErrorCode(code),
		HTTPCode: code,
		Message:  message,
	})
}

// BadRequest 400错误
func BadRequest(c *gin.Context, log *logger.Logger, message string) {
	HTTPError(c, log, http.StatusBadRequest, message)
}

// Unauthorized 401错误
func Unauthorized(c *gin.Context, log *logger.Logger, message string) {
	HTTPError(c, log, http.StatusUnauthorized, message)
}

// Forbidden 403错误
func Forbidden(c *gin.Context, log *logger.Logger, message string) {
	HTTPError(c, log, http.StatusForbidden, message)
}

// NotFound 404错误
func NotFound(c *gin.Context, log *logger.Logger, message string) {
	HTTPError(c, log, http.StatusNotFound, message)
}

// TooManyRequests 429错误
func TooManyRequests(c *gin.Context, log *logger.Logger, message string) {
	HTTPError(c, log, http.StatusTooManyRequests, message)
}

// InternalServerError 500错误
func InternalServerError(c *gin.Context, log *logger.Logger, message string) {
	HTTPError(c, log, http.StatusInternalServerError, message)
}

// ValidationError 验证错误
func ValidationError(c *gin.Context, log *logger.Logger, field, message string) {
	err := errors.New(errors.CodeValidationFailed, fmt.Sprintf("Validation failed for field '%s': %s", field, message))
	Error(c, log, err)
}

// ==================== 辅助方法 ====================

// logError 记录错误日志
func logError(c *gin.Context, log *logger.Logger, err error) {
	fields := []zap.Field{
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("client_ip", c.ClientIP()),
		zap.Error(err),
	}
	if requestID, exists := c.Get("request_id"); exists {
		fields = append(fields, zap.String("request_id", requestID.(string)))
	}
	if userID, exists := c.Get("user_id"); exists {
		fields = append(fields, zap.Uint64("user_id", userID.(uint64)))
	}
	log.Error("Request error", fields...)

	// 严重错误记录堆栈
	if isCriticalError(err) {
		stack := captureStack()
		log.Error("Critical error stack", zap.Strings("stack", stack))
	}
}

// captureStack 捕获调用堆栈
func captureStack() []string {
	const depth = 20
	var stack []string
	for i := 0; i < depth; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		funcName := "unknown"
		if fn != nil {
			funcName = fn.Name()
		}
		stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, funcName))
	}
	return stack
}

// isCriticalError 检查是否为严重错误（HTTP状态码 >= 500）
func isCriticalError(err error) bool {
	if customErr, ok := err.(*errors.Error); ok {
		return customErr.HTTPCode >= http.StatusInternalServerError
	}
	return false
}

// ==================== 文件响应 ====================

// File 文件下载
func File(c *gin.Context, filepath, filename string) {
	c.FileAttachment(filepath, filename)
}

// JSONFile 返回JSON格式的文件下载
func JSONFile(c *gin.Context, data interface{}, filename string) {
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.JSON(http.StatusOK, data)
}

// CSVFile 返回CSV格式的文件下载
func CSVFile(c *gin.Context, content, filename string) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.String(http.StatusOK, content)
}
