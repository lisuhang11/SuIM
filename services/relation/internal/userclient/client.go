package userclient

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
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

func (c *Client) GetUsers(ctx context.Context, ids []string) (map[string]model.UserProfile, error) {
	out := make(map[string]model.UserProfile, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	resp, err := c.rpc.GetUsersByIDs(ctx, &userpb.GetUsersByIDsReq{UserIds: ids})
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query users")
	}
	for id, user := range resp.GetUsers() {
		if user == nil {
			continue
		}
		out[id] = model.UserProfile{UserID: user.GetUserId(), Nickname: user.GetNickname(), AvatarURL: user.GetAvatarUrl()}
	}
	return out, nil
}
