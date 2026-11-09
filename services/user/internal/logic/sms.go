package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

// 投递一次性验证码。除非进程以短信调试日志启动，否则不能把验证码写进日志。
type SMSSender interface {
	Send(ctx context.Context, phone, code, scene string) (string, error)
}

// 校验服务商票据。客户端自己填的字符串不算通过。
type CaptchaChecker interface {
	Verify(ctx context.Context, ticket, phone, scene, ip string) error
}

// 发送侧的频率限制和验证码有效期。
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

// 接入短信发送、人机校验、频率策略和验证码哈希盐。
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
