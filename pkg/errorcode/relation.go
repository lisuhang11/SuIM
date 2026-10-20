package errorcode

import "google.golang.org/grpc/codes"

// Relation service codes, 3000-3999.
const (
	RelationInvalidUserID     Code = 3001
	RelationCanNotAddYourself Code = 3002
	RelationAlreadyFriends    Code = 3003
	RelationUserNotFound      Code = 3004
	RelationRequestRepeated   Code = 3005
	RelationRequestNotFound   Code = 3006
	RelationRequestHandled    Code = 3007
	RelationInvalidHandle     Code = 3008
	RelationInvalidMessage    Code = 3009
	RelationAlreadyBlocked    Code = 3010
	RelationNotFriend         Code = 3011
	RelationInvalidRemark     Code = 3012
	RelationNothingToUpdate   Code = 3013
)

var (
	ErrRelationInvalidUserID     = Define(RelationInvalidUserID, codes.InvalidArgument, "user_id is invalid")
	ErrRelationCanNotAddYourself = Define(RelationCanNotAddYourself, codes.InvalidArgument, "cannot add yourself")
	ErrRelationAlreadyFriends    = Define(RelationAlreadyFriends, codes.AlreadyExists, "already friends")
	ErrRelationUserNotFound      = Define(RelationUserNotFound, codes.NotFound, "user not found")
	ErrRelationRequestRepeated   = Define(RelationRequestRepeated, codes.AlreadyExists, "friend request already pending")
	ErrRelationRequestNotFound   = Define(RelationRequestNotFound, codes.NotFound, "friend request not found")
	ErrRelationRequestHandled    = Define(RelationRequestHandled, codes.FailedPrecondition, "friend request already handled")
	ErrRelationInvalidHandle     = Define(RelationInvalidHandle, codes.InvalidArgument, "handle_result must be 1 or -1")
	ErrRelationInvalidMessage    = Define(RelationInvalidMessage, codes.InvalidArgument, "message is too long")
	ErrRelationAlreadyBlocked    = Define(RelationAlreadyBlocked, codes.AlreadyExists, "already blocked")
	ErrRelationNotFriend         = Define(RelationNotFriend, codes.NotFound, "not friends")
	ErrRelationInvalidRemark     = Define(RelationInvalidRemark, codes.InvalidArgument, "remark is too long")
	ErrRelationNothingToUpdate   = Define(RelationNothingToUpdate, codes.InvalidArgument, "remark or is_pinned is required")
)
