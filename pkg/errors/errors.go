package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
)

// ErrorCode 错误码类型
type ErrorCode int

const (
	// 通用错误码
	CodeUnknown          ErrorCode = 1000
	CodeValidationFailed ErrorCode = 1001
	CodeResourceNotFound ErrorCode = 1002
	CodeUnauthorized     ErrorCode = 1003
	CodeForbidden        ErrorCode = 1004
	CodeTooManyRequests  ErrorCode = 1005
	CodeInternalError    ErrorCode = 1006

	// 业务错误码（可以根据业务扩展）
	CodeUserExists      ErrorCode = 2001
	CodeUserNotFound    ErrorCode = 2002
	CodeInvalidPassword ErrorCode = 2003
	CodeTokenExpired    ErrorCode = 2004
	CodeTokenInvalid    ErrorCode = 2005
)

// Error 自定义错误结构体
type Error struct {
	Code     ErrorCode `json:"code"`              // 业务错误码
	HTTPCode int       `json:"http_code"`         // HTTP状态码
	Message  string    `json:"message"`           // 错误信息
	Details  []string  `json:"details,omitempty"` // 错误详情
	Stack    []string  `json:"stack,omitempty"`   // 调用堆栈（仅开发环境）
	Op       string    `json:"op,omitempty"`      // 操作路径
	Err      error     `json:"-"`                 // 原始错误
}

// Error 实现error接口
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap 实现错误解包
func (e *Error) Unwrap() error {
	return e.Err
}

// New 创建新的自定义错误
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:     code,
		HTTPCode: getHTTPStatusCode(code),
		Message:  message,
		Stack:    captureStack(),
	}
}

// Wrap 包装现有错误
func Wrap(err error, code ErrorCode, message string) *Error {
	if err == nil {
		return nil
	}

	// 如果已经是自定义错误，直接返回
	if customErr, ok := err.(*Error); ok {
		return customErr
	}

	return &Error{
		Code:     code,
		HTTPCode: getHTTPStatusCode(code),
		Message:  message,
		Err:      err,
		Stack:    captureStack(),
	}
}

// Wrapf 格式化包装错误
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *Error {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// WithOp 添加操作路径
func (e *Error) WithOp(op string) *Error {
	e.Op = op
	return e
}

// WithDetails 添加错误详情
func (e *Error) WithDetails(details ...string) *Error {
	e.Details = append(e.Details, details...)
	return e
}

// ToJSON 转换为JSON字符串
func (e *Error) ToJSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

// ==================== HTTP错误快捷方法 ====================

// BadRequest 创建400错误
func BadRequest(message string) *Error {
	return New(CodeValidationFailed, message).WithHTTPCode(http.StatusBadRequest)
}

// Unauthorized 创建401错误
func Unauthorized(message string) *Error {
	return New(CodeUnauthorized, message).WithHTTPCode(http.StatusUnauthorized)
}

// Forbidden 创建403错误
func Forbidden(message string) *Error {
	return New(CodeForbidden, message).WithHTTPCode(http.StatusForbidden)
}

// NotFound 创建404错误
func NotFound(message string) *Error {
	return New(CodeResourceNotFound, message).WithHTTPCode(http.StatusNotFound)
}

// TooManyRequests 创建429错误
func TooManyRequests(message string) *Error {
	return New(CodeTooManyRequests, message).WithHTTPCode(http.StatusTooManyRequests)
}

// InternalServerError 创建500错误
func InternalServerError(message string) *Error {
	return New(CodeInternalError, message).WithHTTPCode(http.StatusInternalServerError)
}

// ValidationError 创建验证错误
func ValidationError(field, message string) *Error {
	return BadRequest(fmt.Sprintf("Validation failed for field '%s': %s", field, message))
}

// ==================== 业务错误快捷方法 ====================

// UserExists 用户已存在错误
func UserExists(username string) *Error {
	return New(CodeUserExists, fmt.Sprintf("User '%s' already exists", username))
}

// UserNotFound 用户不存在错误
func UserNotFound(username string) *Error {
	return New(CodeUserNotFound, fmt.Sprintf("User '%s' not found", username))
}

// InvalidPassword 密码错误
func InvalidPassword() *Error {
	return New(CodeInvalidPassword, "Invalid password")
}

// TokenExpired Token过期错误
func TokenExpired() *Error {
	return New(CodeTokenExpired, "Token has expired")
}

// TokenInvalid Token无效错误
func TokenInvalid() *Error {
	return New(CodeTokenInvalid, "Token is invalid")
}

// ==================== 辅助方法 ====================

// WithHTTPCode 设置HTTP状态码
func (e *Error) WithHTTPCode(code int) *Error {
	e.HTTPCode = code
	return e
}

// getHTTPStatusCode 根据错误码获取HTTP状态码
func getHTTPStatusCode(code ErrorCode) int {
	switch code {
	case CodeValidationFailed:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeResourceNotFound:
		return http.StatusNotFound
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	case CodeInternalError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// captureStack 捕获调用堆栈
func captureStack() []string {
	const depth = 10
	var stack []string

	for i := 0; i < depth; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		stack = append(stack, fmt.Sprintf("%s:%d", file, line))
	}

	return stack
}

// Is 检查错误是否匹配
func Is(err error, target error) bool {
	if customErr, ok := err.(*Error); ok {
		if targetErr, ok := target.(*Error); ok {
			return customErr.Code == targetErr.Code
		}
	}
	return false
}

// As 将错误转换为指定类型
func As(err error, target interface{}) bool {
	switch t := target.(type) {
	case **Error:
		if customErr, ok := err.(*Error); ok {
			*t = customErr
			return true
		}
	}
	return false
}
