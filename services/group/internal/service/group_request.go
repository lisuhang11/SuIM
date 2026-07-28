// Package service 提供入群申请管理相关的业务逻辑实现。
package service

import (
	"context"
	"time"

	apperrors "group/internal/errors"
	"group/internal/logger"
	"group/internal/types"
)

// ApplyToJoinGroup 提交入群申请；当群组不需要验证时自动批准并直接加入。
func (s *groupService) ApplyToJoinGroup(ctx context.Context, in *types.ApplyInput) error {
	logger.Info(ctx, "[group] apply to join", "group", in.GroupID, "user", in.UserID)
	ok, err := s.userVerifier.UserExists(ctx, in.UserID)
	if err != nil {
		return apperrors.NewInternalError("verify user failed").WithDetails(err)
	}
	if !ok {
		return apperrors.NewUserNotExistError()
	}
	g, err := s.repo.GetGroup(ctx, in.GroupID)
	if err != nil {
		return apperrors.NewGroupNotFoundError().WithDetails(err)
	}
	if ok, _ := s.repo.MemberExists(ctx, in.GroupID, in.UserID); ok {
		return apperrors.NewAlreadyMemberError()
	}

	now := time.Now()
	existing, getErr := s.repo.GetRequest(ctx, in.GroupID, in.UserID)
	if getErr == nil {
		// 已存在该 (group, user) 的记录。
		if existing.HandleResult == 0 {
			return apperrors.NewAlreadyRequestedError()
		}
		// 之前被拒绝：重置为新的待处理请求（重新申请）。
		existing.HandleResult = 0
		existing.ReqMsg = in.ReqMsg
		existing.ReqTime = now
		existing.JoinSource = in.JoinSource
		existing.InviterUserID = in.InviterUserID
		existing.HandleUserID = ""
		existing.HandledMsg = ""
		existing.HandledTime = nil
		if err := s.repo.UpdateRequest(ctx, existing); err != nil {
			return apperrors.NewInternalError("reset request failed").WithDetails(err)
		}
		return nil
	}

	req := &types.GroupRequest{
		UserID:        in.UserID,
		GroupID:       in.GroupID,
		HandleResult:  0,
		ReqMsg:        in.ReqMsg,
		ReqTime:       now,
		JoinSource:    in.JoinSource,
		InviterUserID: in.InviterUserID,
	}

	// 无需验证：自动批准并立即添加成员。
	if g.NeedVerification == 0 {
		req.HandleResult = 1
		req.HandleUserID = in.InviterUserID
		t := now
		req.HandledTime = &t
		if err := s.repo.CreateRequest(ctx, req); err != nil {
			return apperrors.NewInternalError("create request failed").WithDetails(err)
		}
		m := &types.GroupMember{
			GroupID:        in.GroupID,
			UserID:         in.UserID,
			RoleLevel:      types.GroupMemberRoleNormal,
			JoinTime:       now,
			JoinSource:     in.JoinSource,
			InviterUserID:  in.InviterUserID,
			OperatorUserID: in.InviterUserID,
		}
		if err := s.repo.CreateMember(ctx, m); err != nil {
			return apperrors.NewInternalError("add member failed").WithDetails(err)
		}
		return nil
	}

	if err := s.repo.CreateRequest(ctx, req); err != nil {
		return apperrors.NewInternalError("create request failed").WithDetails(err)
	}
	return nil
}

// GetPendingApplications 获取群组待处理的入群申请（群主/管理员视角）。
func (s *groupService) GetPendingApplications(ctx context.Context, groupID, opUserID string, offset, limit int) ([]*types.GroupRequest, int, error) {
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleAdmin); err != nil {
		return nil, 0, err
	}
	reqs, total, err := s.repo.ListPendingByGroup(ctx, groupID, offset, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("list requests failed").WithDetails(err)
	}
	return reqs, int(total), nil
}

// GetUserApplications 获取用户的入群申请记录（申请人视角）。
func (s *groupService) GetUserApplications(ctx context.Context, userID string, offset, limit int) ([]*types.GroupRequest, int, error) {
	reqs, total, err := s.repo.ListByUser(ctx, userID, offset, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("list requests failed").WithDetails(err)
	}
	return reqs, int(total), nil
}

// HandleApplication 处理入群申请（1 同意/-1 拒绝），同意时将用户添加为普通成员。
func (s *groupService) HandleApplication(ctx context.Context, in *types.HandleInput) error {
	logger.Info(ctx, "[group] handle application", "group", in.GroupID, "user", in.UserID, "op", in.OpUserID, "result", in.HandleResult)
	if _, _, err := s.authMember(ctx, in.GroupID, in.OpUserID, types.GroupMemberRoleAdmin); err != nil {
		return err
	}
	req, err := s.repo.GetRequest(ctx, in.GroupID, in.UserID)
	if err != nil {
		return apperrors.NewRequestNotFoundError().WithDetails(err)
	}
	if req.HandleResult != 0 {
		return apperrors.NewRequestAlreadyHandledError()
	}
	if in.HandleResult != 1 && in.HandleResult != -1 {
		return apperrors.NewValidationError("handle_result must be 1 (agree) or -1 (reject)")
	}

	now := time.Now()
	if in.HandleResult == 1 {
		// 同意：若非成员则添加为普通成员。
		if ok, _ := s.repo.MemberExists(ctx, in.GroupID, in.UserID); !ok {
			m := &types.GroupMember{
				GroupID:        in.GroupID,
				UserID:         in.UserID,
				RoleLevel:      types.GroupMemberRoleNormal,
				JoinTime:       now,
				JoinSource:     req.JoinSource,
				InviterUserID:  req.InviterUserID,
				OperatorUserID: in.OpUserID,
			}
			if err := s.repo.CreateMember(ctx, m); err != nil {
				return apperrors.NewInternalError("add member failed").WithDetails(err)
			}
		}
	}
	req.HandleResult = in.HandleResult
	req.HandledMsg = in.HandledMsg
	req.HandleUserID = in.OpUserID
	t := now
	req.HandledTime = &t
	if err := s.repo.UpdateRequest(ctx, req); err != nil {
		return apperrors.NewInternalError("update request failed").WithDetails(err)
	}
	return nil
}

// GetUnhandledApplicationCount 统计群组待处理的入群申请数量。
func (s *groupService) GetUnhandledApplicationCount(ctx context.Context, groupID string) (int, error) {
	c, err := s.repo.CountPendingByGroup(ctx, groupID)
	if err != nil {
		return 0, apperrors.NewInternalError("count requests failed").WithDetails(err)
	}
	return int(c), nil
}
