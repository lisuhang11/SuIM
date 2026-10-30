package logic

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const (
	contentTyping     int32 = 113
	notificationBegin int32 = 1000
	notificationEnd   int32 = 5000
)

type msgCallbackReq struct {
	webhook.CommonCallbackReq
	SendID           string   `json:"sendID"`
	RecvID           string   `json:"recvID,omitempty"`
	GroupID          string   `json:"groupID,omitempty"`
	ServerMsgID      string   `json:"serverMsgID"`
	ClientMsgID      string   `json:"clientMsgID"`
	SenderPlatformID int32    `json:"senderPlatformID"`
	SenderNickname   string   `json:"senderNickname"`
	SenderFaceURL    string   `json:"faceURL"`
	SessionType      int32    `json:"sessionType"`
	MsgFrom          int32    `json:"msgFrom"`
	ContentType      int32    `json:"contentType"`
	Status           int32    `json:"status"`
	SendTime         int64    `json:"sendTime"`
	CreateTime       int64    `json:"createTime"`
	Content          string   `json:"content"`
	Seq              int64    `json:"seq"`
	AtUserIDList     []string `json:"atUserList,omitempty"`
	Ex               string   `json:"ex"`
}

type offlinePush struct {
	Title         string `json:"title"`
	Desc          string `json:"desc"`
	Ex            string `json:"ex"`
	IOSPushSound  string `json:"iOSPushSound"`
	IOSBadgeCount bool   `json:"iOSBadgeCount"`
	SignalInfo    string `json:"signalInfo"`
}

type modifyMsgResp struct {
	webhook.CommonCallbackResp
	Content          *string          `json:"content"`
	RecvID           *string          `json:"recvID"`
	GroupID          *string          `json:"groupID"`
	ClientMsgID      *string          `json:"clientMsgID"`
	ServerMsgID      *string          `json:"serverMsgID"`
	SenderPlatformID *int32           `json:"senderPlatformID"`
	SenderNickname   *string          `json:"senderNickname"`
	SenderFaceURL    *string          `json:"senderFaceURL"`
	SessionType      *int32           `json:"sessionType"`
	MsgFrom          *int32           `json:"msgFrom"`
	ContentType      *int32           `json:"contentType"`
	Status           *int32           `json:"status"`
	Options          *map[string]bool `json:"options"`
	OfflinePushInfo  *offlinePush     `json:"offlinePushInfo"`
	AtUserIDList     *[]string        `json:"atUserIDList"`
	AttachedInfo     *string          `json:"attachedInfo"`
	Ex               *string          `json:"ex"`
}

func (s *Server) beforeSend(ctx context.Context, msg *msgpb.MsgData) error {
	if s.hooks.Client == nil || msg == nil || skipMsg(msg, nil) {
		return nil
	}
	switch msg.SessionType {
	case sessionSingle, 4:
		if !s.hooks.BeforeSendSingle.Enable || skipMsg(msg, s.hooks.BeforeSendSingle.DeniedTypes) {
			return nil
		}
		req := messageCallback(webhook.CommandBeforeSendSingleMsg, msg)
		var resp webhook.CommonCallbackResp
		return s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeSendSingleMsg, req, &resp, &s.hooks.BeforeSendSingle.BeforeConfig)
	case sessionGroup, sessionSuper:
		if !s.hooks.BeforeSendGroup.Enable || skipMsg(msg, s.hooks.BeforeSendGroup.DeniedTypes) {
			return nil
		}
		req := messageCallback(webhook.CommandBeforeSendGroupMsg, msg)
		var resp webhook.CommonCallbackResp
		return s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeSendGroupMsg, req, &resp, &s.hooks.BeforeSendGroup.BeforeConfig)
	default:
		return nil
	}
}

func (s *Server) beforeModify(ctx context.Context, msg *msgpb.MsgData) error {
	if s.hooks.Client == nil || !s.hooks.BeforeMsgModify.Enable || skipMsg(msg, s.hooks.BeforeMsgModify.DeniedTypes) {
		return nil
	}
	req := messageCallback(webhook.CommandBeforeMsgModify, msg)
	var resp modifyMsgResp
	if err := s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeMsgModify, req, &resp, &s.hooks.BeforeMsgModify.BeforeConfig); err != nil {
		return err
	}
	applyModify(msg, &resp)
	if len(msg.Content) == 0 {
		return errorcode.ErrMsgInvalidArgs.WithMsg("content is required")
	}
	return nil
}

func (s *Server) afterSend(ctx context.Context, msg *msgpb.MsgData) {
	if s.hooks.Client == nil || msg == nil {
		return
	}
	var command string
	var after MsgAfter
	switch msg.SessionType {
	case sessionSingle, 4:
		command = webhook.CommandAfterSendSingleMsg
		after = s.hooks.AfterSendSingle
	case sessionGroup, sessionSuper:
		command = webhook.CommandAfterSendGroupMsg
		after = s.hooks.AfterSendGroup
	default:
		return
	}
	if skipMsg(msg, after.DeniedTypes) || !allowAttention(msg, after.AttentionIds) {
		return
	}
	req := messageCallback(command, msg)
	s.hooks.Client.AsyncPostWithQuery(ctx, command, req, &webhook.CommonCallbackResp{}, &after.AfterConfig, map[string]string{
		webhook.Key: messageKey(msg),
	})
}

func messageCallback(command string, msg *msgpb.MsgData) msgCallbackReq {
	return msgCallbackReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: command},
		SendID:            msg.SendId,
		RecvID:            msg.RecvId,
		GroupID:           msg.GroupId,
		ServerMsgID:       msg.ServerMsgId,
		ClientMsgID:       msg.ClientMsgId,
		SenderPlatformID:  msg.SenderPlatformId,
		SenderNickname:    msg.SenderNickname,
		SenderFaceURL:     msg.SenderFaceUrl,
		SessionType:       msg.SessionType,
		MsgFrom:           msg.MsgFrom,
		ContentType:       msg.ContentType,
		Status:            msg.Status,
		SendTime:          msg.SendTime,
		CreateTime:        msg.CreateTime,
		Content:           messageText(msg),
		Seq:               msg.Seq,
		AtUserIDList:      msg.AtUserIdList,
		Ex:                msg.Ex,
	}
}

func applyModify(msg *msgpb.MsgData, resp *modifyMsgResp) {
	if resp.Content != nil {
		msg.Content = []byte(*resp.Content)
	}
	if resp.RecvID != nil {
		msg.RecvId = *resp.RecvID
	}
	if resp.GroupID != nil {
		msg.GroupId = *resp.GroupID
	}
	if resp.ClientMsgID != nil {
		msg.ClientMsgId = *resp.ClientMsgID
	}
	if resp.ServerMsgID != nil {
		msg.ServerMsgId = *resp.ServerMsgID
	}
	if resp.SenderPlatformID != nil {
		msg.SenderPlatformId = *resp.SenderPlatformID
	}
	if resp.SenderNickname != nil {
		msg.SenderNickname = *resp.SenderNickname
	}
	if resp.SenderFaceURL != nil {
		msg.SenderFaceUrl = *resp.SenderFaceURL
	}
	if resp.SessionType != nil {
		msg.SessionType = *resp.SessionType
	}
	if resp.MsgFrom != nil {
		msg.MsgFrom = *resp.MsgFrom
	}
	if resp.ContentType != nil {
		msg.ContentType = *resp.ContentType
	}
	if resp.Status != nil {
		msg.Status = *resp.Status
	}
	if resp.Options != nil {
		msg.Options = *resp.Options
	}
	if resp.AtUserIDList != nil {
		msg.AtUserIdList = *resp.AtUserIDList
	}
	if resp.AttachedInfo != nil {
		msg.AttachedInfo = *resp.AttachedInfo
	}
	if resp.Ex != nil {
		msg.Ex = *resp.Ex
	}
	if resp.OfflinePushInfo != nil {
		msg.OfflinePushInfo = &msgpb.OfflinePushInfo{
			Title:         resp.OfflinePushInfo.Title,
			Desc:          resp.OfflinePushInfo.Desc,
			Ex:            resp.OfflinePushInfo.Ex,
			IosPushSound:  resp.OfflinePushInfo.IOSPushSound,
			IosBadgeCount: resp.OfflinePushInfo.IOSBadgeCount,
			SignalInfo:    resp.OfflinePushInfo.SignalInfo,
		}
	}
}

func messageText(msg *msgpb.MsgData) string {
	if msg.ContentType >= notificationBegin && msg.ContentType <= notificationEnd {
		tips := &msgpb.TipsComm{}
		if err := proto.Unmarshal(msg.Content, tips); err == nil && tips.JsonDetail != "" {
			return tips.JsonDetail
		}
	}
	return string(msg.Content)
}

func messageKey(msg *msgpb.MsgData) string {
	body, _ := json.Marshal(struct {
		SendID  string `json:"sendID"`
		RecvID  string `json:"recvID"`
		GroupID string `json:"groupID"`
	}{msg.SendId, msg.RecvId, msg.GroupId})
	return base64.StdEncoding.EncodeToString(body)
}

func skipMsg(msg *msgpb.MsgData, denied []int32) bool {
	if msg == nil || msgconv.IsNotification(msg) {
		return true
	}
	if msg.ContentType == contentTyping || (msg.ContentType >= notificationBegin && msg.ContentType <= notificationEnd) {
		return true
	}
	for _, item := range denied {
		if msg.ContentType == item {
			return true
		}
	}
	return false
}

func allowAttention(msg *msgpb.MsgData, ids []string) bool {
	if len(ids) == 0 {
		return true
	}
	switch msg.SessionType {
	case sessionSingle, 4:
		return containsID(ids, msg.RecvId)
	case sessionGroup, sessionSuper:
		return containsID(ids, msg.GroupId)
	default:
		return false
	}
}

func containsID(list []string, id string) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}
	return false
}
