// Package errorcode stores business error codes for every service.
//
//	0         success
//	1000-1999 common
//	2000-2999 user
//
// The next service takes the following thousand.
package errorcode

import (
	"errors"
	"strconv"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const domain = "suim"

// Code is a business error code. Zero means success.
type Code int32

// Error is a sentinel business error. It maps to a standard gRPC status and
// carries Code in the status details so clients can read it after transport.
type Error struct {
	code Code
	grpc codes.Code
	msg  string
}

var byCode = map[Code]*Error{}

// Define registers a sentinel. It panics when code is already registered.
func Define(code Code, grpc codes.Code, msg string) *Error {
	if code == 0 {
		panic("errorcode: code 0 is success")
	}
	if _, ok := byCode[code]; ok {
		panic("errorcode: duplicate code " + strconv.FormatInt(int64(code), 10))
	}
	e := &Error{code: code, grpc: grpc, msg: msg}
	byCode[code] = e
	return e
}

// Lookup returns the sentinel registered for code.
func Lookup(code Code) (*Error, bool) {
	e, ok := byCode[code]
	return e, ok
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.msg
}

func (e *Error) Code() Code { return e.code }

func (e *Error) GRPCCode() codes.Code { return e.grpc }

// WithMsg returns a copy with a more specific message and the same code.
func (e *Error) WithMsg(msg string) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.msg = msg
	return &cp
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e.code == t.code
}

func (e *Error) GRPCStatus() *status.Status {
	st := status.New(e.grpc, e.msg)
	detailed, err := st.WithDetails(&errdetails.ErrorInfo{
		Domain: domain,
		Reason: strconv.FormatInt(int64(e.code), 10),
	})
	if err != nil {
		return st
	}
	return detailed
}

// CodeOf reads a business code from an Error or from gRPC status details.
func CodeOf(err error) (Code, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.code, true
	}
	st, ok := status.FromError(err)
	if !ok {
		return 0, false
	}
	for _, detail := range st.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.GetDomain() != domain {
			continue
		}
		n, convErr := strconv.ParseInt(info.GetReason(), 10, 32)
		if convErr != nil {
			return 0, false
		}
		return Code(n), true
	}
	return 0, false
}
