package logic

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) Login(ctx context.Context, req *userpb.LoginReq) (*userpb.LoginResp, error) {
	// 1. 校验邮箱、密码和平台号。
	email := strings.ToLower(strings.TrimSpace(req.GetEmail()))
	if email == "" || req.GetPassword() == "" || req.GetPlatformId() <= 0 {
		return nil, errorcode.ErrUserInvalidLogin
	}
	// 2. 按邮箱查用户。不存在时与密码错误返回同一错误。
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserBadCredential
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	// 3. 校验密码。
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.GetPassword())) != nil {
		return nil, errorcode.ErrUserBadCredential
	}
	// 4. 停用账号拒绝登录。
	if !user.IsActive {
		return nil, errorcode.ErrUserDisabled
	}
	// 5. 签发 token，并作废该平台上一次会话。
	tok, err := s.issue(user.UserID, req.GetPlatformId())
	if err != nil {
		return nil, err
	}
	revoked, err := s.store.StartSession(ctx, tok)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create session")
	}
	return &userpb.LoginResp{
		User:              toPB(user),
		Token:             tok.Raw,
		ExpireTimeSeconds: s.issuer.ExpireSeconds(),
	}, nil
}
