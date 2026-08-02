package sdkerrs

import "fmt"

// CodeError is the SDK error contract used by async callbacks.
type CodeError interface {
	error
	Code() int
}

type codeError struct {
	code int
	msg  string
}

func (e *codeError) Error() string { return e.msg }
func (e *codeError) Code() int     { return e.code }

func New(code int, msg string) error {
	return &codeError{code: code, msg: msg}
}

func Wrap(code int, msg string, err error) error {
	if err == nil {
		return New(code, msg)
	}
	return &codeError{code: code, msg: fmt.Sprintf("%s: %v", msg, err)}
}

const (
	UnknownCode      = 10000
	ArgsError        = 10001
	NetworkError     = 10002
	SdkInternalError = 10003
	UserNotFound     = 10004
	NotInit          = 10005
	NotLogin         = 10006
	RecordNotFound   = 10007
)

var (
	ErrArgs         = New(ArgsError, "args error")
	ErrNetwork      = New(NetworkError, "network error")
	ErrSdkInternal  = New(SdkInternalError, "sdk internal error")
	ErrUserNotFound = New(UserNotFound, "user not found")
	ErrNotInit      = New(NotInit, "sdk not init")
	ErrNotLogin     = New(NotLogin, "sdk not login")
	ErrRecordNotFound = New(RecordNotFound, "record not found")
)
