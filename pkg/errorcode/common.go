package errorcode

import "google.golang.org/grpc/codes"

const (
	// Internal means the service failed while doing its own work.
	Internal Code = 1001
)

var (
	ErrInternal = Define(Internal, codes.Internal, "internal error")
)
