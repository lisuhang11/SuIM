package logic

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/localcache"
	svcconfig "github.com/lisuhang11/SuIM/services/user/internal/config"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

const (
	profilePrefix = "suim:user:profile:"
	profileTTL    = 12 * time.Hour
)

// 按用户 ID 缓存用户行。登录仍读 MySQL，密码哈希不从这里返回。
type Profiles struct {
	rdb   redis.UniversalClient
	store *store.Store
	topic string
	local localcache.Cache[*model.User]
}

// 启动本进程的资料缓存，并订阅其他实例发来的删除。
func NewProfileCache(ctx context.Context, rdb redis.UniversalClient, st *store.Store, cfg svcconfig.LocalCache) (*Profiles, func()) {
	if cfg.Topic == "" {
		cfg.Topic = "suim:user:profile:delete"
	}
	if cfg.SlotNum <= 0 {
		cfg.SlotNum = 100
	}
	if cfg.SlotSize <= 0 {
		cfg.SlotSize = 2000
	}
	if cfg.SuccessExpire <= 0 {
		cfg.SuccessExpire = 300
	}
	if cfg.FailedExpire <= 0 {
		cfg.FailedExpire = 5
	}
	p := &Profiles{rdb: rdb, store: st, topic: cfg.Topic}
	p.local = localcache.New[*model.User](
		localcache.WithLocalSlotNum(cfg.SlotNum),
		localcache.WithLocalSlotSize(cfg.SlotSize),
		localcache.WithLinkDisable(),
		localcache.WithLocalSuccessTTL(time.Duration(cfg.SuccessExpire)*time.Second),
		localcache.WithLocalFailedTTL(time.Duration(cfg.FailedExpire)*time.Second),
	)
	subCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		subscribeProfileDeletes(subCtx, rdb, cfg.Topic, p.local.DelLocal)
	}()
	return p, func() {
		cancel()
		<-done
		p.local.Stop()
	}
}

// 按用户 ID 取资料。本进程和 Redis 都没有时读 MySQL。
func (p *Profiles) Get(ctx context.Context, userID string) (*model.User, error) {
	user, err := p.local.Get(ctx, userID, func(ctx context.Context) (*model.User, error) {
		if p.rdb != nil {
			cached, ok, err := loadProfile(ctx, p.rdb, userID)
			if err != nil {
				slog.Warn("read user profile cache", "err", err)
			} else if ok {
				return cached, nil
			}
		}
		row, err := p.store.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		row = publicCopy(row)
		if p.rdb != nil {
			if err := storeProfile(ctx, p.rdb, row); err != nil {
				slog.Warn("write user profile cache", "err", err)
			}
		}
		return row, nil
	})
	if err != nil {
		return nil, err
	}
	return publicCopy(user), nil
}

// 从 Redis 和本进程删掉这份资料，并通知其他实例删除。
func (p *Profiles) Drop(ctx context.Context, userIDs ...string) {
	if p == nil {
		return
	}
	ids := make([]string, 0, len(userIDs))
	keys := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id == "" {
			continue
		}
		ids = append(ids, id)
		keys = append(keys, profilePrefix+id)
	}
	if len(ids) == 0 {
		return
	}
	if p.rdb != nil {
		if err := p.rdb.Del(ctx, keys...).Err(); err != nil {
			slog.Warn("delete user profile cache", "err", err)
		}
		body, err := json.Marshal(ids)
		if err != nil {
			slog.Warn("encode user profile delete", "err", err)
		} else if err := p.rdb.Publish(ctx, p.topic, body).Err(); err != nil {
			slog.Warn("publish user profile delete", "err", err)
		}
	}
	p.local.DelLocal(ctx, ids...)
}

func subscribeProfileDeletes(ctx context.Context, rdb redis.UniversalClient, topic string, del func(context.Context, ...string)) {
	if rdb == nil || topic == "" || del == nil {
		return
	}
	for ctx.Err() == nil {
		pubsub := rdb.Subscribe(ctx, topic)
		ch := pubsub.Channel()
		closed := false
		for !closed && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				closed = true
			case msg, ok := <-ch:
				if !ok {
					closed = true
					break
				}
				var ids []string
				if err := json.Unmarshal([]byte(msg.Payload), &ids); err != nil {
					slog.Warn("user profile delete payload", "err", err)
					continue
				}
				if len(ids) > 0 {
					del(ctx, ids...)
				}
			}
		}
		_ = pubsub.Close()
		if ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

type profileSnap struct {
	UserID           string `json:"user_id"`
	Email            string `json:"email,omitempty"`
	Nickname         string `json:"nickname,omitempty"`
	AvatarURL        string `json:"avatar_url,omitempty"`
	AppMangerLevel   int32  `json:"app_manger_level,omitempty"`
	GlobalRecvMsgOpt int32  `json:"global_recv_msg_opt,omitempty"`
	IsActive         bool   `json:"is_active"`
	CreateTime       int64  `json:"create_time,omitempty"`
	AreaCode         string `json:"area_code,omitempty"`
	PhoneNumber      string `json:"phone_number,omitempty"`
	Account          string `json:"account,omitempty"`
	Gender           int32  `json:"gender,omitempty"`
	BirthTime        int64  `json:"birth_time,omitempty"`
	Level            int32  `json:"level,omitempty"`
	AllowVibration   int32  `json:"allow_vibration,omitempty"`
	AllowBeep        int32  `json:"allow_beep,omitempty"`
	AllowAddFriend   int32  `json:"allow_add_friend,omitempty"`
	RegisterType     int32  `json:"register_type,omitempty"`
	Ex               string `json:"ex,omitempty"`
	HasPassword      bool   `json:"has_password"`
}

func loadProfile(ctx context.Context, rdb redis.UniversalClient, userID string) (*model.User, bool, error) {
	raw, err := rdb.Get(ctx, profilePrefix+userID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var snap profileSnap
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, false, err
	}
	return snap.user(), true, nil
}

func storeProfile(ctx context.Context, rdb redis.UniversalClient, user *model.User) error {
	body, err := json.Marshal(snapOf(user))
	if err != nil {
		return err
	}
	return rdb.Set(ctx, profilePrefix+user.UserID, body, profileJitter()).Err()
}

func snapOf(u *model.User) profileSnap {
	snap := profileSnap{
		UserID:           u.UserID,
		Email:            u.Email,
		Nickname:         u.Nickname,
		AvatarURL:        u.AvatarURL,
		AppMangerLevel:   u.AppMangerLevel,
		GlobalRecvMsgOpt: u.GlobalRecvMsgOpt,
		IsActive:         u.IsActive,
		AreaCode:         u.AreaCode,
		PhoneNumber:      u.PhoneNumber,
		Account:          u.Account,
		Gender:           u.Gender,
		Level:            u.Level,
		AllowVibration:   u.AllowVibration,
		AllowBeep:        u.AllowBeep,
		AllowAddFriend:   u.AllowAddFriend,
		RegisterType:     u.RegisterType,
		Ex:               u.Ex,
		HasPassword:      u.HasPassword,
	}
	if !u.CreateTime.IsZero() {
		snap.CreateTime = u.CreateTime.UnixMilli()
	}
	if !u.BirthTime.IsZero() {
		snap.BirthTime = u.BirthTime.UnixMilli()
	}
	return snap
}

func (s profileSnap) user() *model.User {
	u := &model.User{
		UserID:           s.UserID,
		Email:            s.Email,
		Nickname:         s.Nickname,
		AvatarURL:        s.AvatarURL,
		AppMangerLevel:   s.AppMangerLevel,
		GlobalRecvMsgOpt: s.GlobalRecvMsgOpt,
		IsActive:         s.IsActive,
		AreaCode:         s.AreaCode,
		PhoneNumber:      s.PhoneNumber,
		Account:          s.Account,
		Gender:           s.Gender,
		Level:            s.Level,
		AllowVibration:   s.AllowVibration,
		AllowBeep:        s.AllowBeep,
		AllowAddFriend:   s.AllowAddFriend,
		RegisterType:     s.RegisterType,
		Ex:               s.Ex,
		HasPassword:      s.HasPassword,
	}
	if s.CreateTime > 0 {
		u.CreateTime = time.UnixMilli(s.CreateTime).UTC()
	}
	if s.BirthTime > 0 {
		u.BirthTime = time.UnixMilli(s.BirthTime).UTC()
	}
	return u
}

func publicCopy(u *model.User) *model.User {
	if u == nil {
		return nil
	}
	cp := *u
	cp.PasswordHash = ""
	return &cp
}

func profileJitter() time.Duration {
	spread := int64(profileTTL) / 5
	return profileTTL + time.Duration(rand.Int64N(spread*2)-spread)
}
