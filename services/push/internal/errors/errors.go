// Package errors 定义 push 服务使用的统一 AppError 类型和错误码。
package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode 错误码数值标识。
type ErrorCode int

const (
	// --- 通用错误 1000-1099 ---
	CodeUnknown    ErrorCode = 1000 // 未知错误
	CodeValidation ErrorCode = 1001 // 参数校验错误
	CodeInternal   ErrorCode = 1002 // 内部错误
	CodeNotFound   ErrorCode = 1003 // 未找到

	// --- push 错误 4300-4399 ---
	CodePushFailed      ErrorCode = 4300 // 推送失败
	CodeTokenNotFound   ErrorCode = 4301 // 令牌未找到
)

// AppError 统一错误类型。
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

// Error 实现 error 接口。
func (e *AppError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%d] %s", e.Code, e.Message)
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	return b.String()
}

// Unwrap 支持 errors.Is / errors.As。
func (e *AppError) Unwrap() error { return e.Err }

// GetAppError 解包错误链中的第一个 *AppError。
func GetAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}

// ---------- 构造函数 ----------

// NewValidationError 创建参数校验错误。
func NewValidationError(msg string) *AppError {
	return &AppError{Code: CodeValidation, Message: msg}
}

// NewInternalError 创建内部错误。
func NewInternalError(msg string) *AppError {
	return &AppError{Code: CodeInternal, Message: msg}
}

// NewPushFailedError 创建推送失败错误。
func NewPushFailedError(msg string) *AppError {
	return &AppError{Code: CodePushFailed, Message: msg}
}

// NewTokenNotFoundError 创建令牌未找到错误。
func NewTokenNotFoundError() *AppError {
	return &AppError{Code: CodeTokenNotFound, Message: "push token not found"}
}

// WithDetails 附加底层错误。
func (e *AppError) WithDetails(err error) *AppError {
	e.Err = err
	return e
}
