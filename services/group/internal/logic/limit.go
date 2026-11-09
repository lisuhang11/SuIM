package logic

import (
	"context"
	"errors"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

const (
	maxCreateMembers = 100
	maxInviteUsers   = 50

	createPerHour = 10
	createPerDay  = 30
	invitePerHour = 100
	invitePerDay  = 500
	joinPerHour   = 20
)

// 统计一个用户创建群、邀请和入群的次数。
type Quota interface {
	Reserve(ctx context.Context, limits []store.Limit) error
}

func (s *Server) reserveCreate(ctx context.Context, userID string) error {
	return s.reserve(ctx, userID, "create", 1, []store.Limit{
		{Limit: createPerHour, TTL: time.Hour},
		{Limit: createPerDay, TTL: 24 * time.Hour},
	})
}

func (s *Server) reserveInvite(ctx context.Context, userID string, people int) error {
	return s.reserve(ctx, userID, "invite", int64(people), []store.Limit{
		{Limit: invitePerHour, TTL: time.Hour},
		{Limit: invitePerDay, TTL: 24 * time.Hour},
	})
}

func (s *Server) reserveJoin(ctx context.Context, userID string) error {
	return s.reserve(ctx, userID, "join", 1, []store.Limit{
		{Limit: joinPerHour, TTL: time.Hour},
	})
}

func (s *Server) reserve(ctx context.Context, userID, action string, n int64, windows []store.Limit) error {
	if s == nil || s.quota == nil {
		return nil
	}
	limits := make([]store.Limit, 0, len(windows))
	for i, window := range windows {
		window.Key = "group:limit:" + action + ":" + windowName(i) + ":" + userID
		window.N = n
		limits = append(limits, window)
	}
	if err := s.quota.Reserve(ctx, limits); err != nil {
		if errors.Is(err, store.ErrLimited) {
			return errorcode.ErrGroupTooFrequent
		}
		return errorcode.ErrInternal.WithMsg("reserve group limit")
	}
	return nil
}

func windowName(i int) string {
	if i == 0 {
		return "hour"
	}
	return "day"
}
