// Package service 实现群组业务逻辑，包括群组生命周期、群成员管理和禁言设置。
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	apperrors "group/internal/errors"
	"group/internal/logger"
	"group/internal/repository"
	"group/internal/types"
	"group/internal/types/interfaces"
)

// 哨兵错误从仓库层别名导出，使 errors.Is 可跨层使用，避免重复定义消息字符串。
var (
	ErrGroupNotFound  = repository.ErrGroupNotFound
	ErrMemberNotFound = repository.ErrMemberNotFound
	ErrRequestNotFound = repository.ErrRequestNotFound
)

// groupService 实现 GroupService 接口，封装群组业务逻辑。
type groupService struct {
	repo         interfaces.GroupRepository
	userVerifier interfaces.UserVerifier
}

// NewGroupService 创建群组服务实例。
func NewGroupService(repo interfaces.GroupRepository, userVerifier interfaces.UserVerifier) interfaces.GroupService {
	return &groupService{repo: repo, userVerifier: userVerifier}
}

// authMember 加载群组和调用者的成员信息，强制调用者至少满足 minRole 角色要求。
// 返回群组和成员记录。
func (s *groupService) authMember(ctx context.Context, groupID, userID string, minRole int) (*types.Group, *types.GroupMember, error) {
	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return nil, nil, apperrors.NewGroupNotFoundError().WithDetails(err)
	}
	m, err := s.repo.GetMember(ctx, groupID, userID)
	if err != nil {
		return nil, nil, apperrors.NewMemberNotFoundError().WithDetails(err)
	}
	if m.RoleLevel < minRole {
		return nil, nil, apperrors.NewNotAuthorizedError()
	}
	return g, m, nil
}

// CreateGroup 创建群组，创建者成为群主，可选邀请初始成员。
func (s *groupService) CreateGroup(ctx context.Context, in *types.CreateGroupInput) (string, *types.Group, error) {
	logger.Info(ctx, "[group] create group", "creator", in.CreatorUserID, "name", in.GroupName)

	if in.CreatorUserID == "" || in.GroupName == "" {
		return "", nil, apperrors.NewValidationError("creator_user_id and group_name are required")
	}
	ok, err := s.userVerifier.UserExists(ctx, in.CreatorUserID)
	if err != nil {
		return "", nil, apperrors.NewInternalError("verify user failed").WithDetails(err)
	}
	if !ok {
		return "", nil, apperrors.NewUserNotExistError()
	}

	now := time.Now()
	groupID := uuid.New().String()
	g := &types.Group{
		GroupID:                groupID,
		GroupName:              in.GroupName,
		Notification:           in.Notification,
		Introduction:           in.Introduction,
		FaceURL:                in.FaceURL,
		CreateTime:             now,
		Ex:                     in.Ex,
		Status:                 0,
		CreatorUserID:          in.CreatorUserID,
		GroupType:              in.GroupType,
		NeedVerification:       in.NeedVerification,
		LookMemberInfo:         in.LookMemberInfo,
		ApplyMemberFriend:      in.ApplyMemberFriend,
		NotificationUpdateTime: now,
		NotificationUserID:     in.CreatorUserID,
	}
	if err := s.repo.CreateGroup(ctx, g); err != nil {
		return "", nil, apperrors.NewInternalError("create group failed").WithDetails(err)
	}

	// 创建者成为群主。
	owner := &types.GroupMember{
		GroupID:        groupID,
		UserID:         in.CreatorUserID,
		RoleLevel:      types.GroupMemberRoleOwner,
		JoinTime:       now,
		JoinSource:     0,
		InviterUserID:  "",
		OperatorUserID: in.CreatorUserID,
	}
	if err := s.repo.CreateMember(ctx, owner); err != nil {
		return "", nil, apperrors.NewInternalError("add owner failed").WithDetails(err)
	}

	// 邀请初始成员（批量校验用户存在性）。
	if len(in.MemberIDs) > 0 {
		exist, err := s.userVerifier.UsersExist(ctx, in.MemberIDs)
		if err != nil {
			return "", nil, apperrors.NewInternalError("verify users failed").WithDetails(err)
		}
		for _, uid := range in.MemberIDs {
			if uid == in.CreatorUserID {
				continue
			}
			if !exist[uid] {
				return "", nil, apperrors.NewUserNotExistError().WithDetails(fmt.Errorf("user %s not found", uid))
			}
			m := &types.GroupMember{
				GroupID:        groupID,
				UserID:         uid,
				RoleLevel:      types.GroupMemberRoleNormal,
				JoinTime:       now,
				JoinSource:     0,
				InviterUserID:  in.CreatorUserID,
				OperatorUserID: in.CreatorUserID,
			}
			if err := s.repo.CreateMember(ctx, m); err != nil {
				return "", nil, apperrors.NewInternalError("add member failed").WithDetails(err)
			}
		}
	}

	logger.Info(ctx, "[group] group created", "group_id", groupID)
	return groupID, g, nil
}

// DismissGroup 硬删除群组及其所有成员和请求记录。opUserID 必须是群主。
func (s *groupService) DismissGroup(ctx context.Context, groupID, opUserID string) error {
	logger.Info(ctx, "[group] dismiss group", "group", groupID, "op", opUserID)
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleOwner); err != nil {
		return err
	}
	if err := s.repo.DeleteMembersByGroup(ctx, groupID); err != nil {
		return apperrors.NewInternalError("delete members failed").WithDetails(err)
	}
	if err := s.repo.DeleteRequestsByGroup(ctx, groupID); err != nil {
		return apperrors.NewInternalError("delete requests failed").WithDetails(err)
	}
	if err := s.repo.DeleteGroup(ctx, groupID); err != nil {
		return apperrors.NewInternalError("delete group failed").WithDetails(err)
	}
	logger.Info(ctx, "[group] group dismissed", "group_id", groupID)
	return nil
}

// TransferGroupOwner 转让群主，原群主降级为管理员。
func (s *groupService) TransferGroupOwner(ctx context.Context, groupID, opUserID, newOwnerUserID string) error {
	logger.Info(ctx, "[group] transfer owner", "group", groupID, "op", opUserID, "new", newOwnerUserID)
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleOwner); err != nil {
		return err
	}
	if newOwnerUserID == opUserID {
		return apperrors.NewValidationError("cannot transfer ownership to yourself")
	}
	newM, err := s.repo.GetMember(ctx, groupID, newOwnerUserID)
	if err != nil {
		return apperrors.NewMemberNotFoundError().WithDetails(err)
	}

	oldM, err := s.repo.GetMember(ctx, groupID, opUserID)
	if err != nil {
		return apperrors.NewMemberNotFoundError().WithDetails(err)
	}
	oldM.RoleLevel = types.GroupMemberRoleAdmin
	newM.RoleLevel = types.GroupMemberRoleOwner
	if err := s.repo.UpdateMember(ctx, oldM); err != nil {
		return apperrors.NewInternalError("update old owner failed").WithDetails(err)
	}
	if err := s.repo.UpdateMember(ctx, newM); err != nil {
		return apperrors.NewInternalError("update new owner failed").WithDetails(err)
	}
	logger.Info(ctx, "[group] owner transferred", "group_id", groupID, "new_owner", newOwnerUserID)
	return nil
}

// UpdateGroupInfo 更新群组可修改字段。opUserID 必须是群主或管理员。
func (s *groupService) UpdateGroupInfo(ctx context.Context, in *types.UpdateGroupInfoInput) (*types.Group, error) {
	logger.Info(ctx, "[group] update group info", "group", in.GroupID, "op", in.OpUserID)
	g, _, err := s.authMember(ctx, in.GroupID, in.OpUserID, types.GroupMemberRoleAdmin)
	if err != nil {
		return nil, err
	}

	// 仅更新非空/非 nil 的字段。
	if in.GroupName != "" {
		g.GroupName = in.GroupName
	}
	if in.FaceURL != "" {
		g.FaceURL = in.FaceURL
	}
	if in.Introduction != "" {
		g.Introduction = in.Introduction
	}
	if in.Notification != "" {
		g.Notification = in.Notification
		g.NotificationUpdateTime = time.Now()
		g.NotificationUserID = in.OpUserID
	}
	if in.NeedVerification != nil {
		g.NeedVerification = *in.NeedVerification
	}
	if in.LookMemberInfo != nil {
		g.LookMemberInfo = *in.LookMemberInfo
	}
	if in.ApplyMemberFriend != nil {
		g.ApplyMemberFriend = *in.ApplyMemberFriend
	}
	if in.Ex != "" {
		g.Ex = in.Ex
	}

	if err := s.repo.UpdateGroup(ctx, g); err != nil {
		return nil, apperrors.NewInternalError("update group failed").WithDetails(err)
	}
	return g, nil
}

// GetGroup 根据 ID 获取群组信息。
func (s *groupService) GetGroup(ctx context.Context, groupID string) (*types.Group, error) {
	g, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return nil, apperrors.NewGroupNotFoundError().WithDetails(err)
	}
	return g, nil
}

// SetGroupMute 设置群全员禁言开关。opUserID 必须是群主或管理员。
func (s *groupService) SetGroupMute(ctx context.Context, groupID, opUserID string, muted bool) error {
	logger.Info(ctx, "[group] set group mute", "group", groupID, "op", opUserID, "muted", muted)
	g, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleAdmin)
	if err != nil {
		return err
	}
	g.SetMuted(muted)
	if err := s.repo.UpdateGroup(ctx, g); err != nil {
		return apperrors.NewInternalError("set group mute failed").WithDetails(err)
	}
	return nil
}

// SetMemberMute 设置单个成员的禁言到期时间（0 表示取消禁言）。opUserID 必须是群主或管理员。
func (s *groupService) SetMemberMute(ctx context.Context, groupID, opUserID, targetUserID string, muteEndTime int64) error {
	logger.Info(ctx, "[group] set member mute", "group", groupID, "op", opUserID, "target", targetUserID, "until", muteEndTime)
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleAdmin); err != nil {
		return err
	}
	target, err := s.repo.GetMember(ctx, groupID, targetUserID)
	if err != nil {
		return apperrors.NewMemberNotFoundError().WithDetails(err)
	}
	var t *time.Time
	if muteEndTime > 0 {
		v := time.UnixMilli(muteEndTime)
		t = &v
	}
	target.MuteEndTime = t
	if err := s.repo.UpdateMember(ctx, target); err != nil {
		return apperrors.NewInternalError("set member mute failed").WithDetails(err)
	}
	return nil
}
