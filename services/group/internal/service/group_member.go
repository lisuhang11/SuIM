// Package service 提供群成员管理相关的业务逻辑实现。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	apperrors "group/internal/errors"
	"group/internal/logger"
	"group/internal/types"
	"group/internal/types/interfaces"
)

// InviteUserToGroup 邀请用户加入群组，opUserID 必须是群主或管理员。
func (s *groupService) InviteUserToGroup(ctx context.Context, in *types.InviteInput) error {
	logger.Info(ctx, "[group] invite members", "group", in.GroupID, "op", in.OpUserID, "count", len(in.UserIDs))
	if len(in.UserIDs) == 0 {
		return apperrors.NewValidationError("user_ids is required")
	}
	exist, err := s.userVerifier.UsersExist(ctx, in.UserIDs)
	if err != nil {
		return apperrors.NewInternalError("verify users failed").WithDetails(err)
	}
	seen := make(map[string]struct{}, len(in.UserIDs))
	for _, uid := range in.UserIDs {
		if uid == "" || !exist[uid] {
			return apperrors.NewUserNotExistError().WithDetails(fmt.Errorf("user %s not found", uid))
		}
		if _, duplicate := seen[uid]; duplicate {
			return apperrors.NewValidationError("user_ids contains duplicates")
		}
		seen[uid] = struct{}{}
	}
	now := time.Now()
	var joined, recipients []string
	err = s.repo.WithinTransaction(ctx, func(repo interfaces.GroupRepository) error {
		if _, _, err := authMemberWithRepo(ctx, repo, in.GroupID, in.OpUserID, types.GroupMemberRoleAdmin); err != nil {
			return err
		}
		for _, uid := range in.UserIDs {
			alreadyMember, err := repo.MemberExists(ctx, in.GroupID, uid)
			if err != nil {
				return apperrors.NewInternalError("check member failed").WithDetails(err)
			}
			if alreadyMember {
				continue
			}
			m := &types.GroupMember{GroupID: in.GroupID, UserID: uid, RoleLevel: types.GroupMemberRoleNormal, JoinTime: now, JoinSource: 0, InviterUserID: in.OpUserID, OperatorUserID: in.OpUserID}
			if err := repo.CreateMember(ctx, m); err != nil {
				return apperrors.NewInternalError("add member failed").WithDetails(err)
			}
			joined = append(joined, uid)
		}
		var err error
		recipients, err = memberIDs(ctx, repo, in.GroupID)
		if err != nil {
			return err
		}
		if len(joined) > 0 {
			if err := bumpJoinVersions(ctx, repo, joined, in.GroupID, types.VersionStateInsert); err != nil {
				return apperrors.NewInternalError("bump join version failed").WithDetails(err)
			}
			if err := bumpMemberVersion(ctx, repo, in.GroupID, joined, types.VersionStateInsert); err != nil {
				return apperrors.NewInternalError("bump member version failed").WithDetails(err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(joined) > 0 {
		s.invalidateGroupCache(ctx, in.GroupID)
		s.publish(ctx, interfaces.GroupEvent{Type: "group.members_joined", GroupID: in.GroupID, OperatorUserID: in.OpUserID, SubjectUserIDs: joined, RecipientUserIDs: recipients})
	}
	return nil
}

// KickGroupMember 踢出群成员，opUserID 角色必须高于目标成员。
func (s *groupService) KickGroupMember(ctx context.Context, groupID, opUserID, targetUserID string) error {
	logger.Info(ctx, "[group] kick member", "group", groupID, "op", opUserID, "target", targetUserID)
	var recipients []string
	err := s.repo.WithinTransaction(ctx, func(repo interfaces.GroupRepository) error {
		_, op, err := authMemberWithRepo(ctx, repo, groupID, opUserID, types.GroupMemberRoleAdmin)
		if err != nil {
			return err
		}
		target, err := repo.GetMember(ctx, groupID, targetUserID)
		if err != nil {
			return apperrors.NewMemberNotFoundError().WithDetails(err)
		}
		if op.RoleLevel <= target.RoleLevel {
			return apperrors.NewCannotKickRoleError()
		}
		if err := repo.DeleteMember(ctx, groupID, targetUserID); err != nil {
			return apperrors.NewInternalError("kick member failed").WithDetails(err)
		}
		if err := bumpJoinVersions(ctx, repo, []string{targetUserID}, groupID, types.VersionStateDelete); err != nil {
			return apperrors.NewInternalError("bump join version failed").WithDetails(err)
		}
		if err := bumpMemberVersion(ctx, repo, groupID, []string{targetUserID}, types.VersionStateDelete); err != nil {
			return apperrors.NewInternalError("bump member version failed").WithDetails(err)
		}
		recipients, err = memberIDs(ctx, repo, groupID)
		return err
	})
	if err != nil {
		return err
	}
	s.invalidateGroupCache(ctx, groupID)
	s.publish(ctx, interfaces.GroupEvent{Type: "group.member_kicked", GroupID: groupID, OperatorUserID: opUserID, SubjectUserIDs: []string{targetUserID}, RecipientUserIDs: recipients})
	return nil
}

// QuitGroup 退出群组，群主需先转让群主后方可退出。
func (s *groupService) QuitGroup(ctx context.Context, groupID, userID string) error {
	logger.Info(ctx, "[group] quit group", "group", groupID, "user", userID)
	var recipients []string
	err := s.repo.WithinTransaction(ctx, func(repo interfaces.GroupRepository) error {
		m, err := repo.GetMember(ctx, groupID, userID)
		if err != nil {
			return apperrors.NewMemberNotFoundError().WithDetails(err)
		}
		if m.RoleLevel == types.GroupMemberRoleOwner {
			return apperrors.NewCannotQuitAsOwnerError()
		}
		if err := repo.DeleteMember(ctx, groupID, userID); err != nil {
			return apperrors.NewInternalError("quit group failed").WithDetails(err)
		}
		if err := bumpJoinVersions(ctx, repo, []string{userID}, groupID, types.VersionStateDelete); err != nil {
			return apperrors.NewInternalError("bump join version failed").WithDetails(err)
		}
		if err := bumpMemberVersion(ctx, repo, groupID, []string{userID}, types.VersionStateDelete); err != nil {
			return apperrors.NewInternalError("bump member version failed").WithDetails(err)
		}
		recipients, err = memberIDs(ctx, repo, groupID)
		return err
	})
	if err != nil {
		return err
	}
	s.invalidateGroupCache(ctx, groupID)
	s.publish(ctx, interfaces.GroupEvent{Type: "group.member_quit", GroupID: groupID, OperatorUserID: userID, SubjectUserIDs: []string{userID}, RecipientUserIDs: recipients})
	return nil
}

// GetGroupMembers 分页获取群成员列表。

func (s *groupService) GetGroupMembers(ctx context.Context, groupID, opUserID string, offset, limit int) ([]*types.GroupMember, int, error) {
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleNormal); err != nil {
		return nil, 0, err
	}
	members, total, err := s.repo.ListMembers(ctx, groupID, offset, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("list members failed").WithDetails(err)
	}
	return members, int(total), nil
}

// GetGroupMemberUserIDs 对齐 OpenIM getGroupMemberUserIDs。
func (s *groupService) GetGroupMemberUserIDs(ctx context.Context, groupID string) ([]string, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, apperrors.NewValidationError("group_id is required")
	}
	if _, err := s.repo.GetGroup(ctx, groupID); err != nil {
		return nil, apperrors.NewGroupNotFoundError().WithDetails(err)
	}
	ids, err := s.repo.ListMemberUserIDs(ctx, groupID)
	if err != nil {
		return nil, apperrors.NewInternalError("list member user ids failed").WithDetails(err)
	}
	return ids, nil
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
	if err := s.fillMemberCounts(ctx, groups); err != nil {
		return nil, 0, err
	}
	return groups, int(total), nil
}
