package logic

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// CancelMuteGroup 解除全员禁言。已解散的群保持解散，不会被改回正常。
func (s *Server) CancelMuteGroup(ctx context.Context, req *grouppb.CancelMuteGroupReq) (*grouppb.CancelMuteGroupResp, error) {
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
	if err := s.store.SetGroupMuted(ctx, groupID, false, s.now()); err != nil {
		return nil, mapStore(err)
	}
	s.notifyGroupMuted(ctx, op, groupID, true)
	return &grouppb.CancelMuteGroupResp{}, nil
}
