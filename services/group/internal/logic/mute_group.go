package logic

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// MuteGroup 全员禁言。群主和管理员可以发言，已解散的群不能改状态。
func (s *Server) MuteGroup(ctx context.Context, req *grouppb.MuteGroupReq) (*grouppb.MuteGroupResp, error) {
	op, err := bindActor(ctx, req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	groupID, op, err := muteOperator(req.GetGroupId(), op)
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
