package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// TransferGroupOwner 群主把群转给一名成员，自己降为该成员原来的角色。
func (s *Server) TransferGroupOwner(ctx context.Context, req *grouppb.TransferGroupOwnerReq) (*grouppb.TransferGroupOwnerResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := normalizeUserID(req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	next, err := normalizeUserID(req.GetNewOwnerUserId())
	if err != nil {
		return nil, err
	}
	if op == next {
		return nil, errorcode.ErrGroupSameOwner
	}
	group, err := s.activeGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group.OwnerUserID != op {
		return nil, errorcode.ErrGroupNoPermission
	}
	members, err := s.store.FindMembers(ctx, groupID, []string{op, next})
	if err != nil {
		return nil, err
	}
	var oldOwner, newOwner *model.Member
	for _, member := range members {
		switch member.UserID {
		case op:
			oldOwner = member
		case next:
			newOwner = member
		}
	}
	if oldOwner == nil || oldOwner.RoleLevel != model.RoleOwner || newOwner == nil {
		return nil, errorcode.ErrGroupNotMember
	}
	// 新群主若处于禁言，转让时解除。原群主降为新群主原来的角色。
	if err := s.store.TransferOwner(ctx, groupID, op, next, s.now()); err != nil {
		return nil, mapStore(err)
	}
	// webhook 尚未接入。转让会连跳多个成员版本，通知里带上新版本，客户端对不上就拉增量。
	if latest, err := s.store.TakeGroup(ctx, groupID); err == nil {
		group = latest
	}
	if refreshed, err := s.store.FindMembers(ctx, groupID, []string{op, next}); err == nil {
		for _, member := range refreshed {
			switch member.UserID {
			case op:
				oldOwner = member
			case next:
				newOwner = member
			}
		}
	}
	s.notifyTransferred(ctx, op, group, oldOwner, newOwner)
	return &grouppb.TransferGroupOwnerResp{}, nil
}
