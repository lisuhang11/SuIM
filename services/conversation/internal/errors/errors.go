// Package errors 定义统一的 AppError 类型及会话服务的错误码。
package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode 为每个已知错误条件分配数字标识。
type ErrorCode int

const (
	// --- 通用错误 1000-1099 ---
	CodeUnknown    ErrorCode = 1000 // 未知错误
	CodeValidation ErrorCode = 1001 // 参数校验错误
	CodeInternal   ErrorCode = 1002 // 内部服务错误

	// --- 会话错误 4100-4199 ---
	CodeConversationNotFound ErrorCode = 4100 // 会话不存在
)

// AppError 统一错误类型，包含机器可读的错误码和人类可读的消息。
type AppError struct {
	Code    ErrorCode // 错误码
	Message string    // 错误消息
	Err     error     // 底层原始错误
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

// Unwrap 支持 errors.Is / errors.As 检查包装的错误。
func (e *AppError) Unwrap() error { return e.Err }

// IsAppError 检查 err 是否为（或包装了）*AppError。
func IsAppError(err error) bool {
	var ae *AppError
	return errors.As(err, &ae)
}

// GetAppError 解包链中第一个 *AppError，不存在则返回 nil。
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

// NewConversationNotFoundError 创建会话不存在错误。
func NewConversationNotFoundError() *AppError {
	return &AppError{Code: CodeConversationNotFound, Message: "conversation not found"}
}

// WithDetails 附加底层错误信息用于调试。
func (e *AppError) WithDetails(err error) *AppError {
	e.Err = err
	return e
}
