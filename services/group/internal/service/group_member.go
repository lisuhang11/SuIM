// Package service 提供群成员管理相关的业务逻辑实现。
package service

import (
	"context"
	"fmt"
	"time"

	apperrors "group/internal/errors"
	"group/internal/logger"
	"group/internal/types"
)

// InviteUserToGroup 邀请用户加入群组，opUserID 必须是群主或管理员。
func (s *groupService) InviteUserToGroup(ctx context.Context, in *types.InviteInput) error {
	logger.Info(ctx, "[group] invite members", "group", in.GroupID, "op", in.OpUserID, "count", len(in.UserIDs))
	if _, _, err := s.authMember(ctx, in.GroupID, in.OpUserID, types.GroupMemberRoleAdmin); err != nil {
		return err
	}

	exist, err := s.userVerifier.UsersExist(ctx, in.UserIDs)
	if err != nil {
		return apperrors.NewInternalError("verify users failed").WithDetails(err)
	}
	now := time.Now()
	for _, uid := range in.UserIDs {
		if !exist[uid] {
			return apperrors.NewUserNotExistError().WithDetails(fmt.Errorf("user %s not found", uid))
		}
		// 幂等处理：已是成员则跳过。
		if ok, _ := s.repo.MemberExists(ctx, in.GroupID, uid); ok {
			continue
		}
		m := &types.GroupMember{
			GroupID:        in.GroupID,
			UserID:         uid,
			RoleLevel:      types.GroupMemberRoleNormal,
			JoinTime:       now,
			JoinSource:     0,
			InviterUserID:  in.OpUserID,
			OperatorUserID: in.OpUserID,
		}
		if err := s.repo.CreateMember(ctx, m); err != nil {
			return apperrors.NewInternalError("add member failed").WithDetails(err)
		}
	}
	return nil
}

// KickGroupMember 踢出群成员，opUserID 角色必须高于目标成员。
func (s *groupService) KickGroupMember(ctx context.Context, groupID, opUserID, targetUserID string) error {
	logger.Info(ctx, "[group] kick member", "group", groupID, "op", opUserID, "target", targetUserID)
	_, op, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleAdmin)
	if err != nil {
		return err
	}
	target, err := s.repo.GetMember(ctx, groupID, targetUserID)
	if err != nil {
		return apperrors.NewMemberNotFoundError().WithDetails(err)
	}
	// 不能踢出同级或更高级别的成员（群主/同级管理员）。
	if op.RoleLevel <= target.RoleLevel {
		return apperrors.NewCannotKickRoleError()
	}
	if err := s.repo.DeleteMember(ctx, groupID, targetUserID); err != nil {
		return apperrors.NewInternalError("kick member failed").WithDetails(err)
	}
	return nil
}

// QuitGroup 退出群组，群主需先转让群主后方可退出。
func (s *groupService) QuitGroup(ctx context.Context, groupID, userID string) error {
	logger.Info(ctx, "[group] quit group", "group", groupID, "user", userID)
	m, err := s.repo.GetMember(ctx, groupID, userID)
	if err != nil {
		return apperrors.NewMemberNotFoundError().WithDetails(err)
	}
	if m.RoleLevel == types.GroupMemberRoleOwner {
		return apperrors.NewCannotQuitAsOwnerError()
	}
	if err := s.repo.DeleteMember(ctx, groupID, userID); err != nil {
		return apperrors.NewInternalError("quit group failed").WithDetails(err)
	}
	return nil
}

// GetGroupMembers 分页获取群成员列表。
func (s *groupService) GetGroupMembers(ctx context.Context, groupID string, offset, limit int) ([]*types.GroupMember, int, error) {
	members, total, err := s.repo.ListMembers(ctx, groupID, offset, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("list members failed").WithDetails(err)
	}
	return members, int(total), nil
}

// GetJoinedGroups 分页获取用户已加入的群组列表。
func (s *groupService) GetJoinedGroups(ctx context.Context, userID string, offset, limit int) ([]*types.Group, int, error) {
	members, total, err := s.repo.ListGroupsOfUser(ctx, userID, offset, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("list joined groups failed").WithDetails(err)
	}
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.GroupID)
	}
	groups, err := s.repo.ListGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("load groups failed").WithDetails(err)
	}
	return groups, int(total), nil
}
