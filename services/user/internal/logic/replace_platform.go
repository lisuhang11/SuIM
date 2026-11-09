package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

// ReplacePlatformSession keeps the caller's token and revokes the other live
// tokens of that user on the same platform.
func (s *Server) ReplacePlatformSession(ctx context.Context, req *userpb.ReplacePlatformSessionReq) (*userpb.ReplacePlatformSessionResp, error) {
	claims, tok, err := s.loadUsable(ctx, req.GetToken(), model.TokenType)
	if err != nil {
		return nil, err
	}
	revoked, err := s.store.RevokeOtherSessions(ctx, claims.UserID, claims.PlatformID, tok.Hash)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("revoke other sessions")
	}
	return &userpb.ReplacePlatformSessionResp{}, nil
}
