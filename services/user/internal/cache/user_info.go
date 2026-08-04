package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"user/internal/types"

	"github.com/redis/go-redis/v9"
)

const userInfoKeyPrefix = "USER_INFO:"

// UserInfoCache 用户公开资料旁路缓存（不含 password_hash）。
type UserInfoCache struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewUserInfoCache 创建助手；rdb 为 nil 时所有操作降级为空操作。
func NewUserInfoCache(rdb *redis.Client, ttl time.Duration) *UserInfoCache {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &UserInfoCache{rdb: rdb, ttl: ttl}
}

func userInfoKey(userID string) string {
	return userInfoKeyPrefix + userID
}

// Get 读取单个用户缓存；miss 或禁用时返回 (nil, false)。
func (c *UserInfoCache) Get(ctx context.Context, userID string) (*types.User, bool) {
	if c == nil || c.rdb == nil || userID == "" {
		return nil, false
	}
	raw, err := c.rdb.Get(ctx, userInfoKey(userID)).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			slog.WarnContext(ctx, "user info cache get failed", "user_id", userID, "error", err)
		}
		return nil, false
	}
	u, err := unmarshalCachedUser(raw)
	if err != nil {
		slog.WarnContext(ctx, "user info cache unmarshal failed", "user_id", userID, "error", err)
		return nil, false
	}
	return u, true
}

// MGet 批量读缓存，返回命中 map 与未命中 ID 列表（保持 ids 顺序中的 miss）。
func (c *UserInfoCache) MGet(ctx context.Context, ids []string) (hit map[string]*types.User, miss []string) {
	hit = make(map[string]*types.User, len(ids))
	if c == nil || c.rdb == nil || len(ids) == 0 {
		return hit, append([]string(nil), ids...)
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = userInfoKey(id)
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		slog.WarnContext(ctx, "user info cache mget failed", "error", err)
		return hit, append([]string(nil), ids...)
	}
	for i, v := range vals {
		id := ids[i]
		if v == nil {
			miss = append(miss, id)
			continue
		}
		raw, ok := v.(string)
		if !ok {
			miss = append(miss, id)
			continue
		}
		u, err := unmarshalCachedUser([]byte(raw))
		if err != nil {
			miss = append(miss, id)
			continue
		}
		hit[id] = u
	}
	return hit, miss
}

// Set 回填单个用户；失败仅记日志。
func (c *UserInfoCache) Set(ctx context.Context, user *types.User) {
	if c == nil || c.rdb == nil || user == nil || user.UserID == "" {
		return
	}
	raw, err := marshalCachedUser(user)
	if err != nil {
		slog.WarnContext(ctx, "user info cache marshal failed", "user_id", user.UserID, "error", err)
		return
	}
	if err := c.rdb.Set(ctx, userInfoKey(user.UserID), raw, c.ttl).Err(); err != nil {
		slog.WarnContext(ctx, "user info cache set failed", "user_id", user.UserID, "error", err)
	}
}

// Del 写后失效；失败仅记日志。
func (c *UserInfoCache) Del(ctx context.Context, userIDs ...string) {
	if c == nil || c.rdb == nil || len(userIDs) == 0 {
		return
	}
	keys := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id != "" {
			keys = append(keys, userInfoKey(id))
		}
	}
	if len(keys) == 0 {
		return
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		slog.WarnContext(ctx, "user info cache del failed", "error", err)
	}
}

type cachedUser struct {
	UserID           string `json:"user_id"`
	Email            string `json:"email"`
	Nickname         string `json:"nickname"`
	AvatarURL        string `json:"avatar_url"`
	Ex               string `json:"ex"`
	AppMangerLevel   int    `json:"app_manger_level"`
	GlobalRecvMsgOpt int    `json:"global_recv_msg_opt"`
	IsActive         bool   `json:"is_active"`
	CreateTime       int64  `json:"create_time"`
	UpdatedAt        int64  `json:"updated_at"`
}

func marshalCachedUser(u *types.User) ([]byte, error) {
	return json.Marshal(cachedUser{
		UserID:           u.UserID,
		Email:            u.Email,
		Nickname:         u.Nickname,
		AvatarURL:        u.AvatarURL,
		Ex:               u.Ex,
		AppMangerLevel:   u.AppMangerLevel,
		GlobalRecvMsgOpt: u.GlobalRecvMsgOpt,
		IsActive:         u.IsActive,
		CreateTime:       u.CreateTime.UnixMilli(),
		UpdatedAt:        u.UpdatedAt.UnixMilli(),
	})
}

func unmarshalCachedUser(raw []byte) (*types.User, error) {
	var c cachedUser
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	u := &types.User{
		UserID:           c.UserID,
		Email:            c.Email,
		Nickname:         c.Nickname,
		AvatarURL:        c.AvatarURL,
		Ex:               c.Ex,
		AppMangerLevel:   c.AppMangerLevel,
		GlobalRecvMsgOpt: c.GlobalRecvMsgOpt,
		IsActive:         c.IsActive,
	}
	if c.CreateTime > 0 {
		u.CreateTime = time.UnixMilli(c.CreateTime)
	}
	if c.UpdatedAt > 0 {
		u.UpdatedAt = time.UnixMilli(c.UpdatedAt)
	}
	return u, nil
}
