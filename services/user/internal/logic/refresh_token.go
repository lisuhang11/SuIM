package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

// RefreshToken rotates the refresh token and issues a new access+refresh pair in the same family.
// Replaying a revoked refresh invalidates the entire family.
func (s *Server) RefreshToken(ctx context.Context, req *userpb.RefreshTokenReq) (*userpb.LoginResp, error) {
	claims, err := s.issuer.Parse(req.GetRefreshToken())
	if err != nil || claims.Type != model.TokenTypeRefresh {
		return nil, errorcode.ErrUserInvalidToken
	}
	pair, err := s.issuePair(claims.UserID, claims.PlatformID, "", claims.ID)
	if err != nil {
		return nil, err
	}
	// Family id is filled from the stored refresh row inside RotateRefresh; temporary family on pair is overwritten.
	revoked, err := s.store.RotateRefresh(ctx, claims.ID, pair.Access, pair.Refresh)
	s.invalidate(ctx, revoked)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errorcode.ErrUserInvalidToken
	}
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("rotate refresh")
	}
	user, err := s.userByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserInvalidToken
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	if !user.IsActive {
		return nil, errorcode.ErrUserDisabled
	}
	return &userpb.LoginResp{
		User:              toPB(user),
		Token:             pair.Access.Raw,
		RefreshToken:      pair.Refresh.Raw,
		ExpireTimeSeconds: s.issuer.ExpireSeconds(),
	}, nil
}
