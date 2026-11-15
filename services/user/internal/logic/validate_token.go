package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

// 确认令牌签名正确且会话仍有效。账号已停用或不存在时视为无效。
func (s *Server) ValidateToken(ctx context.Context, req *userpb.ValidateTokenReq) (*userpb.ValidateTokenResp, error) {
	claims, _, err := s.loadUsableAccess(ctx, req.GetToken())
	if err != nil {
		return nil, err
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
	return &userpb.ValidateTokenResp{Valid: true, User: toPB(user), PlatformId: claims.PlatformID}, nil
}

func (s *Server) loadUsableAccess(ctx context.Context, raw string) (*token.Claims, *model.Token, error) {
	return s.loadUsable(ctx, raw, true)
}

func (s *Server) loadUsableRefresh(ctx context.Context, raw string) (*token.Claims, *model.Token, error) {
	return s.loadUsable(ctx, raw, false)
}

func (s *Server) loadUsable(ctx context.Context, raw string, access bool) (*token.Claims, *model.Token, error) {
	claims, err := s.issuer.Parse(raw)
	if err != nil {
		return nil, nil, errorcode.ErrUserInvalidToken
	}
	if access {
		if !model.IsAccess(claims.Type) {
			return nil, nil, errorcode.ErrUserInvalidToken
		}
	} else if claims.Type != model.TokenTypeRefresh {
		return nil, nil, errorcode.ErrUserInvalidToken
	}
	jti := claims.ID
	var tok *model.Token
	if s.tokens != nil {
		tok, err = s.tokens.Get(ctx, jti, func(ctx context.Context) (*model.Token, error) {
			return s.store.Lookup(ctx, jti)
		})
	} else {
		tok, err = s.store.Lookup(ctx, jti)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, errorcode.ErrUserInvalidToken
	}
	if err != nil {
		return nil, nil, errorcode.ErrInternal.WithMsg("lookup token")
	}
	if tok.IsRevoked || tok.UserID != claims.UserID || tok.PlatformID != claims.PlatformID || tok.Type != claims.Type || !tok.ExpiresAt.After(s.now()) {
		return nil, nil, errorcode.ErrUserInvalidToken
	}
	return claims, tok, nil
}
