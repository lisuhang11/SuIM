// Package errors 定义统一的 AppError 类型及错误码，供整个服务使用。
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

	// --- 认证错误 1100-1199 ---
	CodeUnauthorized ErrorCode = 1101 // 未授权
	CodeForbidden    ErrorCode = 1103 // 禁止访问

	// --- 用户错误 2000-2099 ---
	CodeUserNotFound    ErrorCode = 2000 // 用户不存在
	CodeUserExists      ErrorCode = 2001 // 用户已存在
	CodePasswordInvalid ErrorCode = 2002 // 密码无效
	CodePasswordPolicy  ErrorCode = 2003 // 密码策略不满足
	CodeUserInactive    ErrorCode = 2004 // 用户被禁用

	// --- 令牌错误 2100-2199 ---
	CodeTokenInvalid   ErrorCode = 2100 // 令牌无效
	CodeTokenExpired   ErrorCode = 2101 // 令牌过期
	CodeTokenRevoked   ErrorCode = 2102 // 令牌已吊销
	CodeTokenWrongType ErrorCode = 2103 // 令牌类型错误
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

// NewUnauthorizedError 创建未授权错误。
func NewUnauthorizedError(msg string) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: msg}
}

// NewUserNotFoundError 创建用户不存在错误。
func NewUserNotFoundError() *AppError {
	return &AppError{Code: CodeUserNotFound, Message: "user not found"}
}

// NewUserExistsError 创建用户已存在错误。
func NewUserExistsError() *AppError {
	return &AppError{Code: CodeUserExists, Message: "user already exists"}
}

// NewPasswordInvalidError 创建密码无效错误。
func NewPasswordInvalidError() *AppError {
	return &AppError{Code: CodePasswordInvalid, Message: "invalid password"}
}

// NewPasswordPolicyError 创建密码策略不满足错误。
func NewPasswordPolicyError() *AppError {
	return &AppError{Code: CodePasswordPolicy, Message: "password must be 8-32 characters with at least one letter and one number"}
}

// NewUserInactiveError 创建用户被禁用错误。
func NewUserInactiveError() *AppError {
	return &AppError{Code: CodeUserInactive, Message: "account is disabled"}
}

// NewTokenInvalidError 创建令牌无效错误。
func NewTokenInvalidError() *AppError {
	return &AppError{Code: CodeTokenInvalid, Message: "invalid token"}
}

// NewTokenExpiredError 创建令牌过期错误。
func NewTokenExpiredError() *AppError {
	return &AppError{Code: CodeTokenExpired, Message: "token expired"}
}

// NewTokenRevokedError 创建令牌已吊销错误。
func NewTokenRevokedError() *AppError {
	return &AppError{Code: CodeTokenRevoked, Message: "token is revoked"}
}

// NewTokenWrongTypeError 创建令牌类型错误。
func NewTokenWrongTypeError() *AppError {
	return &AppError{Code: CodeTokenWrongType, Message: "token type mismatch"}
}

// WithDetails 附加底层错误信息用于调试。
func (e *AppError) WithDetails(err error) *AppError {
	e.Err = err
	return e
}
