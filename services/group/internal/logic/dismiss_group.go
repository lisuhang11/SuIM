package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/grouppb"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// DismissGroup 群主解散群。成员保留，群状态改为已解散。
func (s *Server) DismissGroup(ctx context.Context, req *grouppb.DismissGroupReq) (*grouppb.DismissGroupResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	op, err := normalizeUserID(req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	group, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		return nil, mapStore(err)
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
	// 解散通知和 webhook 尚未接入。
	return &grouppb.DismissGroupResp{}, nil
}
