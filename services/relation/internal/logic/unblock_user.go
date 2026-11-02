package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

func (s *Server) UnblockUser(ctx context.Context, req *relationpb.UnblockUserReq) (*relationpb.UnblockUserResp, error) {
	// 1. 校验双方 userID。调用方必须是拉黑者，操作者身份尚未从上下文接入。
	owner, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	block, err := normalizeUserID(req.GetBlockedUserId())
	if err != nil {
		return nil, err
	}
	// 2. 删除黑名单记录。记录不存在也视为成功。
	if err := s.store.DeleteBlack(ctx, owner, block); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("remove black")
	}
	// 3. 通知黑名单已删除。尚未接入。
	// 4. 删除后 webhook。
	s.webhookAfterRemoveBlack(ctx, owner, block)
	return &relationpb.UnblockUserResp{}, nil
}
