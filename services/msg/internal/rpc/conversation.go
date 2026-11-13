package rpc

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
	"github.com/lisuhang11/SuIM/services/msg/internal/logic"
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

// RecvOpt reads ownerUserID's receive option. The conversation service only
// returns that row to its owner, so this call carries the owner as the operator.
func (c *ConversationClient) RecvOpt(ctx context.Context, ownerUserID, conversationID string) (logic.RecvSetting, error) {
	ctx = rpcctx.WithUser(ctx, ownerUserID)
	resp, err := c.client.GetConversation(ctx, &conversationpb.GetConversationReq{
		OwnerUserId:    ownerUserID,
		ConversationId: conversationID,
	})
	if err != nil {
		if code, ok := errorcode.CodeOf(err); ok && code == errorcode.ConversationNotFound {
			return logic.RecvSetting{}, nil
		}
		return logic.RecvSetting{}, err
	}
	row := resp.GetConversation()
	if row == nil {
		return logic.RecvSetting{}, nil
	}
	return logic.RecvSetting{Found: true, Opt: row.GetRecvMsgOpt()}, nil
}
