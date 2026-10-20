package logic

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) Register(ctx context.Context, req *userpb.RegisterReq) (*userpb.RegisterResp, error) {
	// 1. 校验邮箱、昵称和密码。
	email, nickname, err := normalizeAccount(req.GetEmail(), req.GetUsername())
	if err != nil {
		return nil, err
	}
	if err := validatePassword(req.GetPassword()); err != nil {
		return nil, err
	}
	// 2. 生成密码哈希和用户 ID。
	hash, err := bcrypt.GenerateFromPassword([]byte(req.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("hash password")
	}
	id, err := s.newID()
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create user id")
	}
	// 3. 写入用户。邮箱重复返回已注册。
	user := &model.User{
		UserID:       id,
		Email:        email,
		PasswordHash: string(hash),
		Nickname:     nickname,
		IsActive:     true,
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			return nil, errorcode.ErrUserEmailTaken
		}
		return nil, errorcode.ErrInternal.WithMsg("create user")
	}
	return &userpb.RegisterResp{User: toPB(user)}, nil
}

func normalizeAccount(email, username string) (string, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > 255 {
		return "", "", errorcode.ErrUserInvalidUsername
	}
	if !validEmail(email) {
		return "", "", errorcode.ErrUserInvalidEmail
	}
	return email, username, nil
}

func validEmail(email string) bool {
	if len(email) > 255 || strings.ContainsAny(email, " \t") {
		return false
	}
	at := strings.IndexByte(email, '@')
	if at <= 0 || at != strings.LastIndexByte(email, '@') || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	return strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

func validatePassword(password string) error {
	n := len(password)
	if n < 6 || n > 72 {
		return errorcode.ErrUserInvalidPassword
	}
	return nil
}
