package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) SendSmsCode(ctx context.Context, req *userpb.SendSmsCodeReq) (*userpb.SendSmsCodeResp, error) {
	scene := strings.TrimSpace(req.GetScene())
	if !validScene(scene) {
		return nil, errorcode.ErrUserInvalidScene
	}
	area, national, err := NormalizePhone(req.GetPhone())
	if err != nil {
		return nil, err
	}
	if s.smsSender == nil || s.pepper == "" {
		return nil, errorcode.ErrInternal.WithMsg("sms is not configured")
	}
	ip := strings.TrimSpace(req.GetClientIp())
	device := strings.TrimSpace(req.GetDeviceId())
	if s.policy.CaptchaRequired {
		if s.captcha == nil {
			return nil, errorcode.ErrInternal.WithMsg("captcha is not configured")
		}
		if err := s.captcha.Verify(ctx, strings.TrimSpace(req.GetCaptchaTicket()), providerPhone(area, national), scene, ip); err != nil {
			return nil, errorcode.ErrUserCaptchaInvalid
		}
	}
	key := phoneKey(area, national)
	blocked, err := s.smsBlocked(ctx, key, ip, device)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check sms block")
	}
	if blocked {
		return nil, errorcode.ErrUserSmsRejected
	}
	if err := s.store.ReserveSMS(ctx, s.smsLimits(key, ip, device)); err != nil {
		if errors.Is(err, store.ErrSMSLimited) {
			return nil, errorcode.ErrUserSmsLimited
		}
		return nil, errorcode.ErrInternal.WithMsg("reserve sms")
	}
	user, err := s.store.GetUserByPhone(ctx, area, national)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	registered := err == nil && user != nil
	if sceneNeedsAccount(scene) != registered {
		return &userpb.SendSmsCodeResp{ExpireSeconds: s.policy.expireSeconds()}, nil
	}
	code, err := randomCode()
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("create sms code")
	}
	if _, err := s.smsSender.Send(ctx, providerPhone(area, national), code, scene); err != nil {
		return nil, errorcode.ErrUserSmsFailed
	}
	if err := s.store.SaveSMSCode(ctx, smsCodeKey(scene, key), codeHash(s.pepper, scene, area, national, code), s.policy.codeTTL()); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("store sms code")
	}
	return &userpb.SendSmsCodeResp{ExpireSeconds: s.policy.expireSeconds()}, nil
}
