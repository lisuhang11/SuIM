package rpc

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
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
	return c.client.SendMsg(rpcctx.WithUser(ctx, msg.GetSendId()), &msgpb.SendMsgReq{MsgData: msg})
}

func (c *MsgClient) GetMaxSeq(ctx context.Context, userID string) (*msgpb.GetMaxSeqResp, error) {
	return c.client.GetMaxSeq(rpcctx.WithUser(ctx, userID), &msgpb.GetMaxSeqReq{UserId: userID})
}

func (c *MsgClient) PullMessageBySeqs(ctx context.Context, req *msgpb.PullMessageBySeqsReq) (*msgpb.PushMessages, error) {
	return c.client.PullMessageBySeqs(rpcctx.WithUser(ctx, req.GetUserId()), req)
}

func (c *MsgClient) GetSeqMessage(ctx context.Context, req *msgpb.GetSeqMessageReq) (*msgpb.PushMessages, error) {
	return c.client.GetSeqMessage(rpcctx.WithUser(ctx, req.GetUserId()), req)
}

func (c *MsgClient) GetConversationsHasReadAndMaxSeq(ctx context.Context, req *msgpb.GetConversationsHasReadAndMaxSeqReq) (*msgpb.GetConversationsHasReadAndMaxSeqResp, error) {
	return c.client.GetConversationsHasReadAndMaxSeq(rpcctx.WithUser(ctx, req.GetUserId()), req)
}

func (c *MsgClient) GetLastMessage(ctx context.Context, req *msgpb.GetLastMessageReq) (*msgpb.GetLastMessageResp, error) {
	return c.client.GetLastMessage(rpcctx.WithUser(ctx, req.GetUserId()), req)
}

func (c *MsgClient) MarkMsgsAsRead(ctx context.Context, req *msgpb.MarkMsgsAsReadReq) (*msgpb.MarkMsgsAsReadResp, error) {
	return c.client.MarkMsgsAsRead(rpcctx.WithUser(ctx, req.GetUserId()), req)
}

func (c *MsgClient) MarkConversationAsRead(ctx context.Context, req *msgpb.MarkConversationAsReadReq) (*msgpb.MarkConversationAsReadResp, error) {
	return c.client.MarkConversationAsRead(rpcctx.WithUser(ctx, req.GetUserId()), req)
}

func (c *MsgClient) SetConversationHasReadSeq(ctx context.Context, req *msgpb.SetConversationHasReadSeqReq) (*msgpb.SetConversationHasReadSeqResp, error) {
	return c.client.SetConversationHasReadSeq(rpcctx.WithUser(ctx, req.GetUserId()), req)
}
