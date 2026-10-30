package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/conversation/internal/store"
)

const (
	msgFromSystem = 200
	sessionSingle = 1

	contentConversationChanged = 1300
	contentConversationPrivate = 1701

	optNotNotification = "isNotNotification"
)

func (s *Server) notifyChanged(ctx context.Context, conversationID string, owners []string) {
	for _, owner := range owners {
		s.sendUserNotice(ctx, owner, owner, contentConversationChanged, conversationUpdateTips{
			UserID:             owner,
			ConversationIDList: []string{conversationID},
		})
	}
}

func (s *Server) notifyPrivate(ctx context.Context, conversationID string, notes []store.PrivateNote) {
	for _, note := range notes {
		if note.Owner == "" || note.Peer == "" || note.Owner == note.Peer {
			continue
		}
		s.sendUserNotice(ctx, note.Owner, note.Peer, contentConversationPrivate, conversationPrivateTips{
			RecvID:         note.Peer,
			SendID:         note.Owner,
			IsPrivate:      note.Enabled,
			ConversationID: conversationID,
		})
	}
}

type conversationUpdateTips struct {
	UserID             string   `json:"userID"`
	ConversationIDList []string `json:"conversationIDList"`
}

type conversationPrivateTips struct {
	RecvID         string `json:"recvID"`
	SendID         string `json:"sendID"`
	IsPrivate      bool   `json:"isPrivate"`
	ConversationID string `json:"conversationID"`
}

func (s *Server) sendUserNotice(ctx context.Context, sendID, recvID string, contentType int32, detail any) {
	if s.sender == nil || sendID == "" || recvID == "" {
		return
	}
	body, err := json.Marshal(detail)
	if err != nil {
		slog.Warn("conversation notice", "err", err, "contentType", contentType)
		return
	}
	content, err := json.Marshal(struct {
		Detail string `json:"detail"`
	}{Detail: string(body)})
	if err != nil {
		slog.Warn("conversation notice", "err", err, "contentType", contentType)
		return
	}
	now := s.now().UnixMilli()
	msg := &msgpb.MsgData{
		SendId:      sendID,
		RecvId:      recvID,
		SessionType: sessionSingle,
		MsgFrom:     msgFromSystem,
		ContentType: contentType,
		Content:     content,
		SendTime:    now,
		CreateTime:  now,
		Options:     map[string]bool{optNotNotification: false},
		ClientMsgId: newNoticeID(),
	}
	if err := s.sender.SendNotification(ctx, msg); err != nil {
		slog.Warn("conversation notice", "err", err, "sendID", sendID, "recvID", recvID, "contentType", contentType)
	}
}

func newNoticeID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(buf[:])
}
