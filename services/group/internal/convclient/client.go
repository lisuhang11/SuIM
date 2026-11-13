package convclient

import (
	"context"
	"time"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
)

// Client writes conversation rows and per-user sequence ceilings through the conversation service.
type Client struct {
	api conversationpb.ConversationServiceClient
}

func New(api conversationpb.ConversationServiceClient) *Client {
	return &Client{api: api}
}

func (c *Client) CreateGroup(ctx context.Context, groupID string, userIDs []string, conversationType int32) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.api.CreateGroupChatConversations(ctx, &conversationpb.CreateGroupChatConversationsReq{
		GroupId:          groupID,
		UserIds:          userIDs,
		ConversationType: conversationType,
	})
	return err
}

func (c *Client) SetConversationMaxSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.api.SetConversationMaxSeq(ctx, &conversationpb.SetConversationMaxSeqReq{
		ConversationId: conversationID,
		OwnerUserIds:   userIDs,
		MaxSeq:         seq,
	})
	return err
}
