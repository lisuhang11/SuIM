package logic

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) Login(ctx context.Context, req *userpb.LoginReq) (*userpb.LoginResp, error) {
	// 1. 校验手机号或邮箱、密码和平台号。两者只能填一个。
	email := strings.ToLower(strings.TrimSpace(req.GetEmail()))
	phone := strings.TrimSpace(req.GetPhone())
	if req.GetPassword() == "" || req.GetPlatformId() <= 0 || (email == "" && phone == "") || (email != "" && phone != "") {
		return nil, errorcode.ErrUserInvalidLogin
	}
	// 2. 按手机号或邮箱查用户。不存在、格式不对时与密码错误返回同一错误。
	var (
		user *model.User
		err  error
	)
	if phone != "" {
		area, national, nerr := NormalizePhone(phone)
		if nerr != nil {
			return nil, errorcode.ErrUserBadCredential
		}
		user, err = s.store.GetUserByPhone(ctx, area, national)
	} else {
		user, err = s.store.GetUserByEmail(ctx, email)
	}
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
