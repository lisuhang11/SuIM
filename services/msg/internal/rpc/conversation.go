package rpc

import (
	"context"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// ConversationClient reads conversation rows through the conversation service.
type ConversationClient struct {
	client conversationpb.ConversationServiceClient
}

func NewConversationClient(client conversationpb.ConversationServiceClient) *ConversationClient {
	return &ConversationClient{client: client}
}

func (c *ConversationClient) PinnedConversationIDs(ctx context.Context, userID string) ([]string, error) {
	resp, err := c.client.GetPinnedConversationIDs(ctx, &conversationpb.GetPinnedConversationIDsReq{UserId: userID})
	if err != nil {
		return nil, err
	}
	return resp.GetConversationIds(), nil
}
