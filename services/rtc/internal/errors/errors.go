// Package errors 定义 rtc 服务使用的统一 AppError 类型和错误码。
package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode 错误码数值标识。
type ErrorCode int

const (
	CodeUnknown        ErrorCode = 1000
	CodeValidation     ErrorCode = 1001
	CodeInternal       ErrorCode = 1002
	CodeNotFound       ErrorCode = 1003
	CodeNotFriend      ErrorCode = 4700
	CodeNotParticipant ErrorCode = 4701
	CodeUnavailable    ErrorCode = 4702
	CodeBusy           ErrorCode = 4703
	CodeInvalidState   ErrorCode = 4704
)

// AppError 统一错误类型。
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%d] %s", e.Code, e.Message)
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	return b.String()
}

func (e *AppError) Unwrap() error { return e.Err }

// GetAppError 解包错误链中的第一个 *AppError。
func GetAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}

func NewValidationError(msg string) *AppError {
	return &AppError{Code: CodeValidation, Message: msg}
}

func NewInternalError(msg string) *AppError {
	return &AppError{Code: CodeInternal, Message: msg}
}

func NewNotFoundError(msg string) *AppError {
	return &AppError{Code: CodeNotFound, Message: msg}
}

func NewNotFriendError() *AppError {
	return &AppError{Code: CodeNotFriend, Message: "not_friend"}
}

func NewNotParticipantError() *AppError {
	return &AppError{Code: CodeNotParticipant, Message: "not a call participant"}
}

func NewUnavailableError() *AppError {
	return &AppError{Code: CodeUnavailable, Message: "unavailable"}
}

func NewBusyError() *AppError {
	return &AppError{Code: CodeBusy, Message: "busy"}
}

func NewInvalidStateError(msg string) *AppError {
	return &AppError{Code: CodeInvalidState, Message: msg}
}

func (e *AppError) WithDetails(err error) *AppError {
	e.Err = err
	return e
}
