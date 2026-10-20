package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

func (s *Server) ValidateToken(ctx context.Context, req *userpb.ValidateTokenReq) (*userpb.ValidateTokenResp, error) {
	// 1. 解析 token，并核对该会话仍有效。
	claims, _, err := s.loadUsable(ctx, req.GetToken(), model.TokenType)
	if err != nil {
		return nil, err
	}
	// 2. 查用户，停用账号视为无效。
	user, err := s.store.GetUserByID(ctx, claims.UserID)
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

func (s *Server) loadUsable(ctx context.Context, raw, wantType string) (*token.Claims, *model.Token, error) {
	// 1. 验签，并核对令牌类型。
	claims, err := s.issuer.Parse(raw)
	if err != nil || claims.Type != wantType {
		return nil, nil, errorcode.ErrUserInvalidToken
	}
	// 2. 先查本地缓存，未命中再查 Redis 和 MySQL。
	hash := token.Hash(raw)
	var tok *model.Token
	if s.tokens != nil {
		tok, err = s.tokens.Get(ctx, hash, func(ctx context.Context) (*model.Token, error) {
			return s.store.Lookup(ctx, hash)
		})
	} else {
		tok, err = s.store.Lookup(ctx, hash)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, errorcode.ErrUserInvalidToken
	}
	if err != nil {
		return nil, nil, errorcode.ErrInternal.WithMsg("lookup token")
	}
	// 3. 拒绝已撤销、身份不一致或已过期的令牌。
	if tok.IsRevoked || tok.UserID != claims.UserID || tok.PlatformID != claims.PlatformID || tok.Type != wantType || !tok.ExpiresAt.After(s.now()) {
		return nil, nil, errorcode.ErrUserInvalidToken
	}
	return claims, tok, nil
}
