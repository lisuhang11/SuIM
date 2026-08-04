package online

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	pkgnotif "SuIM/pkg/notification"
	messagepb "SuIM/proto/messagepb"
	"msggateway/internal/types"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

// StatusSnapshot 订阅时返回的当前状态。
type StatusSnapshot struct {
	UserID      string  `json:"user_id"`
	Status      int32   `json:"status"`
	PlatformIDs []int32 `json:"platform_ids,omitempty"`
}

// LocalPlatformsFn 查询本机某用户在线平台。
type LocalPlatformsFn func(userID string) []int32

// WriteConnFn 向单连接写 WS 消息（typ + data）。
type WriteConnFn func(conn types.WritableConn, typ string, data json.RawMessage) error

// Hub 协调 Redis 在线集、本机订阅与 tip 推送。
type Hub struct {
	store *Store
	sub   *Subscription
	local LocalPlatformsFn
	write WriteConnFn
}

// NewHub 创建 presence hub；store 可为 nil（仅本机）。
func NewHub(store *Store, local LocalPlatformsFn, write WriteConnFn) *Hub {
	return &Hub{
		store: store,
		sub:   NewSubscription(),
		local: local,
		write: write,
	}
}

// OnConnect 平台上线。
func (h *Hub) OnConnect(ctx context.Context, userID string, platformID int32) {
	var platforms []int32
	var changed bool
	if h.store != nil && h.store.Enabled() {
		var err error
		platforms, changed, err = h.store.SetPlatformOnline(ctx, userID, platformID)
		if err != nil {
			slog.Warn("online store set online failed", "user_id", userID, "error", err)
		}
	} else {
		platforms = h.mergeLocal(userID, platformID, true)
		changed = true
	}
	if changed {
		h.broadcastLocal(userID, platforms)
	}
}

// OnDisconnect 平台下线（该平台已无连接时调用）。
func (h *Hub) OnDisconnect(ctx context.Context, userID string, platformID int32) {
	var platforms []int32
	var changed bool
	if h.store != nil && h.store.Enabled() {
		var err error
		platforms, changed, err = h.store.SetPlatformOffline(ctx, userID, platformID)
		if err != nil {
			slog.Warn("online store set offline failed", "user_id", userID, "error", err)
		}
	} else {
		platforms = h.mergeLocal(userID, platformID, false)
		changed = true
	}
	if changed {
		h.broadcastLocal(userID, platforms)
	}
}

func (h *Hub) mergeLocal(userID string, platformID int32, online bool) []int32 {
	set := map[int32]struct{}{}
	if h.local != nil {
		for _, p := range h.local(userID) {
			set[p] = struct{}{}
		}
	}
	if online {
		set[platformID] = struct{}{}
	} else {
		delete(set, platformID)
	}
	out := make([]int32, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}

// Subscribe 注册订阅并返回当前快照。
func (h *Hub) Subscribe(ctx context.Context, conn types.WritableConn, userIDs []string) []StatusSnapshot {
	h.sub.Sub(conn, userIDs, nil)
	out := make([]StatusSnapshot, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid == "" {
			continue
		}
		platforms := h.lookupPlatforms(ctx, uid)
		out = append(out, StatusSnapshot{
			UserID:      uid,
			Status:      statusFromPlatforms(platforms),
			PlatformIDs: platforms,
		})
	}
	return out
}

// Unsubscribe 取消订阅。
func (h *Hub) Unsubscribe(conn types.WritableConn, userIDs []string) {
	h.sub.Sub(conn, nil, userIDs)
}

// DelConn 连接断开清理订阅。
func (h *Hub) DelConn(conn types.WritableConn) {
	h.sub.DelConn(conn)
}

func (h *Hub) lookupPlatforms(ctx context.Context, userID string) []int32 {
	if h.store != nil && h.store.Enabled() {
		if plats, err := h.store.GetPlatforms(ctx, userID); err == nil {
			return plats
		}
	}
	if h.local != nil {
		return h.local(userID)
	}
	return nil
}

// GetStatus 查询在线状态（优先 Redis）。
func (h *Hub) GetStatus(ctx context.Context, userID string) StatusSnapshot {
	platforms := h.lookupPlatforms(ctx, userID)
	return StatusSnapshot{
		UserID:      userID,
		Status:      statusFromPlatforms(platforms),
		PlatformIDs: platforms,
	}
}

// HandleRemoteChange 处理其他网关经 Redis 广播的变更。
func (h *Hub) HandleRemoteChange(ev ChangeEvent) {
	if h.store != nil && ev.Source != "" && ev.Source == h.store.InstanceID() {
		return // 本机已在 OnConnect/OnDisconnect 推过
	}
	h.broadcastLocal(ev.UserID, ev.PlatformIDs)
}

func (h *Hub) broadcastLocal(userID string, platforms []int32) {
	subs := h.sub.Subscribers(userID)
	if len(subs) == 0 || h.write == nil {
		return
	}
	raw, err := buildOnlineTipRaw(userID, platforms)
	if err != nil {
		slog.Warn("build online tip failed", "error", err)
		return
	}
	for _, conn := range subs {
		if conn == nil || conn.IsClosed() {
			continue
		}
		if err := h.write(conn, types.MsgTypePush, raw); err != nil {
			slog.Debug("presence tip write failed", "error", err)
		}
	}
}

// StartRedisSubscriber 阻塞订阅 Redis 变更。
func (h *Hub) StartRedisSubscriber(ctx context.Context) {
	if h.store == nil || !h.store.Enabled() {
		return
	}
	_ = h.store.SubscribeChanges(ctx, func(ev ChangeEvent) {
		h.HandleRemoteChange(ev)
	})
}

// StartRenewal 定时续期本机在线用户。
func (h *Hub) StartRenewal(ctx context.Context, listOnline func() map[string][]int32, interval time.Duration) {
	if h.store == nil || !h.store.Enabled() {
		return
	}
	if interval <= 0 {
		interval = OnlineExpire / 3
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for uid, plats := range listOnline() {
				if err := h.store.Renew(ctx, uid, plats); err != nil {
					slog.Debug("online renew failed", "user_id", uid, "error", err)
				}
			}
		}
	}
}

func buildOnlineTipRaw(userID string, platforms []int32) (json.RawMessage, error) {
	tips := pkgnotif.UserOnlineStatusTips{
		UserID:      userID,
		Status:      statusFromPlatforms(platforms),
		PlatformIDs: platforms,
	}
	content, err := json.Marshal(tips)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	msg := &messagepb.MsgData{
		ClientMsgId: uuid.NewString(),
		SendId:      pkgnotif.SystemSenderID,
		RecvId:      userID,
		MsgFrom:     pkgnotif.MsgFromSystem,
		ContentType: pkgnotif.UserOnlineStatusNotification,
		SessionType: pkgnotif.SessionTypeSingle,
		Content:     string(content),
		SendTime:    now,
		CreateTime:  now,
	}
	raw, err := protojson.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}
