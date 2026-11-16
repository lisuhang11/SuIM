package rpc

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	pushpb "github.com/lisuhang11/SuIM/proto/push"
)

// PushClient clears device push tokens on logout.
type PushClient struct {
	client pushpb.PushServiceClient
}

func NewPushClient(client pushpb.PushServiceClient) *PushClient {
	return &PushClient{client: client}
}

// DelUserPushToken clears the token of one platform for the given user.
func (c *PushClient) DelUserPushToken(ctx context.Context, userID string, platformID int32) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.client.DelUserPushToken(rpcctx.WithUser(ctx, userID), &pushpb.DelUserPushTokenReq{
		UserId:     userID,
		PlatformId: platformID,
	})
	return err
}
