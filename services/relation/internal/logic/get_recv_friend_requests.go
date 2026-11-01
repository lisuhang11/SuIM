package logic

import (
	"context"

	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

func (s *Server) GetRecvFriendRequests(ctx context.Context, req *relationpb.GetRecvFriendRequestsReq) (*relationpb.GetRecvFriendRequestsResp, error) {
	// 1. 校验用户、分页和处理状态。调用方必须是本人，操作者身份尚未从上下文接入。
	rows, total, err := s.friendRequests(ctx, req.GetUserId(), req.GetOffset(), req.GetLimit(), req.GetHandleResults(), true)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetRecvFriendRequestsResp{Requests: rows, Total: total}, nil
}
