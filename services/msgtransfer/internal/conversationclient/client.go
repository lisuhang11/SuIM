package conversationclient

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// Client creates conversation rows through the conversation service.
type Client struct {
	conversations conversationpb.ConversationServiceClient
	groups        grouppb.GroupServiceClient
}

func New(conversations conversationpb.ConversationServiceClient, groups grouppb.GroupServiceClient) *Client {
	return &Client{conversations: conversations, groups: groups}
}

func (c *Client) Ensure(ctx context.Context, conversationID, sendID, recvID string, sessionType int32) error {
	ctx, cancel := context.WithTimeout(rpcctx.WithUser(ctx, sendID), 3*time.Second)
	defer cancel()
	_, err := c.conversations.CreateSingleChatConversations(ctx, &conversationpb.CreateSingleChatConversationsReq{
		SendId:           sendID,
		RecvId:           recvID,
		ConversationId:   conversationID,
		ConversationType: sessionType,
	})
	return err
}

func (c *Client) EnsureGroup(ctx context.Context, groupID, callerID string, sessionType int32) error {
	ctx, cancel := context.WithTimeout(rpcctx.WithUser(ctx, callerID), 3*time.Second)
	defer cancel()
	members, err := c.groups.GetGroupMemberUserIDs(ctx, &grouppb.GetGroupMemberUserIDsReq{
		GroupId: groupID,
		UserId:  callerID,
	})
	if err != nil {
		return err
	}
	_, err = c.conversations.CreateGroupChatConversations(ctx, &conversationpb.CreateGroupChatConversationsReq{
		GroupId:          groupID,
		UserIds:          members.GetUserIds(),
		ConversationType: sessionType,
	})
	return err
}
