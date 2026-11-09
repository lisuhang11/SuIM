package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

// 分别返回两人的好友列表里是否有对方。调用者必须是其中一人。
func (s *Server) IsFriend(ctx context.Context, req *relationpb.IsFriendReq) (*relationpb.IsFriendResp, error) {
	// 1. 校验双方 userID。上下文里的用户必须是其中一人。
	user1, err := normalizeUserID(req.GetUser1())
	if err != nil {
		return nil, err
	}
	user2, err := normalizeUserID(req.GetUser2())
	if err != nil {
		return nil, err
	}
	if err := rpcctx.OneOf(ctx, user1, user2); err != nil {
		return nil, err
	}
	// 2. 查询双方好友列表。有缓存时走本地和 Redis，没有则直接查表。
	in1, in2, err := s.friendship(ctx, user1, user2)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check friendship")
	}
	return &relationpb.IsFriendResp{InUser1Friends: in1, InUser2Friends: in2}, nil
}
