package groupclient

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// Client asks the group service whether two users may add each other.
type Client struct {
	api grouppb.GroupServiceClient
}

func New(api grouppb.GroupServiceClient) *Client {
	return &Client{api: api}
}

func (c *Client) MemberFriendAllowed(ctx context.Context, user1, user2 string) (bool, error) {
	ctx, cancel := context.WithTimeout(rpcctx.WithUser(ctx, user1), 3*time.Second)
	defer cancel()
	resp, err := c.api.MemberFriendAllowed(ctx, &grouppb.MemberFriendAllowedReq{
		UserId_1: user1,
		UserId_2: user2,
	})
	if err != nil {
		return false, err
	}
	return resp.GetAllowed(), nil
}
