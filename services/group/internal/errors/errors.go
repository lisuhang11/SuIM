// Package errors 定义统一的 AppError 类型及群组服务的错误码。
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

	// --- 群组错误 3100-3199 ---
	CodeGroupNotFound         ErrorCode = 3100 // 群组不存在
	CodeMemberNotFound        ErrorCode = 3101 // 成员不存在
	CodeNotGroupOwner         ErrorCode = 3102 // 非群主
	CodeAlreadyMember         ErrorCode = 3103 // 已是群成员
	CodeNotMember             ErrorCode = 3104 // 非群成员
	CodeCannotQuitAsOwner     ErrorCode = 3105 // 群主不能退出
	CodeAlreadyRequested      ErrorCode = 3106 // 已存在待处理的请求
	CodeRequestNotFound       ErrorCode = 3107 // 入群请求不存在
	CodeRequestAlreadyHandled ErrorCode = 3108 // 入群请求已处理
	CodeCannotKickRole        ErrorCode = 3109 // 不能踢出同级或更高级别成员
	CodeUserNotExist          ErrorCode = 3110 // 用户不存在
	CodeNotAuthorized         ErrorCode = 3111 // 无操作权限
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

// NewGroupNotFoundError 创建群组不存在错误。
func NewGroupNotFoundError() *AppError {
	return &AppError{Code: CodeGroupNotFound, Message: "group not found"}
}

// NewMemberNotFoundError 创建成员不存在错误。
func NewMemberNotFoundError() *AppError {
	return &AppError{Code: CodeMemberNotFound, Message: "group member not found"}
}

// NewNotGroupOwnerError 创建非群主错误。
func NewNotGroupOwnerError() *AppError {
	return &AppError{Code: CodeNotGroupOwner, Message: "only the group owner can perform this operation"}
}

// NewAlreadyMemberError 创建已是群成员错误。
func NewAlreadyMemberError() *AppError {
	return &AppError{Code: CodeAlreadyMember, Message: "user is already a group member"}
}

// NewNotMemberError 创建非群成员错误。
func NewNotMemberError() *AppError {
	return &AppError{Code: CodeNotMember, Message: "user is not a group member"}
}

// NewCannotQuitAsOwnerError 创建群主不能退出错误。
func NewCannotQuitAsOwnerError() *AppError {
	return &AppError{Code: CodeCannotQuitAsOwner, Message: "group owner cannot quit; transfer ownership first"}
}

// NewAlreadyRequestedError 创建已存在待处理请求错误。
func NewAlreadyRequestedError() *AppError {
	return &AppError{Code: CodeAlreadyRequested, Message: "a pending join request already exists"}
}

// NewRequestNotFoundError 创建入群请求不存在错误。
func NewRequestNotFoundError() *AppError {
	return &AppError{Code: CodeRequestNotFound, Message: "group request not found"}
}

// NewRequestAlreadyHandledError 创建入群请求已处理错误。
func NewRequestAlreadyHandledError() *AppError {
	return &AppError{Code: CodeRequestAlreadyHandled, Message: "group request already handled"}
}

// NewCannotKickRoleError 创建不能踢出同级/更高级别成员错误。
func NewCannotKickRoleError() *AppError {
	return &AppError{Code: CodeCannotKickRole, Message: "cannot kick a member with equal or higher role"}
}

// NewUserNotExistError 创建用户不存在错误。
func NewUserNotExistError() *AppError {
	return &AppError{Code: CodeUserNotExist, Message: "user does not exist"}
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
