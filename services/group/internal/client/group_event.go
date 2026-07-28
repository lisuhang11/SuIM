package client

import (
	"context"
	"encoding/json"
	"errors"

	"group/internal/types/interfaces"

	convpb "SuIM/proto/conversationpb"
	msgpb "SuIM/proto/messagepb"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

const groupEventContentType int32 = 1100

type groupEventPublisher struct {
	conversation convpb.ConversationClient
	message      msgpb.MessageClient
}

// NewGroupEventPublisher 创建群事件发布器，负责会话记录和群系统消息联动。
func NewGroupEventPublisher(conversationConn, messageConn grpc.ClientConnInterface) interfaces.GroupEventPublisher {
	return &groupEventPublisher{
		conversation: convpb.NewConversationClient(conversationConn),
		message:      msgpb.NewMessageClient(messageConn),
	}
}

func distinct(values ...[]string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, list := range values {
		for _, value := range list {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func (p *groupEventPublisher) Publish(ctx context.Context, event interfaces.GroupEvent) error {
	var publishErr error
	if event.Type == "group.created" || event.Type == "group.members_joined" || event.Type == "group.application_accepted" {
		if len(event.SubjectUserIDs) > 0 {
			_, err := p.conversation.CreateGroupChatConversations(ctx, &convpb.CreateGroupChatConversationsReq{GroupId: event.GroupID, UserIds: distinct(event.SubjectUserIDs)})
			publishErr = errors.Join(publishErr, err)
		}
	}

	recipients := distinct(event.RecipientUserIDs)
	if event.Type == "group.member_kicked" || event.Type == "group.member_quit" || event.Type == "group.dismissed" {
		recipients = distinct(recipients, event.SubjectUserIDs)
	}
	content, err := json.Marshal(map[string]any{
		"type":             event.Type,
		"group_id":         event.GroupID,
		"operator_user_id": event.OperatorUserID,
		"subject_user_ids": event.SubjectUserIDs,
	})
	if err != nil {
		return errors.Join(publishErr, err)
	}
	recvIDs := make([]string, 0, len(recipients))
	for _, userID := range recipients {
		if userID != event.OperatorUserID {
			recvIDs = append(recvIDs, userID)
		}
	}
	if event.OperatorUserID != "" && len(recipients) > 0 {
		_, err = p.message.SendMsg(ctx, &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
			ClientMsgId: uuid.NewString(), ConversationId: "gid_" + event.GroupID,
			SendId: event.OperatorUserID, GroupId: event.GroupID, SessionType: 2,
			MsgFrom: 1, ContentType: groupEventContentType, Content: string(content),
			RecvUserIds: recvIDs,
		}})
		publishErr = errors.Join(publishErr, err)
	}

	if event.Type == "group.member_kicked" || event.Type == "group.member_quit" || event.Type == "group.dismissed" {
		for _, userID := range distinct(event.SubjectUserIDs) {
			_, err := p.conversation.DeleteConversations(ctx, &convpb.DeleteConversationsReq{OwnerUserId: userID, ConversationIds: []string{"gid_" + event.GroupID}})
			publishErr = errors.Join(publishErr, err)
		}
	}
	return publishErr
}
