// Package errors 定义统一的 AppError 类型及关系服务的错误码。
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

	// --- 关系错误 3000-3099 ---
	CodeRelationNotFound        ErrorCode = 3000 // 关系不存在
	CodeAlreadyFriends          ErrorCode = 3001 // 已是好友
	CodeAlreadyBlocked          ErrorCode = 3002 // 已被拉黑
	CodeNotBlocked              ErrorCode = 3003 // 未被拉黑
	CodeCannotFriendSelf        ErrorCode = 3004 // 不能添加自己为好友
	CodeFriendRequestNotFound   ErrorCode = 3005 // 好友请求不存在
	CodeAlreadyRequested        ErrorCode = 3006 // 已发送过好友请求
	CodeRequestAlreadyProcessed ErrorCode = 3007 // 好友请求已处理
	CodeNotAuthorized           ErrorCode = 3008 // 无操作权限
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

// NewRelationNotFoundError 创建关系不存在错误。
func NewRelationNotFoundError() *AppError {
	return &AppError{Code: CodeRelationNotFound, Message: "relation not found"}
}

// NewAlreadyFriendsError 创建已是好友错误。
func NewAlreadyFriendsError() *AppError {
	return &AppError{Code: CodeAlreadyFriends, Message: "already friends"}
}

// NewAlreadyBlockedError 创建已被拉黑错误。
func NewAlreadyBlockedError() *AppError {
	return &AppError{Code: CodeAlreadyBlocked, Message: "user is already blocked"}
}

// NewNotBlockedError 创建未被拉黑错误。
func NewNotBlockedError() *AppError {
	return &AppError{Code: CodeNotBlocked, Message: "user is not blocked"}
}

// NewCannotFriendSelfError 创建不能加自己为好友错误。
func NewCannotFriendSelfError() *AppError {
	return &AppError{Code: CodeCannotFriendSelf, Message: "cannot friend yourself"}
}

// NewFriendRequestNotFoundError 创建好友请求不存在错误。
func NewFriendRequestNotFoundError() *AppError {
	return &AppError{Code: CodeFriendRequestNotFound, Message: "friend request not found"}
}

// NewAlreadyRequestedError 创建已发送过请求错误。
func NewAlreadyRequestedError() *AppError {
	return &AppError{Code: CodeAlreadyRequested, Message: "friend request already sent"}
}

// NewRequestAlreadyProcessedError 创建请求已处理错误。
func NewRequestAlreadyProcessedError() *AppError {
	return &AppError{Code: CodeRequestAlreadyProcessed, Message: "friend request already processed"}
}

// NewNotAuthorizedError 创建无操作权限错误。
func NewNotAuthorizedError() *AppError {
	return &AppError{Code: CodeNotAuthorized, Message: "not authorized for this operation"}
}

// WithDetails 附加底层错误信息用于调试。
func (e *AppError) WithDetails(err error) *AppError {
	e.Err = err
	return e
}
