package logic

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// 解除一名成员的禁言。权限和禁言相同。
func (s *Server) CancelMuteGroupMember(ctx context.Context, req *grouppb.CancelMuteGroupMemberReq) (*grouppb.CancelMuteGroupMemberResp, error) {
	op, err := bindActor(ctx, req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	groupID, op, userID, err := muteUsers(req.GetGroupId(), op, req.GetUserId())
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
