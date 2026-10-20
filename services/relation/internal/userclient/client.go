package userclient

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
)

// Client checks that user IDs exist by calling the user service.
type Client struct {
	rpc userpb.UserServiceClient
}

func New(rpc userpb.UserServiceClient) *Client {
	return &Client{rpc: rpc}
}

func (c *Client) CheckUsers(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	resp, err := c.rpc.GetUsersByIDs(ctx, &userpb.GetUsersByIDsReq{UserIds: ids})
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check users")
	}
	users := resp.GetUsers()
	for _, id := range ids {
		if _, ok := users[id]; !ok {
			return errorcode.ErrRelationUserNotFound
		}
	}
	return nil
}
