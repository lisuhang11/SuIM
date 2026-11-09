package logic

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

// 用账号和密码登录并签发令牌。手机号不走这里。
func (s *Server) Login(ctx context.Context, req *userpb.LoginReq) (*userpb.LoginResp, error) {
	// 1. 只接受账户和密码。手机号不能走这条登录。
	account := strings.TrimSpace(req.GetAccount())
	if account == "" {
		account = strings.TrimSpace(req.GetEmail())
	}
	if strings.TrimSpace(req.GetPhone()) != "" || account == "" || req.GetPassword() == "" || req.GetPlatformId() <= 0 {
		return nil, errorcode.ErrUserInvalidLogin
	}
	// 2. 先按账户查，邮箱注册的旧账号再按邮箱查。不存在和密码错误返回同一结果。
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
	// 3. 校验密码。没有设置登录密码的账号，和密码错误返回同一结果。
	match := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.GetPassword())) == nil
	if !user.HasPassword || !match {
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
