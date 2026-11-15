package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

// 作废这个用户在该平台上的会话。签名正确但已过期的令牌也可以退出。
func (s *Server) Logout(ctx context.Context, req *userpb.LogoutReq) (*userpb.LogoutResp, error) {
	claims, err := s.issuer.ParseIgnoreExpiry(req.GetToken())
	if err != nil || (!model.IsAccess(claims.Type) && claims.Type != model.TokenTypeRefresh) {
		return nil, errorcode.ErrUserInvalidToken
	}
	revoked, err := s.store.RevokeSession(ctx, claims.UserID, claims.PlatformID)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("revoke session")
	}
	return &userpb.LogoutResp{}, nil
}
