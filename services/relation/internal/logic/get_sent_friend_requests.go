package logic

import (
	"context"

	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

func (s *Server) GetSentFriendRequests(ctx context.Context, req *relationpb.GetSentFriendRequestsReq) (*relationpb.GetSentFriendRequestsResp, error) {
	// 1. 校验用户、分页和处理状态。用户取上下文里的本人。
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	rows, total, err := s.friendRequests(ctx, userID, req.GetOffset(), req.GetLimit(), req.GetHandleResults(), false)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetSentFriendRequestsResp{Requests: rows, Total: total}, nil
}
