package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
)

// 为新的单聊或通知会话写行。重复调用不覆盖已有行，也不发会话变更通知。通知会话只有接收方一行。
func (s *Server) CreateSingleChatConversations(ctx context.Context, req *conversationpb.CreateSingleChatConversationsReq) (*conversationpb.CreateSingleChatConversationsResp, error) {
	sendID, err := normalizeUserID(req.GetSendId())
	if err != nil {
		return nil, err
	}
	recvID, err := normalizeUserID(req.GetRecvId())
	if err != nil {
		return nil, err
	}
	if sendID == recvID {
		return nil, errorcode.ErrConversationInvalidArgs.WithMsg("send_id and recv_id are the same")
	}
	kind := req.GetConversationType()
	if kind == 0 {
		kind = model.ConversationSingle
	}
	if kind != model.ConversationSingle && kind != model.ConversationNotification {
		return nil, errorcode.ErrConversationInvalidArgs.WithMsg("conversation_type is invalid")
	}
	conversationID := strings.TrimSpace(req.GetConversationId())
	if conversationID == "" {
		conversationID = singleConversationID(kind, sendID, recvID)
	}
	if len(conversationID) > 255 {
		return nil, errorcode.ErrConversationInvalidArgs.WithMsg("conversation_id is invalid")
	}
	rows := []*model.Conversation{{
		OwnerUserID:      recvID,
		ConversationID:   conversationID,
		ConversationType: kind,
		UserID:           sendID,
	}}
	if kind == model.ConversationSingle {
		rows = append(rows, &model.Conversation{
			OwnerUserID:      sendID,
			ConversationID:   conversationID,
			ConversationType: kind,
			UserID:           recvID,
		})
	}
	if err := s.store.CreateMissing(ctx, rows, s.now()); err != nil {
		return nil, err
	}
	return &conversationpb.CreateSingleChatConversationsResp{}, nil
}

func singleConversationID(kind int32, sendID, recvID string) string {
	a, b := sendID, recvID
	if a > b {
		a, b = b, a
	}
	prefix := "si_"
	if kind == model.ConversationNotification {
		prefix = "n_"
	}
	return prefix + a + "_" + b
}
