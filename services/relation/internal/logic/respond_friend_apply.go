package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

func (s *Server) RespondFriendApply(ctx context.Context, req *relationpb.RespondFriendApplyReq) (*relationpb.RespondFriendApplyResp, error) {
	// 1. 校验双方 userID。调用方必须是被申请人，操作者身份尚未从上下文接入。
	from, err := normalizeUserID(req.GetFromUserId())
	if err != nil {
		return nil, err
	}
	to, err := normalizeUserID(req.GetToUserId())
	if err != nil {
		return nil, err
	}
	handleMsg, err := normalizeMessage(req.GetHandleMsg())
	if err != nil {
		return nil, err
	}
	switch req.GetHandleResult() {
	case model.HandleAgree:
		// 2. 同意前 webhook。尚未接入。
		// 3. 把申请标为同意，并写入双方好友关系。
		if err := s.store.AgreeFriendRequest(ctx, from, to, handleMsg, s.now()); err != nil {
			return nil, friendRequestErr(err)
		}
		// 4. 同意后 webhook，并通知申请人。尚未接入。
		return &relationpb.RespondFriendApplyResp{}, nil
	case model.HandleRefuse:
		// 5. 把申请标为拒绝。
		if err := s.store.RefuseFriendRequest(ctx, from, to, handleMsg, s.now()); err != nil {
			return nil, friendRequestErr(err)
		}
		// 6. 通知申请人申请被拒绝。尚未接入。
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
