// Package online 提供 Redis 集群在线状态与本机订阅推送。
package online

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	onlineKeyPrefix = "ONLINE:"
	OnlineChannel   = "online_change"
	OnlineExpire    = 30 * time.Minute
)

// ChangeEvent Redis Pub/Sub 载荷。
type ChangeEvent struct {
	UserID      string  `json:"user_id"`
	Status      int32   `json:"status"` // 1=online 0=offline
	PlatformIDs []int32 `json:"platform_ids"`
	Source      string  `json:"source,omitempty"` // 发布方实例 ID，用于忽略本机回环
}

// Store Redis ONLINE:{userID} ZSET（member=platformID, score=expireUnix）。
type Store struct {
	rdb        *redis.Client
	expire     time.Duration
	instanceID string
}

// NewStore 创建在线状态存储；rdb 为 nil 时所有操作 no-op。
func NewStore(rdb *redis.Client, instanceID string) *Store {
	return &Store{rdb: rdb, expire: OnlineExpire, instanceID: instanceID}
}

// InstanceID 返回本机实例标识。
func (s *Store) InstanceID() string {
	if s == nil {
		return ""
	}
	return s.instanceID
}

func (s *Store) key(userID string) string { return onlineKeyPrefix + userID }

// Enabled 是否启用了 Redis。
func (s *Store) Enabled() bool { return s != nil && s.rdb != nil }

// GetPlatforms 返回未过期的在线平台列表。
func (s *Store) GetPlatforms(ctx context.Context, userID string) ([]int32, error) {
	if !s.Enabled() {
		return nil, nil
	}
	members, err := s.rdb.ZRangeByScore(ctx, s.key(userID), &redis.ZRangeBy{
		Min: strconv.FormatInt(time.Now().Unix(), 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]int32, 0, len(members))
	for _, m := range members {
		v, err := strconv.Atoi(m)
		if err != nil {
			continue
		}
		out = append(out, int32(v))
	}
	return out, nil
}

// SetPlatformOnline 标记平台上线；changed 表示平台集合是否变化。
func (s *Store) SetPlatformOnline(ctx context.Context, userID string, platformID int32) (platforms []int32, changed bool, err error) {
	return s.apply(ctx, userID, []int32{platformID}, nil)
}

// SetPlatformOffline 标记平台下线。
func (s *Store) SetPlatformOffline(ctx context.Context, userID string, platformID int32) (platforms []int32, changed bool, err error) {
	return s.apply(ctx, userID, nil, []int32{platformID})
}

// Renew 续期本地仍在线的平台（不强制 publish）。
func (s *Store) Renew(ctx context.Context, userID string, platformIDs []int32) error {
	if !s.Enabled() || len(platformIDs) == 0 {
		return nil
	}
	_, _, err := s.apply(ctx, userID, platformIDs, nil)
	return err
}

func (s *Store) apply(ctx context.Context, userID string, online, offline []int32) ([]int32, bool, error) {
	if !s.Enabled() {
		return nil, false, nil
	}
	key := s.key(userID)
	now := time.Now()
	before, err := s.GetPlatforms(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	pipe := s.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now.Unix()-1, 10))
	for _, p := range offline {
		pipe.ZRem(ctx, key, strconv.Itoa(int(p)))
	}
	score := float64(now.Add(s.expire).Unix())
	for _, p := range online {
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: strconv.Itoa(int(p))})
	}
	pipe.Expire(ctx, key, s.expire)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, false, err
	}
	after, err := s.GetPlatforms(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	changed := !samePlatformSet(before, after)
	if changed {
		ev := ChangeEvent{
			UserID:      userID,
			PlatformIDs: after,
			Status:      statusFromPlatforms(after),
			Source:      s.instanceID,
		}
		raw, _ := json.Marshal(ev)
		if err := s.rdb.Publish(ctx, OnlineChannel, string(raw)).Err(); err != nil {
			return after, true, fmt.Errorf("publish: %w", err)
		}
	}
	return after, changed, nil
}

func statusFromPlatforms(platforms []int32) int32 {
	if len(platforms) > 0 {
		return 1
	}
	return 0
}

func samePlatformSet(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[int32]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; !ok {
			return false
		}
	}
	return true
}

// SubscribeChanges 订阅 online_change；回调在独立 goroutine 中调用。
func (s *Store) SubscribeChanges(ctx context.Context, onChange func(ChangeEvent)) error {
	if !s.Enabled() {
		<-ctx.Done()
		return ctx.Err()
	}
	pubsub := s.rdb.Subscribe(ctx, OnlineChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var ev ChangeEvent
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			if ev.UserID == "" {
				continue
			}
			onChange(ev)
		}
	}
}
