package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	pushpb "github.com/lisuhang11/SuIM/proto/push"
)

// FcmUpdateToken 保存或清空一台设备的 FCM token，空 token 表示删除。
func (s *Server) FcmUpdateToken(ctx context.Context, req *pushpb.FcmUpdateTokenReq) (*pushpb.FcmUpdateTokenResp, error) {
	if req.GetUserId() == "" || req.GetPlatformId() <= 0 {
		return nil, errorcode.ErrPushInvalidArgs.WithMsg("user_id and platform_id are required")
	}
	userID := req.GetUserId()
	if actor := rpcctx.UserID(ctx); actor != "" && actor != userID {
		return nil, errorcode.ErrPushInvalidArgs.WithMsg("cannot update token for another user")
	}
	if err := s.tokens.SetFCMToken(ctx, userID, req.GetPlatformId(), req.GetToken()); err != nil {
		return nil, err
	}
	return &pushpb.FcmUpdateTokenResp{}, nil
}
