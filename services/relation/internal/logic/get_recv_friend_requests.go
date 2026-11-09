package logic

import (
	"context"

	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

// 分页返回别人发给调用者的好友申请。
func (s *Server) GetRecvFriendRequests(ctx context.Context, req *relationpb.GetRecvFriendRequestsReq) (*relationpb.GetRecvFriendRequestsResp, error) {
	// 1. 校验用户、分页和处理状态。调用方必须是本人。
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	rows, total, err := s.friendRequests(ctx, userID, req.GetOffset(), req.GetLimit(), req.GetHandleResults(), true)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetRecvFriendRequestsResp{Requests: rows, Total: total}, nil
}
