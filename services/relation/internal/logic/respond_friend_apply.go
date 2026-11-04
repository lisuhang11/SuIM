package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

func (s *Server) RespondFriendApply(ctx context.Context, req *relationpb.RespondFriendApplyReq) (*relationpb.RespondFriendApplyResp, error) {
	// 1. 校验双方 userID。调用方必须是被申请人，操作者身份尚未从上下文接入。
	from, err := normalizeUserID(req.GetFromUserId())
	if err != nil {
		return nil, err
	}
	to, err := actorID(ctx, req.GetToUserId())
	if err != nil {
		return nil, err
	}
	handleMsg, err := normalizeMessage(req.GetHandleMsg())
	if err != nil {
		return nil, err
	}
	switch req.GetHandleResult() {
	case model.HandleAgree:
		// 2. 任一方拉黑则拒绝。申请保持待处理，不发同意回调。
		if err := s.rejectIfBlocked(ctx, from, to); err != nil {
			return nil, err
		}
		// 3. 同意前 webhook。回调拒绝则中断。
		if err := s.beforeAddFriendAgree(ctx, from, to, handleMsg, model.HandleAgree); err != nil {
			return nil, err
		}
		// 4. 把申请标为同意，并写入双方好友关系。
		if err := s.store.AgreeFriendRequest(ctx, from, to, handleMsg, s.now()); err != nil {
			return nil, friendRequestErr(err)
		}
		s.dropFriends(ctx, from, to)
		// 5. 通知申请人已同意，并调用同意后 webhook。
		s.notifyFriendAgreed(ctx, from, to, handleMsg)
		s.afterAddFriendAgree(ctx, from, to, handleMsg, model.HandleAgree)
		return &relationpb.RespondFriendApplyResp{}, nil
	case model.HandleRefuse:
		// 6. 把申请标为拒绝。拒绝不查黑名单。
		if err := s.store.RefuseFriendRequest(ctx, from, to, handleMsg, s.now()); err != nil {
			return nil, friendRequestErr(err)
		}
		// 7. 通知申请人申请被拒绝。
		s.notifyFriendRefused(ctx, from, to, handleMsg)
		return &relationpb.RespondFriendApplyResp{}, nil
	default:
		return nil, errorcode.ErrRelationInvalidHandle
	}
}

func friendRequestErr(err error) error {
	switch {
	case errors.Is(err, store.ErrRequestNotFound):
		return errorcode.ErrRelationRequestNotFound
	case errors.Is(err, store.ErrRequestHandled):
		return errorcode.ErrRelationRequestHandled
	default:
		return errorcode.ErrInternal.WithMsg("update friend request")
	}
}
