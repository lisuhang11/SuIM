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

func (s *Server) RegisterPhone(ctx context.Context, req *userpb.RegisterPhoneReq) (*userpb.RegisterPhoneResp, error) {
	version := strings.TrimSpace(req.GetProtocolVersion())
	if !req.GetAgreeProtocol() || version == "" || len(version) > 32 {
		return nil, errorcode.ErrUserProtocolRequired
	}
	if strings.TrimSpace(req.GetScene()) != SceneRegister {
		return nil, errorcode.ErrUserInvalidScene
	}
	if req.GetPlatformId() <= 0 {
		return nil, errorcode.ErrUserInvalidLogin.WithMsg("platform_id is required")
	}
	area, national, err := NormalizePhone(req.GetPhone())
	if err != nil {
		return nil, err
	}
	password := req.GetPassword()
	if password != "" {
		if err := validatePassword(password); err != nil {
			return nil, err
		}
	}
	nickname, err := phoneNickname(req.GetNickname(), national)
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
	got, err := s.store.ConsumeSMSCode(ctx,
		smsCodeKey(SceneRegister, phoneKey(area, national)),
		codeHash(s.pepper, SceneRegister, area, national, strings.TrimSpace(req.GetCode())),
		receipt, s.policy.attempts(), s.policy.retryTTL(),
	)
	if err != nil {
		if errors.Is(err, store.ErrSMSCode) {
			return nil, errorcode.ErrUserSmsCodeInvalid
		}
		return nil, errorcode.ErrInternal.WithMsg("consume sms code")
	}
	existing, err := s.store.GetUserByPhone(ctx, area, national)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	if existing != nil {
		if existing.RegisterReceipt != "" && existing.RegisterReceipt == got {
			return s.finishPhoneRegister(ctx, existing, req.GetPlatformId())
		}
		return nil, errorcode.ErrUserPhoneTaken
	}
	hash, err := s.phonePasswordHash(password)
	if err != nil {
		return nil, err
	}
	id, err := s.newID()
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create user id")
	}
	ip := strings.TrimSpace(req.GetClientIp())
	if len(ip) > 64 {
		ip = ip[:64]
	}
	user := &model.User{
		UserID:           id,
		PasswordHash:     hash,
		Nickname:         nickname,
		IsActive:         true,
		AreaCode:         area,
		PhoneNumber:      national,
		RegisterType:     model.RegisterTypePhone,
		ProtocolVersion:  version,
		ProtocolAgreedAt: s.now().UTC(),
		RegisterIP:       ip,
		RegisterReceipt:  got,
	}
	if err := s.beforeRegister(ctx, user); err != nil {
		return nil, err
	}
	if err := s.store.CreatePhoneUser(ctx, user); err != nil {
		if errors.Is(err, store.ErrPhoneTaken) {
			raced, gerr := s.store.GetUserByPhone(ctx, area, national)
			if gerr != nil {
				return nil, errorcode.ErrInternal.WithMsg("query user")
			}
			if raced.RegisterReceipt != "" && raced.RegisterReceipt == got {
				return s.finishPhoneRegister(ctx, raced, req.GetPlatformId())
			}
			return nil, errorcode.ErrUserPhoneTaken
		}
		return nil, errorcode.ErrInternal.WithMsg("create user")
	}
	s.afterRegister(ctx, user)
	return s.finishPhoneRegister(ctx, user, req.GetPlatformId())
}

func (s *Server) phonePasswordHash(password string) (string, error) {
	if password == "" {
		secret, err := randomSecret()
		if err != nil {
			return "", errorcode.ErrInternal.WithMsg("create password")
		}
		password = secret
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errorcode.ErrInternal.WithMsg("hash password")
	}
	return string(hash), nil
}

func (s *Server) finishPhoneRegister(ctx context.Context, user *model.User, platformID int32) (*userpb.RegisterPhoneResp, error) {
	tok, err := s.issue(user.UserID, platformID)
	if err != nil {
		return nil, err
	}
	revoked, err := s.store.StartSession(ctx, tok)
	s.invalidate(ctx, revoked)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create session")
	}
	return &userpb.RegisterPhoneResp{
		User:              toPB(user),
		Token:             tok.Raw,
		ExpireTimeSeconds: s.issuer.ExpireSeconds(),
	}, nil
}
