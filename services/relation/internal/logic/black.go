package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/webhook"
)

const (
	callbackBeforeAddBlack   = "callbackBeforeAddBlackCommand"
	callbackAfterRemoveBlack = "callbackAfterRemoveBlackCommand"
)

type beforeAddBlackReq struct {
	webhook.CommonCallbackReq
	OwnerUserID string `json:"ownerUserID"`
	BlackUserID string `json:"blackUserID"`
}

type afterRemoveBlackReq struct {
	webhook.CommonCallbackReq
	OwnerUserID string `json:"ownerUserID"`
	BlackUserID string `json:"blackUserID"`
}

func (s *Server) webhookBeforeAddBlack(ctx context.Context, owner, block string) error {
	return webhook.WithCondition(ctx, &s.hooks.BeforeAddBlack, func(ctx context.Context) error {
		if s.hooks.Client == nil {
			return errorcode.ErrInternal.WithMsg("webhook client is nil")
		}
		body := &beforeAddBlackReq{
			CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: callbackBeforeAddBlack},
			OwnerUserID:       owner,
			BlackUserID:       block,
		}
		return s.hooks.Client.SyncPost(ctx, body.GetCallbackCommand(), body, &webhook.CommonCallbackResp{}, &s.hooks.BeforeAddBlack)
	})
}

func (s *Server) webhookAfterRemoveBlack(ctx context.Context, owner, block string) {
	if s.hooks.Client == nil {
		return
	}
	body := &afterRemoveBlackReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: callbackAfterRemoveBlack},
		OwnerUserID:       owner,
		BlackUserID:       block,
	}
	s.hooks.Client.AsyncPost(ctx, body.GetCallbackCommand(), body, &webhook.CommonCallbackResp{}, &s.hooks.AfterRemoveBlack)
}
