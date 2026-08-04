package interfaces

import (
	"context"

	"rtc/internal/types"
)

// CallService 通话业务层契约。
type CallService interface {
	Invite(ctx context.Context, callerID, calleeID, mediaType string) (*types.Call, string, error)
	Accept(ctx context.Context, userID, callID string) (*types.Call, string, error)
	Reject(ctx context.Context, userID, callID string) (*types.Call, error)
	Cancel(ctx context.Context, userID, callID string) (*types.Call, error)
	Hangup(ctx context.Context, userID, callID string) (*types.Call, error)
	GetCall(ctx context.Context, userID, callID string) (*types.Call, error)
	RefreshToken(ctx context.Context, userID, callID string) (token, roomName string, err error)
}

// CallRepository 通话持久层契约。
type CallRepository interface {
	Create(ctx context.Context, call *types.Call) error
	GetByID(ctx context.Context, callID string) (*types.Call, error)
	Update(ctx context.Context, call *types.Call) error
	FindActiveByUser(ctx context.Context, userID string) (*types.Call, error)
}
