package logic

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) ChangePassword(ctx context.Context, req *userpb.ChangePasswordReq) (*userpb.ChangePasswordResp, error) {
	// 1. 校验用户 ID 和新密码。
	id, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err := validatePassword(req.GetNewPassword()); err != nil {
		return nil, err
	}
	// 2. 查用户。停用账号不能改密。
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	if !user.IsActive {
		return nil, errorcode.ErrUserDisabled
	}
	// 3. 校验旧密码。
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.GetOldPassword())) != nil {
		return nil, errorcode.ErrUserPasswordMismatch
	}
	// 4. 写入新密码，并作废全部会话。
	//    kafka: 通知 msggateway 断开该用户全部平台的在线连接。尚未接入。
	hash, err := bcrypt.GenerateFromPassword([]byte(req.GetNewPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("hash password")
	}
	revoked, err := s.store.ChangePassword(ctx, id, string(hash))
	s.invalidate(ctx, revoked)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("change password")
	}
	return &userpb.ChangePasswordResp{}, nil
}
