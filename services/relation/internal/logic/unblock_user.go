package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

type afterRemoveBlackReq struct {
	webhook.CommonCallbackReq
	OwnerUserID string `json:"ownerUserID"`
	BlackUserID string `json:"blackUserID"`
}

// 解除调用者对对方的拉黑。对方不在黑名单里时拒绝。
func (s *Server) UnblockUser(ctx context.Context, req *relationpb.UnblockUserReq) (*relationpb.UnblockUserResp, error) {
	// 1. 校验双方 userID。调用方必须是拉黑者。
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
	s.dropBlacks(ctx, owner)
	// 3. 通知对方已取消拉黑，并调用删除后 webhook。
	s.notifyBlackDeleted(ctx, owner, block)
	s.webhookAfterRemoveBlack(ctx, owner, block)
	return &relationpb.UnblockUserResp{}, nil
}

func (s *Server) webhookAfterRemoveBlack(ctx context.Context, owner, block string) {
	if s.hooks.Client == nil {
		return
	}
	body := &afterRemoveBlackReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterRemoveBlack},
		OwnerUserID:       owner,
		BlackUserID:       block,
	}
	s.hooks.Client.AsyncPost(ctx, body.GetCallbackCommand(), body, &webhook.CommonCallbackResp{}, &s.hooks.AfterRemoveBlack)
}
