package errors

import (
	"fmt"
	"runtime"
)

type ErrorCode int

const (
	CodeUnknown          ErrorCode = 1000
	CodeValidationFailed ErrorCode = 1001
	CodeResourceNotFound ErrorCode = 1002
	CodeUnauthorized     ErrorCode = 1003
	CodeForbidden        ErrorCode = 1004
	CodeTooManyRequests  ErrorCode = 1005
	CodeInternalError    ErrorCode = 1006
)

type Error struct {
	Code     ErrorCode `json:"code"`
	HTTPCode int       `json:"http_code"`
	Message  string    `json:"message"`
	Details  []string  `json:"details,omitempty"`
	Stack    []string  `json:"stack,omitempty"`
	Op       string    `json:"op,omitempty"`
	Err      error     `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:     code,
		HTTPCode: getHTTPStatusCode(code),
		Message:  message,
		Stack:    captureStack(2),
	}
}

func Wrap(err error, code ErrorCode, message string) *Error {
	if err == nil {
		return nil
	}
	if custom, ok := err.(*Error); ok {
		return custom
	}
	return &Error{
		Code:     code,
		HTTPCode: getHTTPStatusCode(code),
		Message:  message,
		Err:      err,
		Stack:    captureStack(2),
	}
}

func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *Error {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

func (e *Error) WithOp(op string) *Error {
	e.Op = op
	return e
}

func (e *Error) WithDetails(details ...string) *Error {
	e.Details = append(e.Details, details...)
	return e
}

func (e *Error) WithHTTPCode(code int) *Error {
	e.HTTPCode = code
	return e
}

func captureStack(skip int) []string {
	const depth = 32
	var stack []string
	for i := skip; i < depth; i++ {
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

func getHTTPStatusCode(code ErrorCode) int {
	switch code {
	case CodeValidationFailed:
		return 400
	case CodeUnauthorized:
		return 401
	case CodeForbidden:
		return 403
	case CodeResourceNotFound:
		return 404
	case CodeTooManyRequests:
		return 429
	default:
		return 500
	}
}

// Is 支持错误码比较
func Is(err error, target *Error) bool {
	if custom, ok := err.(*Error); ok {
		return custom.Code == target.Code
	}
	return false
}
