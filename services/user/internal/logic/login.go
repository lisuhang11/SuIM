package logic

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/prommetrics"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

// 用账号和密码登录并签发令牌。手机号不走这里。
func (s *Server) Login(ctx context.Context, req *userpb.LoginReq) (*userpb.LoginResp, error) {
	account := strings.TrimSpace(req.GetAccount())
	if account == "" {
		account = strings.TrimSpace(req.GetEmail())
	}
	if strings.TrimSpace(req.GetPhone()) != "" || account == "" || req.GetPassword() == "" || req.GetPlatformId() <= 0 {
		return nil, errorcode.ErrUserInvalidLogin
	}
	user, err := s.store.GetUserByAccount(ctx, account)
	if errors.Is(err, store.ErrNotFound) && strings.Contains(account, "@") {
		user, err = s.store.GetUserByEmail(ctx, strings.ToLower(account))
	}
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserBadCredential
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.GetPassword())) != nil {
		return nil, errorcode.ErrUserBadCredential
	}
	if !user.IsActive {
		return nil, errorcode.ErrUserDisabled
	}
	pair, err := s.issuePair(user.UserID, req.GetPlatformId(), "", "")
	if err != nil {
		return nil, err
	}
	revoked, err := s.store.StartSession(ctx, pair.Access, pair.Refresh)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create session")
	}
	s.recordLoginIP(ctx, user.UserID, req.GetClientIp())
	prommetrics.UserLoginCounter.Inc()
	return &userpb.LoginResp{
		User:              toPB(user),
		Token:             pair.Access.Raw,
		RefreshToken:      pair.Refresh.Raw,
		ExpireTimeSeconds: s.issuer.ExpireSeconds(),
	}, nil
}
