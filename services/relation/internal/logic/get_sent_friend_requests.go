package logic

import (
	"context"

	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

func (s *Server) GetSentFriendRequests(ctx context.Context, req *relationpb.GetSentFriendRequestsReq) (*relationpb.GetSentFriendRequestsResp, error) {
	// 1. 校验用户、分页和处理状态。
	rows, total, err := s.friendRequests(ctx, req.GetUserId(), req.GetOffset(), req.GetLimit(), req.GetHandleResults(), false)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetSentFriendRequestsResp{Requests: rows, Total: total}, nil
}
