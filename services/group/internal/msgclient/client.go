package msgclient

import (
	"context"
	"time"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// Client sends a notification through the msg service.
type Client struct {
	api msgpb.MsgServiceClient
}

func New(api msgpb.MsgServiceClient) *Client {
	return &Client{api: api}
}

func (c *Client) SendNotification(ctx context.Context, msg *msgpb.MsgData) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.api.SendMsg(ctx, &msgpb.SendMsgReq{MsgData: msg})
	return err
}
