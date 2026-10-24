package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// KickGroupMember 把成员移出群。群主可踢管理员和普通成员，管理员只能踢普通成员。
func (s *Server) KickGroupMember(ctx context.Context, req *grouppb.KickGroupMemberReq) (*grouppb.KickGroupMemberResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := normalizeUserID(req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	target, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if op == target {
		return nil, errorcode.ErrGroupKickSelf
	}
	if _, err := s.activeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	members, err := s.store.FindMembers(ctx, groupID, []string{op, target})
	if err != nil {
		return nil, err
	}
	var opMember, targetMember *model.Member
	for _, member := range members {
		switch member.UserID {
		case op:
			opMember = member
		case target:
			targetMember = member
		}
	}
	if opMember == nil {
		return nil, errorcode.ErrGroupNoPermission
	}
	if targetMember == nil {
		return nil, errorcode.ErrGroupNotMember
	}
	if targetMember.RoleLevel == model.RoleOwner {
		return nil, errorcode.ErrGroupOwnerKick
	}
	switch opMember.RoleLevel {
	case model.RoleOwner:
	case model.RoleAdmin:
		if targetMember.RoleLevel != model.RoleOrdinary {
			return nil, errorcode.ErrGroupNoPermission
		}
	default:
		return nil, errorcode.ErrGroupNoPermission
	}
	if err := s.store.DeleteMember(ctx, groupID, target, s.now()); err != nil {
		return nil, mapStore(err)
	}
	// 踢人通知、会话 seq 和 webhook 尚未接入。
	return &grouppb.KickGroupMemberResp{}, nil
}
