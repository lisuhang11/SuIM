package client

import (
	"context"

	pb "SuIM/proto/messagepb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MessageClient wraps message gRPC for last-message preview.
type MessageClient struct {
	client pb.MessageClient
}

func NewMessageClient(conn grpc.ClientConnInterface) *MessageClient {
	return &MessageClient{client: pb.NewMessageClient(conn)}
}

func withForwardedMD(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md.Copy())
}

// GetLastMessage fetches the last visible message per conversation for the caller.
func (c *MessageClient) GetLastMessage(ctx context.Context, conversationIDs []string) (map[string]*pb.MsgData, error) {
	if len(conversationIDs) == 0 {
		return map[string]*pb.MsgData{}, nil
	}
	resp, err := c.client.GetLastMessage(withForwardedMD(ctx), &pb.GetLastMessageReq{ConversationIds: conversationIDs})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Msgs == nil {
		return map[string]*pb.MsgData{}, nil
	}
	return resp.Msgs, nil
}
