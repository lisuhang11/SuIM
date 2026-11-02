package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

func (s *Server) GetFriendList(ctx context.Context, req *relationpb.GetFriendListReq) (*relationpb.GetFriendListResp, error) {
	// 1. 校验用户和分页。调用方必须是本人，操作者身份尚未从上下文接入。
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	// 2. 按置顶、添加时间、用户 ID 的固定顺序分页。
	total, rows, err := s.store.PageFriends(ctx, userID, offset, limit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list friends")
	}
	friends, err := s.friendInfos(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetFriendListResp{Friends: friends, Total: int32(total)}, nil
}
