package logic

import (
	"context"
	"log/slog"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

type beforeOfflinePushReq struct {
	webhook.CommonCallbackReq
	UserIDs []string       `json:"userIDList"`
	MsgData *msgpb.MsgData `json:"msgData"`
}

func (r beforeOfflinePushReq) GetCallbackCommand() string { return webhook.CommandBeforeOfflinePush }

type beforeOfflinePushResp struct {
	webhook.CommonCallbackResp
	UserIDs []string `json:"userIDList"`
}

func (r *beforeOfflinePushResp) Parse() error { return r.CommonCallbackResp.Parse() }

// webhookBeforeOfflinePush 返回回调改写后的接收人，未开启时原样返回。
func (s *Server) webhookBeforeOfflinePush(ctx context.Context, userIDs []string, msg *msgpb.MsgData) ([]string, error) {
	if s.hooks.Client == nil || !s.hooks.BeforeOfflinePush.Enable {
		return userIDs, nil
	}
	req := beforeOfflinePushReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeOfflinePush},
		UserIDs:           userIDs,
		MsgData:           msg,
	}
	resp := &beforeOfflinePushResp{}
	err := s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeOfflinePush, req, resp, &s.hooks.BeforeOfflinePush)
	if err := webhook.Proceed(err); err != nil {
		if s.hooks.BeforeOfflinePush.FailedContinue {
			slog.Warn("before offline push webhook failed, continue", "err", err)
			return userIDs, nil
		}
		return nil, err
	}
	if len(resp.UserIDs) > 0 {
		return resp.UserIDs, nil
	}
	return userIDs, nil
}
