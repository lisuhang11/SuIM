package errorcode

import "google.golang.org/grpc/codes"

// Call signaling codes, 8000-8999.
const (
	CallInvalid   Code = 8001
	CallBusy      Code = 8002
	CallPeerBusy  Code = 8003
	CallNotFound  Code = 8004
	CallNotMember Code = 8005
	CallState     Code = 8006
	CallGroupBusy Code = 8007
	CallFull      Code = 8008
)

var (
	ErrCallInvalid   = Define(CallInvalid, codes.InvalidArgument, "invalid call signal")
	ErrCallBusy      = Define(CallBusy, codes.FailedPrecondition, "already in a call")
	ErrCallPeerBusy  = Define(CallPeerBusy, codes.FailedPrecondition, "peer is in a call")
	ErrCallNotFound  = Define(CallNotFound, codes.NotFound, "call not found")
	ErrCallNotMember = Define(CallNotMember, codes.PermissionDenied, "not in this call")
	ErrCallState     = Define(CallState, codes.FailedPrecondition, "call is in the wrong state")
	ErrCallGroupBusy = Define(CallGroupBusy, codes.FailedPrecondition, "group already has a call")
	ErrCallFull      = Define(CallFull, codes.FailedPrecondition, "too many people in the call")
)
