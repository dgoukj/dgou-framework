package response

import (
	"context"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构体
type Response struct {
	RequestID string      `json:"request_id,omitempty"` // 请求ID
	Code      int         `json:"code"`                 // 业务状态码
	Message   string      `json:"message"`              // 消息
	Data      interface{} `json:"data,omitempty"`       // 数据
	Timestamp int64       `json:"timestamp"`            // 时间戳
}

// Pagination 分页信息
type Pagination struct {
	Page      int   `json:"page"`       // 当前页
	PageSize  int   `json:"page_size"`  // 每页大小
	Total     int64 `json:"total"`      // 总记录数
	TotalPage int   `json:"total_page"` // 总页数
}

// PageResponse 分页响应
type PageResponse struct {
	List       interface{} `json:"list"`                 // 数据列表
	Pagination *Pagination `json:"pagination,omitempty"` // 分页信息
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
		Timestamp: getTimestamp(),
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

	pageResponse := PageResponse{
		List:       data,
		Pagination: pagination,
	}

	Success(c, pageResponse)
}

// Error 错误响应
func Error(c *gin.Context, err error) {
	var requestID string
	if id, exists := c.Get("request_id"); exists {
		requestID = id.(string)
	}

	// 记录错误日志
	logError(c, err)

	// 处理不同类型的错误
	switch e := err.(type) {
	case *errors.Error:
		// 自定义错误
		response := Response{
			RequestID: requestID,
			Code:      int(e.Code),
			Message:   e.Message,
			Timestamp: getTimestamp(),
		}

		// 开发环境显示详细错误信息
		if gin.Mode() == gin.DebugMode && (e.Details != nil || e.Stack != nil) {
			debugInfo := map[string]interface{}{
				"details": e.Details,
				"stack":   e.Stack,
			}
			response.Data = debugInfo
		}

		c.JSON(e.HTTPCode, response)

	default:
		// 通用错误
		c.JSON(http.StatusInternalServerError, Response{
			RequestID: requestID,
			Code:      http.StatusInternalServerError,
			Message:   "Internal server error",
			Timestamp: getTimestamp(),
		})
	}
}

// HTTPError HTTP错误响应快捷方法
func HTTPError(c *gin.Context, code int, message string) {
	Error(c, &errors.Error{
		Code:     errors.ErrorCode(code),
		HTTPCode: code,
		Message:  message,
	})
}

// BadRequest 400错误响应
func BadRequest(c *gin.Context, message string) {
	HTTPError(c, http.StatusBadRequest, message)
}

// Unauthorized 401错误响应
func Unauthorized(c *gin.Context, message string) {
	HTTPError(c, http.StatusUnauthorized, message)
}

// Forbidden 403错误响应
func Forbidden(c *gin.Context, message string) {
	HTTPError(c, http.StatusForbidden, message)
}

// NotFound 404错误响应
func NotFound(c *gin.Context, message string) {
	HTTPError(c, http.StatusNotFound, message)
}

// TooManyRequests 429错误响应
func TooManyRequests(c *gin.Context, message string) {
	HTTPError(c, http.StatusTooManyRequests, message)
}

// InternalServerError 500错误响应
func InternalServerError(c *gin.Context, message string) {
	HTTPError(c, http.StatusInternalServerError, message)
}

// ValidationError 验证错误响应
func ValidationError(c *gin.Context, field, message string) {
	BadRequest(c, errors.ValidationError(field, message).Error())
}

// ==================== 辅助方法 ====================

// getTimestamp 获取当前时间戳
func getTimestamp() int64 {
	// 如果需要毫秒级时间戳，可以使用 time.Now().UnixNano() / 1e6
	return 0 // 这里返回0，实际使用时应该返回真实时间戳
}

// logError 记录错误日志
func logError(c *gin.Context, err error) {
	// 获取请求信息
	path := c.Request.URL.Path
	method := c.Request.Method
	clientIP := c.ClientIP()

	// 获取请求ID
	var requestID string
	if id, exists := c.Get("request_id"); exists {
		requestID = id.(string)
	}

	// 获取用户信息
	var userID string
	if id, exists := c.Get("user_id"); exists {
		userID = id.(string)
	}

	// 创建日志上下文
	ctx := c.Request.Context()
	if requestID != "" {
		ctx = context.WithValue(ctx, logger.RequestIDKey, requestID)
	}
	if userID != "" {
		ctx = context.WithValue(ctx, logger.UserIDKey, userID)
	}

	// 记录错误日志
	logger.CtxError(ctx, "Request error",
		logger.String("method", method),
		logger.String("path", path),
		logger.String("client_ip", clientIP),
		logger.Any("error", err),
	)

	// 如果是严重错误，记录堆栈信息
	if isCriticalError(err) {
		stack := captureStack()
		logger.CtxError(ctx, "Critical error stack",
			logger.Any("stack", stack),
		)
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

		// 获取函数名
		fn := runtime.FuncForPC(pc)
		funcName := "unknown"
		if fn != nil {
			funcName = fn.Name()
		}

		stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, funcName))
	}

	return stack
}

// isCriticalError 检查是否是严重错误
func isCriticalError(err error) bool {
	if customErr, ok := err.(*errors.Error); ok {
		return customErr.HTTPCode >= http.StatusInternalServerError
	}
	return false
}

// ==================== 文件响应 ====================

// File 文件响应
func File(c *gin.Context, filepath, filename string) {
	c.FileAttachment(filepath, filename)
}

// JSONFile JSON文件响应
func JSONFile(c *gin.Context, data interface{}, filename string) {
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.JSON(http.StatusOK, data)
}

// CSVFile CSV文件响应
func CSVFile(c *gin.Context, content, filename string) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.String(http.StatusOK, content)
}
