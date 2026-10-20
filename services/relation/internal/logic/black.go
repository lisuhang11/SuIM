package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
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

func (s *Server) BlockUser(ctx context.Context, req *relationpb.BlockUserReq) (*relationpb.BlockUserResp, error) {
	// 1. 校验双方 userID。调用方必须是拉黑者，操作者身份尚未从上下文接入。
	owner, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	block, err := normalizeUserID(req.GetBlockedUserId())
	if err != nil {
		return nil, err
	}
	// 2. 拉黑前 webhook。回调拒绝则中断。
	if err := s.webhookBeforeAddBlack(ctx, owner, block); err != nil {
		return nil, err
	}
	// 3. 确认双方用户存在。
	if err := s.users.CheckUsers(ctx, []string{owner, block}); err != nil {
		return nil, err
	}
	// 4. 写入黑名单。操作者暂记为拉黑者本人。
	if err := s.store.CreateBlack(ctx, owner, block, owner, s.now()); err != nil {
		if errors.Is(err, store.ErrAlreadyBlocked) {
			return nil, errorcode.ErrRelationAlreadyBlocked
		}
		return nil, errorcode.ErrInternal.WithMsg("add black")
	}
	// 5. 通知对方已被拉黑。尚未接入。
	return &relationpb.BlockUserResp{}, nil
}

func (s *Server) UnblockUser(ctx context.Context, req *relationpb.UnblockUserReq) (*relationpb.UnblockUserResp, error) {
	// 1. 校验双方 userID。调用方必须是拉黑者，操作者身份尚未从上下文接入。
	owner, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	block, err := normalizeUserID(req.GetBlockedUserId())
	if err != nil {
		return nil, err
	}
	// 2. 删除黑名单记录。记录不存在也视为成功。
	if err := s.store.DeleteBlack(ctx, owner, block, s.now()); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("remove black")
	}
	// 3. 通知黑名单已删除。尚未接入。
	// 4. 删除后 webhook。
	s.webhookAfterRemoveBlack(ctx, owner, block)
	return &relationpb.UnblockUserResp{}, nil
}

func (s *Server) IsBlack(ctx context.Context, req *relationpb.IsBlackReq) (*relationpb.IsBlackResp, error) {
	// 1. 校验双方 userID。调用方必须是其中一人，操作者身份尚未从上下文接入。
	user1, err := normalizeUserID(req.GetUser1())
	if err != nil {
		return nil, err
	}
	user2, err := normalizeUserID(req.GetUser2())
	if err != nil {
		return nil, err
	}
	// 2. 查询双方黑名单。
	in1, in2, err := s.store.CheckBlack(ctx, user1, user2)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check black")
	}
	return &relationpb.IsBlackResp{InUser1Blacklist: in1, InUser2Blacklist: in2}, nil
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
