package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) LoginPhone(ctx context.Context, req *userpb.LoginPhoneReq) (*userpb.LoginResp, error) {
	if req.GetPlatformId() <= 0 {
		return nil, errorcode.ErrUserInvalidLogin.WithMsg("platform_id is required")
	}
	area, national, err := NormalizePhone(req.GetPhone())
	if err != nil {
		return nil, err
	}
	if s.pepper == "" {
		return nil, errorcode.ErrInternal.WithMsg("sms is not configured")
	}
	receipt, err := s.newID()
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create receipt")
	}
	if _, err := s.store.ConsumeSMSCode(ctx,
		smsCodeKey(SceneLogin, phoneKey(area, national)),
		codeHash(s.pepper, SceneLogin, area, national, strings.TrimSpace(req.GetCode())),
		receipt, s.policy.attempts(), s.policy.retryTTL(),
	); err != nil {
		if errors.Is(err, store.ErrSMSCode) {
			return nil, errorcode.ErrUserSmsCodeInvalid
		}
		return nil, errorcode.ErrInternal.WithMsg("consume sms code")
	}
	user, err := s.store.GetUserByPhone(ctx, area, national)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserBadCredential
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	if !user.IsActive {
		return nil, errorcode.ErrUserDisabled
	}
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
