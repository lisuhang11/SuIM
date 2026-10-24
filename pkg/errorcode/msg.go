package errorcode

import "google.golang.org/grpc/codes"

// Message codes, 6000-6999.
const (
	MsgNotFriend   Code = 6001
	MsgBlocked     Code = 6002
	MsgInvalidArgs Code = 6003
)

var (
	ErrMsgNotFriend   = Define(MsgNotFriend, codes.FailedPrecondition, "not friends")
	ErrMsgBlocked     = Define(MsgBlocked, codes.FailedPrecondition, "blocked")
	ErrMsgInvalidArgs = Define(MsgInvalidArgs, codes.InvalidArgument, "invalid message")
)
