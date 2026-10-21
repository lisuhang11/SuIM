package logic

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/config"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

const afterMsgSaveDBCommand = "callbackAfterMsgSaveDBCommand"

type afterMsgSaveDBReq struct {
	webhook.CommonCallbackReq
	SendID           string `json:"sendID"`
	RecvID           string `json:"recvID"`
	GroupID          string `json:"groupID"`
	ServerMsgID      string `json:"serverMsgID"`
	ClientMsgID      string `json:"clientMsgID"`
	SenderPlatformID int32  `json:"senderPlatformID"`
	SenderNickname   string `json:"senderNickname"`
	SessionType      int32  `json:"sessionType"`
	MsgFrom          int32  `json:"msgFrom"`
	ContentType      int32  `json:"contentType"`
	Status           int32  `json:"status"`
	SendTime         int64  `json:"sendTime"`
	CreateTime       int64  `json:"createTime"`
	Content          string `json:"content"`
	Seq              int64  `json:"seq"`
	Ex               string `json:"ex"`
}

func (h *MySQLHandler) webhookAfterMsgSaveDB(ctx context.Context, msg *msgpb.MsgData) {
	if h.webhook == nil || !allowAfterMsg(msg, h.after) {
		return
	}
	req := &afterMsgSaveDBReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: afterMsgSaveDBCommand},
		SendID:            msg.SendId,
		RecvID:            msg.RecvId,
		GroupID:           msg.GroupId,
		ServerMsgID:       msg.ServerMsgId,
		ClientMsgID:       msg.ClientMsgId,
		SenderPlatformID:  msg.SenderPlatformId,
		SenderNickname:    msg.SenderNickname,
		SessionType:       msg.SessionType,
		MsgFrom:           msg.MsgFrom,
		ContentType:       msg.ContentType,
		Status:            msg.Status,
		SendTime:          msg.SendTime,
		CreateTime:        msg.CreateTime,
		Content:           messageContent(msg),
		Seq:               msg.Seq,
		Ex:                msg.Ex,
	}
	h.webhook.AsyncPostWithQuery(ctx, afterMsgSaveDBCommand, req, &webhook.CommonCallbackResp{}, &webhook.AfterConfig{
		Enable:  h.after.Enable,
		Timeout: h.after.Timeout,
	}, map[string]string{webhook.Key: keyQuery(msg)})
}

func messageContent(msg *msgpb.MsgData) string {
	if msg.ContentType >= model.NotificationBegin && msg.ContentType <= model.NotificationEnd {
		tips := &msgpb.TipsComm{}
		if err := proto.Unmarshal(msg.Content, tips); err == nil && tips.JsonDetail != "" {
			return tips.JsonDetail
		}
	}
	return string(msg.Content)
}

func keyQuery(msg *msgpb.MsgData) string {
	body, _ := json.Marshal(struct {
		SendID  string `json:"sendID"`
		RecvID  string `json:"recvID"`
		GroupID string `json:"groupID"`
	}{msg.SendId, msg.RecvId, msg.GroupId})
	return base64.StdEncoding.EncodeToString(body)
}

func allowAfterMsg(msg *msgpb.MsgData, after config.AfterMsgSaveDB) bool {
	if msg == nil {
		return false
	}
	if msg.ContentType >= model.NotificationBegin && msg.ContentType <= model.NotificationEnd {
		return false
	}
	if msg.ContentType == model.ContentTyping {
		return false
	}
	for _, denied := range after.DeniedTypes {
		if msg.ContentType == denied {
			return false
		}
	}
	if len(after.AttentionIds) == 0 {
		return true
	}
	switch msg.SessionType {
	case model.SessionSingle, model.SessionNotification:
		return contains(after.AttentionIds, msg.RecvId)
	case model.SessionWriteGroup, model.SessionReadGroup:
		return contains(after.AttentionIds, msg.GroupId)
	default:
		return false
	}
}

func contains(list []string, id string) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}
	return false
}
