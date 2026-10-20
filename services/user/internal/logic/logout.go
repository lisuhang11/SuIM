package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

func (s *Server) Logout(ctx context.Context, req *userpb.LogoutReq) (*userpb.LogoutResp, error) {
	// 1. 解析 token。过期但签名有效也接受。
	claims, err := s.issuer.ParseIgnoreExpiry(req.GetToken())
	if err != nil || claims.Type != model.TokenType {
		return nil, errorcode.ErrUserInvalidToken
	}
	// 2. 作废该用户在该平台的会话。
	revoked, err := s.store.RevokeSession(ctx, claims.UserID, claims.PlatformID)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("revoke session")
	}
	return &userpb.LogoutResp{}, nil
}
