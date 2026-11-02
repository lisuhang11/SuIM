package errorcode

import "google.golang.org/grpc/codes"

// Message codes, 6000-6999.
const (
	MsgNotFriend    Code = 6001
	MsgBlocked      Code = 6002
	MsgInvalidArgs  Code = 6003
	MsgNotInGroup   Code = 6004
	MsgMutedInGroup Code = 6005
	MsgMutedGroup   Code = 6006
	MsgDismissed    Code = 6007
)

var (
	ErrMsgNotFriend    = Define(MsgNotFriend, codes.FailedPrecondition, "not friends")
	ErrMsgBlocked      = Define(MsgBlocked, codes.FailedPrecondition, "blocked")
	ErrMsgInvalidArgs  = Define(MsgInvalidArgs, codes.InvalidArgument, "invalid message")
	ErrMsgNotInGroup   = Define(MsgNotInGroup, codes.FailedPrecondition, "not in group")
	ErrMsgMutedInGroup = Define(MsgMutedInGroup, codes.FailedPrecondition, "muted in group")
	ErrMsgMutedGroup   = Define(MsgMutedGroup, codes.FailedPrecondition, "group is muted")
	ErrMsgDismissed    = Define(MsgDismissed, codes.FailedPrecondition, "group dismissed")
)
