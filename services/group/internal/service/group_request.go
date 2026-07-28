// Package service 提供入群申请管理相关的业务逻辑实现。
package service

import (
	"context"
	"errors"
	"time"

	apperrors "group/internal/errors"
	"group/internal/logger"
	"group/internal/types"
	"group/internal/types/interfaces"
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
	now := time.Now()
	var joined bool
	var recipients []string
	err = s.repo.WithinTransaction(ctx, func(repo interfaces.GroupRepository) error {
		g, err := repo.GetGroup(ctx, in.GroupID)
		if err != nil {
			return apperrors.NewGroupNotFoundError().WithDetails(err)
		}
		isMember, err := repo.MemberExists(ctx, in.GroupID, in.UserID)
		if err != nil {
			return apperrors.NewInternalError("check member failed").WithDetails(err)
		}
		if isMember {
			return apperrors.NewAlreadyMemberError()
		}

		req, getErr := repo.GetRequest(ctx, in.GroupID, in.UserID)
		isNew := errors.Is(getErr, ErrRequestNotFound)
		if getErr != nil && !isNew {
			return apperrors.NewInternalError("load request failed").WithDetails(getErr)
		}
		if getErr == nil && req.HandleResult == 0 {
			return apperrors.NewAlreadyRequestedError()
		}
		if isNew {
			req = &types.GroupRequest{UserID: in.UserID, GroupID: in.GroupID}
		}
		req.HandleResult = 0
		req.ReqMsg = in.ReqMsg
		req.ReqTime = now
		req.JoinSource = in.JoinSource
		req.InviterUserID = in.InviterUserID
		req.HandleUserID = ""
		req.HandledMsg = ""
		req.HandledTime = nil

		if g.NeedVerification == 0 {
			req.HandleResult = 1
			req.HandleUserID = in.UserID
			t := now
			req.HandledTime = &t
		}
		if isNew {
			if err := repo.CreateRequest(ctx, req); err != nil {
				return apperrors.NewInternalError("create request failed").WithDetails(err)
			}
		} else if err := repo.UpdateRequest(ctx, req); err != nil {
			return apperrors.NewInternalError("reset request failed").WithDetails(err)
		}
		if g.NeedVerification != 0 {
			return nil
		}
		m := &types.GroupMember{GroupID: in.GroupID, UserID: in.UserID, RoleLevel: types.GroupMemberRoleNormal, JoinTime: now, JoinSource: in.JoinSource, InviterUserID: in.InviterUserID, OperatorUserID: in.UserID}
		if err := repo.CreateMember(ctx, m); err != nil {
			return apperrors.NewInternalError("add member failed").WithDetails(err)
		}
		joined = true
		recipients, err = memberIDs(ctx, repo, in.GroupID)
		return err
	})
	if err != nil {
		return err
	}
	if joined {
		s.publish(ctx, interfaces.GroupEvent{Type: "group.members_joined", GroupID: in.GroupID, OperatorUserID: in.UserID, SubjectUserIDs: []string{in.UserID}, RecipientUserIDs: recipients})
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
	if in.HandleResult != 1 && in.HandleResult != -1 {
		return apperrors.NewValidationError("handle_result must be 1 (agree) or -1 (reject)")
	}

	now := time.Now()
	var recipients []string
	err := s.repo.WithinTransaction(ctx, func(repo interfaces.GroupRepository) error {
		if _, _, err := authMemberWithRepo(ctx, repo, in.GroupID, in.OpUserID, types.GroupMemberRoleAdmin); err != nil {
			return err
		}
		req, err := repo.GetRequest(ctx, in.GroupID, in.UserID)
		if err != nil {
			return apperrors.NewRequestNotFoundError().WithDetails(err)
		}
		if req.HandleResult != 0 {
			return apperrors.NewRequestAlreadyHandledError()
		}
		if in.HandleResult == 1 {
			isMember, err := repo.MemberExists(ctx, in.GroupID, in.UserID)
			if err != nil {
				return apperrors.NewInternalError("check member failed").WithDetails(err)
			}
			if !isMember {
				m := &types.GroupMember{GroupID: in.GroupID, UserID: in.UserID, RoleLevel: types.GroupMemberRoleNormal, JoinTime: now, JoinSource: req.JoinSource, InviterUserID: req.InviterUserID, OperatorUserID: in.OpUserID}
				if err := repo.CreateMember(ctx, m); err != nil {
					return apperrors.NewInternalError("add member failed").WithDetails(err)
				}
			}
		}
		req.HandleResult = in.HandleResult
		req.HandledMsg = in.HandledMsg
		req.HandleUserID = in.OpUserID
		t := now
		req.HandledTime = &t
		if err := repo.UpdateRequest(ctx, req); err != nil {
			return apperrors.NewInternalError("update request failed").WithDetails(err)
		}
		recipients, err = memberIDs(ctx, repo, in.GroupID)
		return err
	})
	if err != nil {
		return err
	}
	if in.HandleResult == 1 {
		s.publish(ctx, interfaces.GroupEvent{Type: "group.application_accepted", GroupID: in.GroupID, OperatorUserID: in.OpUserID, SubjectUserIDs: []string{in.UserID}, RecipientUserIDs: recipients})
	}
	return nil
}

// GetUnhandledApplicationCount 统计群组待处理的入群申请数量。
func (s *groupService) GetUnhandledApplicationCount(ctx context.Context, groupID, opUserID string) (int, error) {
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleAdmin); err != nil {
		return 0, err
	}
	c, err := s.repo.CountPendingByGroup(ctx, groupID)
	if err != nil {
		return 0, apperrors.NewInternalError("count requests failed").WithDetails(err)
	}
	return int(c), nil
}
