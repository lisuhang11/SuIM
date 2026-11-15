package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
)

// 保留这次带来的令牌族，作废该用户在同一平台上的其他有效令牌。
func (s *Server) ReplacePlatformSession(ctx context.Context, req *userpb.ReplacePlatformSessionReq) (*userpb.ReplacePlatformSessionResp, error) {
	claims, tok, err := s.loadUsableAccess(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}
	revoked, err := s.store.RevokeOtherSessions(ctx, claims.UserID, claims.PlatformID, tok.FamilyID)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("revoke other sessions")
	}
	return &userpb.ReplacePlatformSessionResp{}, nil
}
