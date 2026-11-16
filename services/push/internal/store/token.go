package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	fcmTokenPrefix = "suim:fcm:token:"
	badgePrefix    = "suim:badge:"
	getuiTokenKey  = "suim:getui:token"
)

// TokenStore 在 Redis 里维护推送 token 与角标计数。
type TokenStore struct {
	rdb redis.UniversalClient
}

func NewTokenStore(rdb redis.UniversalClient) *TokenStore {
	return &TokenStore{rdb: rdb}
}

func fcmKey(userID string, platformID int32) string {
	return fmt.Sprintf("%s%s:%d", fcmTokenPrefix, userID, platformID)
}

func badgeKey(userID string) string { return badgePrefix + userID }

// SetFCMToken 写入或清空一台设备的 FCM token，空 token 表示删除。
func (s *TokenStore) SetFCMToken(ctx context.Context, userID string, platformID int32, token string) error {
	if s == nil || s.rdb == nil || userID == "" {
		return nil
	}
	if token == "" {
		return s.rdb.Del(ctx, fcmKey(userID, platformID)).Err()
	}
	return s.rdb.Set(ctx, fcmKey(userID, platformID), token, 0).Err()
}

// DelFCMToken 删除一台设备的 FCM token。
func (s *TokenStore) DelFCMToken(ctx context.Context, userID string, platformID int32) error {
	if s == nil || s.rdb == nil || userID == "" {
		return nil
	}
	return s.rdb.Del(ctx, fcmKey(userID, platformID)).Err()
}

// FCMTokens 返回用户所有平台上的 FCM token。
func (s *TokenStore) FCMTokens(ctx context.Context, userID string) ([]string, error) {
	if s == nil || s.rdb == nil || userID == "" {
		return nil, nil
	}
	pattern := fcmTokenPrefix + userID + ":*"
	var out []string
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			token, err := s.rdb.Get(ctx, key).Result()
			if err == nil && token != "" {
				out = append(out, token)
			}
		}
		cursor = next
		if cursor == 0 {
			return out, nil
		}
	}
}

// SetBadge 覆盖用户的角标计数。
func (s *TokenStore) SetBadge(ctx context.Context, userID string, count int64) error {
	if s == nil || s.rdb == nil || userID == "" {
		return nil
	}
	return s.rdb.Set(ctx, badgeKey(userID), count, 0).Err()
}

// IncrBadge 增加用户的角标计数并返回新值。
func (s *TokenStore) IncrBadge(ctx context.Context, userID string, delta int64) (int64, error) {
	if s == nil || s.rdb == nil || userID == "" {
		return 0, nil
	}
	return s.rdb.IncrBy(ctx, badgeKey(userID), delta).Result()
}

// Badge 读取用户的角标计数，缺行按 0 处理。
func (s *TokenStore) Badge(ctx context.Context, userID string) (int64, error) {
	if s == nil || s.rdb == nil || userID == "" {
		return 0, nil
	}
	count, err := s.rdb.Get(ctx, badgeKey(userID)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// GetuiToken 读取缓存的个推 token，缺行返回空串。
func (s *TokenStore) GetuiToken(ctx context.Context) (string, error) {
	if s == nil || s.rdb == nil {
		return "", nil
	}
	token, err := s.rdb.Get(ctx, getuiTokenKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	return token, err
}

// SetGetuiToken 缓存个推 token 并设置过期时间。
func (s *TokenStore) SetGetuiToken(ctx context.Context, token string, ttl time.Duration) error {
	if s == nil || s.rdb == nil {
		return nil
	}
	return s.rdb.Set(ctx, getuiTokenKey, token, ttl).Err()
}
