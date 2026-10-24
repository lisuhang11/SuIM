package rpc

import (
	"context"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// MsgClient calls the msg service for send and history.
type MsgClient struct {
	client msgpb.MsgServiceClient
}

func NewMsgClient(client msgpb.MsgServiceClient) *MsgClient {
	return &MsgClient{client: client}
}

func (c *MsgClient) SendMsg(ctx context.Context, msg *msgpb.MsgData) (*msgpb.SendMsgResp, error) {
	return c.client.SendMsg(ctx, &msgpb.SendMsgReq{MsgData: msg})
}

func (c *MsgClient) GetMaxSeq(ctx context.Context, userID string) (*msgpb.GetMaxSeqResp, error) {
	return c.client.GetMaxSeq(ctx, &msgpb.GetMaxSeqReq{UserId: userID})
}

func (c *MsgClient) PullMessageBySeqs(ctx context.Context, req *msgpb.PullMessageBySeqsReq) (*msgpb.PushMessages, error) {
	return c.client.PullMessageBySeqs(ctx, req)
}

func (c *MsgClient) GetSeqMessage(ctx context.Context, req *msgpb.GetSeqMessageReq) (*msgpb.PushMessages, error) {
	return c.client.GetSeqMessage(ctx, req)
}

func (c *MsgClient) GetConversationsHasReadAndMaxSeq(ctx context.Context, req *msgpb.GetConversationsHasReadAndMaxSeqReq) (*msgpb.GetConversationsHasReadAndMaxSeqResp, error) {
	return c.client.GetConversationsHasReadAndMaxSeq(ctx, req)
}

func (c *MsgClient) GetLastMessage(ctx context.Context, req *msgpb.GetLastMessageReq) (*msgpb.GetLastMessageResp, error) {
	return c.client.GetLastMessage(ctx, req)
}
