package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

func (s *Server) IsBlack(ctx context.Context, req *relationpb.IsBlackReq) (*relationpb.IsBlackResp, error) {
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
	// 2. 查询双方黑名单。
	in1, in2, err := s.store.CheckBlack(ctx, user1, user2)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check black")
	}
	return &relationpb.IsBlackResp{InUser1Blacklist: in1, InUser2Blacklist: in2}, nil
}
