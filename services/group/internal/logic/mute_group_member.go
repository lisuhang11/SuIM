package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// MuteGroupMember 禁言一名成员。群主不能被禁言，管理员只有群主能禁言。
func (s *Server) MuteGroupMember(ctx context.Context, req *grouppb.MuteGroupMemberReq) (*grouppb.MuteGroupMemberResp, error) {
	op, err := bindActor(ctx, req.GetOpUserId())
	if err != nil {
		return nil, err
	}
	groupID, op, userID, err := muteUsers(req.GetGroupId(), op, req.GetUserId())
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
