// Package errors defines the unified AppError type and error codes used
// across the service. It mirrors the pattern established in WeKnora.
package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode is a numeric identifier for each known error condition.
type ErrorCode int

const (
	// --- Common errors 1000-1099 ---
	CodeUnknown    ErrorCode = 1000
	CodeValidation ErrorCode = 1001
	CodeInternal   ErrorCode = 1002

	// --- Auth errors 1100-1199 ---
	CodeUnauthorized ErrorCode = 1101
	CodeForbidden    ErrorCode = 1103

	// --- User errors 2000-2099 ---
	CodeUserNotFound    ErrorCode = 2000
	CodeUserExists      ErrorCode = 2001
	CodePasswordInvalid ErrorCode = 2002
	CodePasswordPolicy  ErrorCode = 2003
	CodeUserInactive    ErrorCode = 2004

	// --- Token errors 2100-2199 ---
	CodeTokenInvalid    ErrorCode = 2100
	CodeTokenExpired    ErrorCode = 2101
	CodeTokenRevoked    ErrorCode = 2102
	CodeTokenWrongType  ErrorCode = 2103
)

// AppError is the unified error type with a machine-readable code and a
// human-readable message.
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%d] %s", e.Code, e.Message)
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	return b.String()
}

// Unwrap allows errors.Is / errors.As to inspect the wrapped error.
func (e *AppError) Unwrap() error { return e.Err }

// IsAppError checks whether err is (or wraps) an *AppError.
func IsAppError(err error) bool {
	var ae *AppError
	return errors.As(err, &ae)
}

// GetAppError unwraps the first *AppError in the chain, or nil.
func GetAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}

// ---------- constructors ----------

func NewValidationError(msg string) *AppError {
	return &AppError{Code: CodeValidation, Message: msg}
}

func NewInternalError(msg string) *AppError {
	return &AppError{Code: CodeInternal, Message: msg}
}

func NewUnauthorizedError(msg string) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: msg}
}

func NewUserNotFoundError() *AppError {
	return &AppError{Code: CodeUserNotFound, Message: "user not found"}
}

func NewUserExistsError() *AppError {
	return &AppError{Code: CodeUserExists, Message: "user already exists"}
}

func NewPasswordInvalidError() *AppError {
	return &AppError{Code: CodePasswordInvalid, Message: "invalid password"}
}

func NewPasswordPolicyError() *AppError {
	return &AppError{Code: CodePasswordPolicy, Message: "password must be 8-32 characters with at least one letter and one number"}
}

func NewUserInactiveError() *AppError {
	return &AppError{Code: CodeUserInactive, Message: "account is disabled"}
}

func NewTokenInvalidError() *AppError {
	return &AppError{Code: CodeTokenInvalid, Message: "invalid token"}
}

func NewTokenExpiredError() *AppError {
	return &AppError{Code: CodeTokenExpired, Message: "token expired"}
}

func NewTokenRevokedError() *AppError {
	return &AppError{Code: CodeTokenRevoked, Message: "token is revoked"}
}

func NewTokenWrongTypeError() *AppError {
	return &AppError{Code: CodeTokenWrongType, Message: "token type mismatch"}
}

// WithDetails attaches the underlying error for debugging.
func (e *AppError) WithDetails(err error) *AppError {
	e.Err = err
	return e
}
