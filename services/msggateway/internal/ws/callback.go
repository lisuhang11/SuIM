package ws

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/webhook"
)

type userStatusReq struct {
	CallbackCommand string `json:"callbackCommand"`
	OperationID     string `json:"operationID"`
	PlatformID      int    `json:"platformID"`
	Platform        string `json:"platform"`
	UserID          string `json:"userID"`
	Seq             int64  `json:"seq"`
	IsAppBackground bool   `json:"isAppBackground,omitempty"`
	ConnID          string `json:"connID,omitempty"`
}

func (r userStatusReq) GetCallbackCommand() string { return r.CallbackCommand }

func (s *Server) afterUserOnline(client *Client) {
	s.postUserStatus(webhook.CommandAfterUserOnline, s.hooks.AfterOnline, client, true, false)
}

func (s *Server) afterUserOffline(client *Client) {
	s.postUserStatus(webhook.CommandAfterUserOffline, s.hooks.AfterOffline, client, false, false)
}

func (s *Server) afterUserKickOff(client *Client) {
	s.postUserStatus(webhook.CommandAfterUserKickOff, s.hooks.AfterKickOff, client, false, true)
}

func (s *Server) postUserStatus(command string, after webhook.AfterConfig, client *Client, background, kick bool) {
	if s == nil || s.hooks.Client == nil || client == nil {
		return
	}
	req := userStatusReq{
		CallbackCommand: command,
		PlatformID:      client.platformID,
		Platform:        platformName(client.platformID),
		UserID:          client.userID,
		ConnID:          client.id,
	}
	if background {
		req.IsAppBackground = client.background
	}
	if kick {
		req.ConnID = ""
	}
	s.hooks.Client.AsyncPost(context.Background(), command, req, &webhook.CommonCallbackResp{}, &after)
}

func platformName(id int) string {
	switch id {
	case 1:
		return "IOS"
	case 2:
		return "Android"
	case 3:
		return "Windows"
	case 4:
		return "OSX"
	case 5:
		return "Web"
	case 6:
		return "MiniWeb"
	case 7:
		return "Linux"
	case 8:
		return "AndroidPad"
	case 9:
		return "IPad"
	case 10:
		return "Admin"
	default:
		return ""
	}
}
