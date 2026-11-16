package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	pushpb "github.com/lisuhang11/SuIM/proto/push"
)

// SetAppBadge 覆盖客户端上报的角标计数，负数按 0 处理。
func (s *Server) SetAppBadge(ctx context.Context, req *pushpb.SetAppBadgeReq) (*pushpb.SetAppBadgeResp, error) {
	if req.GetUserId() == "" {
		return nil, errorcode.ErrPushInvalidArgs.WithMsg("user_id is required")
	}
	userID := req.GetUserId()
	if actor := rpcctx.UserID(ctx); actor != "" && actor != userID {
		return nil, errorcode.ErrPushInvalidArgs.WithMsg("cannot set badge for another user")
	}
	count := req.GetCount()
	if count < 0 {
		count = 0
	}
	if err := s.tokens.SetBadge(ctx, userID, count); err != nil {
		return nil, err
	}
	return &pushpb.SetAppBadgeResp{}, nil
}
