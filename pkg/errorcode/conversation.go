package errorcode

import "google.golang.org/grpc/codes"

// Conversation service codes, 7000-7999.
const (
	ConversationInvalidArgs   Code = 7001
	ConversationInvalidUserID Code = 7002
)

var (
	ErrConversationInvalidArgs   = Define(ConversationInvalidArgs, codes.InvalidArgument, "invalid conversation argument")
	ErrConversationInvalidUserID = Define(ConversationInvalidUserID, codes.InvalidArgument, "user_id is invalid")
)
