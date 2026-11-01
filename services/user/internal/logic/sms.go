package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

// SMSSender delivers a one-time code. The code must not be logged unless the
// process was started with sms.debugLogCode.
type SMSSender interface {
	Send(ctx context.Context, phone, code, scene string) (string, error)
}

// CaptchaChecker verifies a provider ticket. A client-supplied string is not proof.
type CaptchaChecker interface {
	Verify(ctx context.Context, ticket, phone, scene, ip string) error
}

// SMSPolicy is the send-side limit and code lifetime configuration.
type SMSPolicy struct {
	CodeTTL          time.Duration
	RetryTTL         time.Duration
	PhonePerMinute   int64
	PhonePerHour     int64
	PhonePerDay      int64
	IPPerMinute      int64
	DevicePerMinute  int64
	MaxAttempts      int
	CaptchaRequired  bool
	BlacklistPhones  map[string]struct{}
	BlacklistIPs     map[string]struct{}
	BlacklistDevices map[string]struct{}
}

func (s *Server) UseSMS(sender SMSSender, captcha CaptchaChecker, policy SMSPolicy, pepper string) {
	if s == nil {
		return
	}
	s.smsSender = sender
	s.captcha = captcha
	s.policy = policy
	s.pepper = pepper
}

func (s *Server) smsBlocked(ctx context.Context, key, ip, device string) (bool, error) {
	if inSet(s.policy.BlacklistPhones, key) || inSet(s.policy.BlacklistIPs, ip) || inSet(s.policy.BlacklistDevices, device) {
		return true, nil
	}
	keys := []string{"sms:block:phone:" + key}
	if ip != "" {
		keys = append(keys, "sms:block:ip:"+ip)
	}
	if device != "" {
		keys = append(keys, "sms:block:device:"+device)
	}
	return s.store.SMSBlocked(ctx, keys)
}

func (s *Server) smsLimits(key, ip, device string) []store.SMSLimit {
	p := s.policy
	limits := []store.SMSLimit{
		{Key: "sms:limit:min:phone:" + key, Limit: p.PhonePerMinute, TTL: time.Minute},
		{Key: "sms:limit:hour:phone:" + key, Limit: p.PhonePerHour, TTL: time.Hour},
		{Key: "sms:limit:day:phone:" + key, Limit: p.PhonePerDay, TTL: 24 * time.Hour},
	}
	if ip != "" {
		limits = append(limits, store.SMSLimit{Key: "sms:limit:min:ip:" + ip, Limit: p.IPPerMinute, TTL: time.Minute})
	}
	if device != "" {
		limits = append(limits, store.SMSLimit{Key: "sms:limit:min:device:" + device, Limit: p.DevicePerMinute, TTL: time.Minute})
	}
	return limits
}

func smsCodeKey(scene, key string) string {
	return "sms:code:" + scene + ":" + key
}

func (p SMSPolicy) codeTTL() time.Duration {
	if p.CodeTTL <= 0 {
		return 300 * time.Second
	}
	return p.CodeTTL
}

func (p SMSPolicy) retryTTL() time.Duration {
	if p.RetryTTL <= 0 {
		return 120 * time.Second
	}
	return p.RetryTTL
}

func (p SMSPolicy) expireSeconds() int64 {
	return int64(p.codeTTL() / time.Second)
}

func (p SMSPolicy) attempts() int {
	if p.MaxAttempts <= 0 {
		return 5
	}
	return p.MaxAttempts
}

func inSet(set map[string]struct{}, key string) bool {
	if key == "" || len(set) == 0 {
		return false
	}
	_, ok := set[key]
	return ok
}
