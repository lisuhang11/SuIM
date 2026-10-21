package errorcode

import "google.golang.org/grpc/codes"

// Group service codes, 4000-4999.
const (
	GroupInvalidArgs     Code = 4001
	GroupInvalidName     Code = 4002
	GroupTypeUnsupported Code = 4003
	GroupInvalidUserID   Code = 4004
	GroupUserNotFound    Code = 4005
	GroupMemberRepeated  Code = 4006
	GroupNotFound        Code = 4007
	GroupDismissed       Code = 4008
	GroupNoPermission    Code = 4009
	GroupAlreadyMember   Code = 4010
	GroupNotMember       Code = 4011
	GroupOwnerKick       Code = 4012
	GroupOwnerQuit       Code = 4013
	GroupKickSelf        Code = 4014
	GroupSameOwner       Code = 4015
	GroupNothingToUpdate Code = 4016
	GroupRequestRepeated Code = 4017
	GroupIDGenFailed     Code = 4018
)

var (
	ErrGroupInvalidArgs     = Define(GroupInvalidArgs, codes.InvalidArgument, "invalid group argument")
	ErrGroupInvalidName     = Define(GroupInvalidName, codes.InvalidArgument, "group name is required")
	ErrGroupTypeUnsupported = Define(GroupTypeUnsupported, codes.InvalidArgument, "group type must be 1 or 2")
	ErrGroupInvalidUserID   = Define(GroupInvalidUserID, codes.InvalidArgument, "user_id is invalid")
	ErrGroupUserNotFound    = Define(GroupUserNotFound, codes.NotFound, "user not found")
	ErrGroupMemberRepeated  = Define(GroupMemberRepeated, codes.InvalidArgument, "group member repeated")
	ErrGroupNotFound        = Define(GroupNotFound, codes.NotFound, "group not found")
	ErrGroupDismissed       = Define(GroupDismissed, codes.FailedPrecondition, "group dismissed")
	ErrGroupNoPermission    = Define(GroupNoPermission, codes.PermissionDenied, "no permission")
	ErrGroupAlreadyMember   = Define(GroupAlreadyMember, codes.AlreadyExists, "already in group")
	ErrGroupNotMember       = Define(GroupNotMember, codes.NotFound, "not a group member")
	ErrGroupOwnerKick       = Define(GroupOwnerKick, codes.InvalidArgument, "cannot kick the group owner")
	ErrGroupOwnerQuit       = Define(GroupOwnerQuit, codes.PermissionDenied, "group owner cannot quit")
	ErrGroupKickSelf        = Define(GroupKickSelf, codes.InvalidArgument, "cannot kick yourself")
	ErrGroupSameOwner       = Define(GroupSameOwner, codes.InvalidArgument, "new owner is the current owner")
	ErrGroupNothingToUpdate = Define(GroupNothingToUpdate, codes.InvalidArgument, "no group field to update")
	ErrGroupRequestRepeated = Define(GroupRequestRepeated, codes.AlreadyExists, "group request already pending")
	ErrGroupIDGenFailed     = Define(GroupIDGenFailed, codes.Internal, "group id generation failed")
)
