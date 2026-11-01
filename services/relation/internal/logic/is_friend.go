package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

func (s *Server) IsFriend(ctx context.Context, req *relationpb.IsFriendReq) (*relationpb.IsFriendResp, error) {
	// 1. 校验双方 userID。调用方必须是其中一人，操作者身份尚未从上下文接入。
	user1, err := normalizeUserID(req.GetUser1())
	if err != nil {
		return nil, err
	}
	user2, err := normalizeUserID(req.GetUser2())
	if err != nil {
		return nil, err
	}
	// 2. 查询双方好友列表。
	in1, in2, err := s.store.CheckIn(ctx, user1, user2)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check friendship")
	}
	return &relationpb.IsFriendResp{InUser1Friends: in1, InUser2Friends: in2}, nil
}
