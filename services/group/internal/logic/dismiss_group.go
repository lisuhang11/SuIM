package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// 群主解散群。 delete_member 为空时只改状态并发送 1511，成员行留着，方便把解散提示推给现有成员。 delete_member 为 true 时删掉全部成员并记入入群版本，不再发提示，也不冻历史。
func (s *Server) DismissGroup(ctx context.Context, req *grouppb.DismissGroupReq) (*grouppb.DismissGroupResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := bindActor(ctx, req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	group, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		return nil, mapStore(err)
	}
	if req.GetDeleteMember() {
		return s.deleteDismissedMembers(ctx, op, group)
	}
	if group.Status == model.GroupStatusDismissed {
		return nil, errorcode.ErrGroupDismissed
	}
	// 只有群主能解散。成员保留，状态改为已解散。
	if group.OwnerUserID != op {
		return nil, errorcode.ErrGroupNoPermission
	}
	if err := s.store.DismissGroup(ctx, groupID, s.now()); err != nil {
		return nil, mapStore(err)
	}
	// 解散通知让在线成员刷新已加入的群。成员行留到推送之后再删。
	opMember, _ := s.store.TakeMember(ctx, groupID, op)
	if latest, err := s.store.TakeGroup(ctx, groupID); err == nil {
		group = latest
	}
	s.notifyDismissed(ctx, op, group, opMember)
	members, _ := s.store.MemberUserIDs(ctx, groupID)
	s.afterDismiss(ctx, group, members)
	return &grouppb.DismissGroupResp{}, nil
}

func (s *Server) deleteDismissedMembers(ctx context.Context, op string, group *model.Group) (*grouppb.DismissGroupResp, error) {
	if group.Status != model.GroupStatusDismissed {
		if group.OwnerUserID == "" || group.OwnerUserID != op {
			return nil, errorcode.ErrGroupNoPermission
		}
		if err := s.store.DismissGroup(ctx, group.GroupID, s.now()); err != nil {
			return nil, mapStore(err)
		}
		group.Status = model.GroupStatusDismissed
	} else if group.OwnerUserID != "" && group.OwnerUserID != op {
		return nil, errorcode.ErrGroupNoPermission
	}
	deleted, err := s.store.DeleteGroupMembers(ctx, group.GroupID, s.now())
	if err != nil {
		return nil, mapStore(err)
	}
	if len(deleted) > 0 {
		if latest, err := s.store.TakeGroup(ctx, group.GroupID); err == nil {
			group = latest
		}
		s.afterDismiss(ctx, group, deleted)
	}
	return &grouppb.DismissGroupResp{}, nil
}
