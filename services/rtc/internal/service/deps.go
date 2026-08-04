package service

import (
	"context"

	"rtc/internal/types"
)

type friendChecker interface {
	IsMutualFriend(ctx context.Context, user1, user2 string) (bool, error)
}

type presenceChecker interface {
	IsUserOnline(ctx context.Context, userID string) (bool, error)
}

type timelineWriter interface {
	WriteCallTimeline(ctx context.Context, call *types.Call) error
}

type offlinePusher interface {
	PushCallSignal(ctx context.Context, call *types.Call, action string, userIDs ...string) error
}
