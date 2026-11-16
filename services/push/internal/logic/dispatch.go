package logic

import (
	"context"
	"log/slog"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const (
	optOfflinePush = "offlinePush"
	contentTyping  = 113
)

// dispatch 把一条消息里离线的人挑出来，经过回调后交给厂商推送。
func (s *Server) dispatch(ctx context.Context, body *msgpb.PushMsgData) error {
	data := body.GetMsgData()
	if data == nil {
		return nil
	}
	if enabled, ok := data.GetOptions()[optOfflinePush]; ok && !enabled {
		return nil
	}
	if data.GetContentType() == contentTyping {
		return nil
	}
	var offline []string
	for _, userID := range s.recipients(ctx, data) {
		if userID == "" || userID == data.GetSendId() {
			continue
		}
		platforms, err := s.online.Get(ctx, userID)
		if err != nil {
			slog.Warn("read online status", "err", err, "userID", userID)
		}
		if len(platforms) == 0 {
			offline = append(offline, userID)
		}
	}
	if len(offline) == 0 {
		return nil
	}
	filtered, err := s.webhookBeforeOfflinePush(ctx, offline, data)
	if err != nil {
		return err
	}
	if len(filtered) == 0 {
		return nil
	}
	title, content, opts := pushContent(data)
	return s.pusher.Push(ctx, filtered, title, content, opts)
}

// recipients 群消息取全部成员，单聊只取接收方。
func (s *Server) recipients(ctx context.Context, data *msgpb.MsgData) []string {
	if groupID := data.GetGroupId(); groupID != "" {
		ids, err := s.members.UserIDs(ctx, groupID, data.GetSendId())
		if err != nil {
			slog.Warn("read group members", "err", err, "groupID", groupID)
			return nil
		}
		return ids
	}
	if recvID := data.GetRecvId(); recvID != "" {
		return []string{recvID}
	}
	return nil
}
