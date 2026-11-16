package errorcode

import "google.golang.org/grpc/codes"

const (
	PushInvalidArgs Code = 7004
)

var (
	ErrPushInvalidArgs = Define(PushInvalidArgs, codes.InvalidArgument, "invalid push argument")
)
