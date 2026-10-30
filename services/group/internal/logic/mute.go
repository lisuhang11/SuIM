package logic

import (
	"context"
	"errors"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// MuteGroupMember 禁言一名成员。群主不能被禁言，管理员只有群主能禁言。
func (s *Server) MuteGroupMember(ctx context.Context, req *grouppb.MuteGroupMemberReq) (*grouppb.MuteGroupMemberResp, error) {
	groupID, op, userID, err := muteUsers(req.GetGroupId(), req.GetOpUserId(), req.GetUserId())
	if err != nil {
		return nil, err
	}
	if req.GetMutedSeconds() == 0 {
		return nil, errorcode.ErrGroupInvalidArgs.WithMsg("muted seconds must be positive")
	}
	if _, err := s.activeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	if err := s.authorizeMute(ctx, groupID, op, userID); err != nil {
		return nil, err
	}
	now := s.now()
	end := now.Add(time.Duration(req.GetMutedSeconds()) * time.Second)
	if err := s.store.SetMemberMute(ctx, groupID, userID, &end, now); err != nil {
		return nil, mapStore(err)
	}
	s.notifyMemberMuted(ctx, op, groupID, userID, req.GetMutedSeconds())
	return &grouppb.MuteGroupMemberResp{}, nil
}

// CancelMuteGroupMember 解除一名成员的禁言。权限和禁言相同。
func (s *Server) CancelMuteGroupMember(ctx context.Context, req *grouppb.CancelMuteGroupMemberReq) (*grouppb.CancelMuteGroupMemberResp, error) {
	groupID, op, userID, err := muteUsers(req.GetGroupId(), req.GetOpUserId(), req.GetUserId())
	if err != nil {
		return nil, err
	}
	if _, err := s.activeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	if err := s.authorizeMute(ctx, groupID, op, userID); err != nil {
		return nil, err
	}
	if err := s.store.SetMemberMute(ctx, groupID, userID, nil, s.now()); err != nil {
		return nil, mapStore(err)
	}
	s.notifyMemberCancelMuted(ctx, op, groupID, userID)
	return &grouppb.CancelMuteGroupMemberResp{}, nil
}

// MuteGroup 全员禁言。群主和管理员可以发言，已解散的群不能改状态。
func (s *Server) MuteGroup(ctx context.Context, req *grouppb.MuteGroupReq) (*grouppb.MuteGroupResp, error) {
	groupID, op, err := muteOperator(req.GetGroupId(), req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	if _, _, err := s.requireManager(ctx, groupID, op); err != nil {
		return nil, err
	}
	if err := s.store.SetGroupMuted(ctx, groupID, true, s.now()); err != nil {
		return nil, mapStore(err)
	}
	s.notifyGroupMuted(ctx, op, groupID, false)
	return &grouppb.MuteGroupResp{}, nil
}

// CancelMuteGroup 解除全员禁言。已解散的群保持解散，不会被改回正常。
func (s *Server) CancelMuteGroup(ctx context.Context, req *grouppb.CancelMuteGroupReq) (*grouppb.CancelMuteGroupResp, error) {
	groupID, op, err := muteOperator(req.GetGroupId(), req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	if _, _, err := s.requireManager(ctx, groupID, op); err != nil {
		return nil, err
	}
	if err := s.store.SetGroupMuted(ctx, groupID, false, s.now()); err != nil {
		return nil, mapStore(err)
	}
	s.notifyGroupMuted(ctx, op, groupID, true)
	return &grouppb.CancelMuteGroupResp{}, nil
}

func muteOperator(groupID, opUserID string) (string, string, error) {
	groupID, err := normalizeGroupID(groupID)
	if err != nil {
		return "", "", err
	}
	op, err := normalizeUserID(opUserID)
	if err != nil {
		return "", "", err
	}
	return groupID, op, nil
}

func muteUsers(groupID, opUserID, userID string) (string, string, string, error) {
	groupID, op, err := muteOperator(groupID, opUserID)
	if err != nil {
		return "", "", "", err
	}
	userID, err = normalizeUserID(userID)
	if err != nil {
		return "", "", "", err
	}
	return groupID, op, userID, nil
}

func (s *Server) authorizeMute(ctx context.Context, groupID, op, userID string) error {
	opMember, err := s.memberOf(ctx, groupID, op)
	if err != nil {
		if errors.Is(err, errorcode.ErrGroupNotMember) {
			return errorcode.ErrGroupNoPermission
		}
		return err
	}
	target, err := s.memberOf(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !canMuteMember(opMember, target) {
		return errorcode.ErrGroupNoPermission
	}
	return nil
}

func canMuteMember(op, target *model.Member) bool {
	switch target.RoleLevel {
	case model.RoleOwner:
		return false
	case model.RoleAdmin:
		return op.RoleLevel == model.RoleOwner
	default:
		return op.RoleLevel == model.RoleOwner || op.RoleLevel == model.RoleAdmin
	}
}
