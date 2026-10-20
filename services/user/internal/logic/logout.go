package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

func (s *Server) Logout(ctx context.Context, req *userpb.LogoutReq) (*userpb.LogoutResp, error) {
	claims, err := s.issuer.ParseIgnoreExpiry(req.GetToken())
	if err != nil || claims.Type != model.TokenType {
		return nil, errorcode.ErrUserInvalidToken
	}
	revoked, err := s.store.RevokeSession(ctx, claims.UserID, claims.PlatformID)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("revoke session")
	}
	return &userpb.LogoutResp{}, nil
}
