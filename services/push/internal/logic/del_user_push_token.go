package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	pushpb "github.com/lisuhang11/SuIM/proto/push"
)

// DelUserPushToken 在用户登出时清空一台设备的推送 token。
func (s *Server) DelUserPushToken(ctx context.Context, req *pushpb.DelUserPushTokenReq) (*pushpb.DelUserPushTokenResp, error) {
	if req.GetUserId() == "" || req.GetPlatformId() <= 0 {
		return nil, errorcode.ErrPushInvalidArgs.WithMsg("user_id and platform_id are required")
	}
	userID := req.GetUserId()
	if actor := rpcctx.UserID(ctx); actor != "" && actor != userID {
		return nil, errorcode.ErrPushInvalidArgs.WithMsg("cannot delete token for another user")
	}
	if err := s.tokens.DelFCMToken(ctx, userID, req.GetPlatformId()); err != nil {
		return nil, err
	}
	return &pushpb.DelUserPushTokenResp{}, nil
}
