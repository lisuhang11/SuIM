package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"group/internal/types"

	"github.com/redis/go-redis/v9"
)

const groupInfoKeyPrefix = "GROUP_INFO:"

// GroupInfoCache 群资料旁路缓存（对齐 OpenIM GROUP_INFO，简化版 cache-aside）。
type GroupInfoCache struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewGroupInfoCache 创建助手；rdb 为 nil 时所有操作降级为空操作。
func NewGroupInfoCache(rdb *redis.Client, ttl time.Duration) *GroupInfoCache {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &GroupInfoCache{rdb: rdb, ttl: ttl}
}

func groupInfoKey(groupID string) string {
	return groupInfoKeyPrefix + groupID
}

// Get 读取单个群缓存；miss 或禁用时返回 (nil, false)。
func (c *GroupInfoCache) Get(ctx context.Context, groupID string) (*types.Group, bool) {
	if c == nil || c.rdb == nil || groupID == "" {
		return nil, false
	}
	raw, err := c.rdb.Get(ctx, groupInfoKey(groupID)).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			slog.WarnContext(ctx, "group info cache get failed", "group_id", groupID, "error", err)
		}
		return nil, false
	}
	g, err := unmarshalCachedGroup(raw)
	if err != nil {
		slog.WarnContext(ctx, "group info cache unmarshal failed", "group_id", groupID, "error", err)
		return nil, false
	}
	return g, true
}

// MGet 批量读缓存，返回命中 map 与未命中 ID 列表。
func (c *GroupInfoCache) MGet(ctx context.Context, ids []string) (hit map[string]*types.Group, miss []string) {
	hit = make(map[string]*types.Group, len(ids))
	if c == nil || c.rdb == nil || len(ids) == 0 {
		return hit, append([]string(nil), ids...)
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = groupInfoKey(id)
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		slog.WarnContext(ctx, "group info cache mget failed", "error", err)
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
		g, err := unmarshalCachedGroup([]byte(raw))
		if err != nil {
			miss = append(miss, id)
			continue
		}
		hit[id] = g
	}
	return hit, miss
}

// Set 回填单个群；失败仅记日志。
func (c *GroupInfoCache) Set(ctx context.Context, group *types.Group) {
	if c == nil || c.rdb == nil || group == nil || group.GroupID == "" {
		return
	}
	raw, err := marshalCachedGroup(group)
	if err != nil {
		slog.WarnContext(ctx, "group info cache marshal failed", "group_id", group.GroupID, "error", err)
		return
	}
	if err := c.rdb.Set(ctx, groupInfoKey(group.GroupID), raw, c.ttl).Err(); err != nil {
		slog.WarnContext(ctx, "group info cache set failed", "group_id", group.GroupID, "error", err)
	}
}

// Del 写后失效；失败仅记日志。
func (c *GroupInfoCache) Del(ctx context.Context, groupIDs ...string) {
	if c == nil || c.rdb == nil || len(groupIDs) == 0 {
		return
	}
	keys := make([]string, 0, len(groupIDs))
	for _, id := range groupIDs {
		if id != "" {
			keys = append(keys, groupInfoKey(id))
		}
	}
	if len(keys) == 0 {
		return
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		slog.WarnContext(ctx, "group info cache del failed", "error", err)
	}
}

type cachedGroup struct {
	GroupID                string `json:"group_id"`
	GroupName              string `json:"group_name"`
	Notification           string `json:"notification"`
	Introduction           string `json:"introduction"`
	FaceURL                string `json:"face_url"`
	CreateTime             int64  `json:"create_time"`
	Ex                     string `json:"ex"`
	Status                 int    `json:"status"`
	CreatorUserID          string `json:"creator_user_id"`
	GroupType              int    `json:"group_type"`
	NeedVerification       int    `json:"need_verification"`
	LookMemberInfo         int    `json:"look_member_info"`
	ApplyMemberFriend      int    `json:"apply_member_friend"`
	NotificationUpdateTime int64  `json:"notification_update_time"`
	NotificationUserID     string `json:"notification_user_id"`
	MemberCount            int    `json:"member_count"`
	OwnerUserID            string `json:"owner_user_id"`
}

func marshalCachedGroup(g *types.Group) ([]byte, error) {
	return json.Marshal(cachedGroup{
		GroupID:                g.GroupID,
		GroupName:              g.GroupName,
		Notification:           g.Notification,
		Introduction:           g.Introduction,
		FaceURL:                g.FaceURL,
		CreateTime:             g.CreateTime.UnixMilli(),
		Ex:                     g.Ex,
		Status:                 g.Status,
		CreatorUserID:          g.CreatorUserID,
		GroupType:              g.GroupType,
		NeedVerification:       g.NeedVerification,
		LookMemberInfo:         g.LookMemberInfo,
		ApplyMemberFriend:      g.ApplyMemberFriend,
		NotificationUpdateTime: g.NotificationUpdateTime.UnixMilli(),
		NotificationUserID:     g.NotificationUserID,
		MemberCount:            g.MemberCount,
		OwnerUserID:            g.OwnerUserID,
	})
}

func unmarshalCachedGroup(raw []byte) (*types.Group, error) {
	var c cachedGroup
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	g := &types.Group{
		GroupID:            c.GroupID,
		GroupName:          c.GroupName,
		Notification:       c.Notification,
		Introduction:       c.Introduction,
		FaceURL:            c.FaceURL,
		Ex:                 c.Ex,
		Status:             c.Status,
		CreatorUserID:      c.CreatorUserID,
		GroupType:          c.GroupType,
		NeedVerification:   c.NeedVerification,
		LookMemberInfo:     c.LookMemberInfo,
		ApplyMemberFriend:  c.ApplyMemberFriend,
		NotificationUserID: c.NotificationUserID,
		MemberCount:        c.MemberCount,
		OwnerUserID:        c.OwnerUserID,
	}
	if c.CreateTime > 0 {
		g.CreateTime = time.UnixMilli(c.CreateTime)
	}
	if c.NotificationUpdateTime > 0 {
		g.NotificationUpdateTime = time.UnixMilli(c.NotificationUpdateTime)
	}
	return g, nil
}
