// Package errors 定义消息服务使用的统一 AppError 类型和错误码。
package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode 错误码数值标识，每个已知错误条件对应一个唯一编码。
type ErrorCode int

const (
	// --- 通用错误 1000-1099 ---
	CodeUnknown    ErrorCode = 1000 // 未知错误
	CodeValidation ErrorCode = 1001 // 参数校验错误
	CodeInternal   ErrorCode = 1002 // 内部错误
	CodeNotFound   ErrorCode = 1003 // 未找到

	// --- 消息错误 4200-4299 ---
	CodeMessageNotFound   ErrorCode = 4200 // 消息未找到
	CodeRevokePermission ErrorCode = 4201 // 撤回权限不足
	CodeInvalidMessage   ErrorCode = 4202 // 无效消息
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

// IsAppError 检查 err 是否为 *AppError。
func IsAppError(err error) bool {
	var ae *AppError
	return errors.As(err, &ae)
}

// GetAppError 解包错误链中的第一个 *AppError，不存在则返回 nil。
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

// NewMessageNotFoundError 创建消息未找到错误。
func NewMessageNotFoundError() *AppError {
	return &AppError{Code: CodeMessageNotFound, Message: "message not found"}
}

// NewRevokePermissionError 创建撤回权限不足错误。
func NewRevokePermissionError() *AppError {
	return &AppError{Code: CodeRevokePermission, Message: "only the sender can revoke this message"}
}

// NewInvalidMessageError 创建无效消息错误。
func NewInvalidMessageError(msg string) *AppError {
	return &AppError{Code: CodeInvalidMessage, Message: msg}
}

// WithDetails 附加底层错误用于调试。
func (e *AppError) WithDetails(err error) *AppError {
	e.Err = err
	return e
}
