package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/webhook"
)

type callbackUser struct {
	UserID   string `json:"userID"`
	Nickname string `json:"nickname"`
	FaceURL  string `json:"faceURL"`
	Ex       string `json:"ex"`
}

type beforeAddFriendReq struct {
	webhook.CommonCallbackReq
	FromUserID string       `json:"fromUserID"`
	ToUserID   string       `json:"toUserID"`
	ReqMsg     string       `json:"reqMsg"`
	Ex         string       `json:"ex"`
	FromUser   callbackUser `json:"fromUser"`
	ToUser     callbackUser `json:"toUser"`
}

type friendAgreeReq struct {
	webhook.CommonCallbackReq
	FromUserID   string `json:"fromUserID"`
	ToUserID     string `json:"blackUserID"`
	HandleResult int32  `json:"HandleResult"`
	HandleMsg    string `json:"HandleMsg"`
}

type setRemarkReq struct {
	webhook.CommonCallbackReq
	OwnerUserID  string `json:"ownerUserID"`
	FriendUserID string `json:"friendUserID"`
	Remark       string `json:"remark"`
}

type setRemarkResp struct {
	webhook.CommonCallbackResp
	Remark *string `json:"remark"`
}

type deleteFriendReq struct {
	webhook.CommonCallbackReq
	OwnerUserID  string `json:"ownerUserID"`
	FriendUserID string `json:"friendUserID"`
}

func (s *Server) beforeAddFriend(ctx context.Context, from, to, message string, fromUser, toUser callbackUser) error {
	if s.hooks.Client == nil || !s.hooks.BeforeAddFriend.Enable {
		return nil
	}
	req := &beforeAddFriendReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeAddFriend},
		FromUserID:        from,
		ToUserID:          to,
		ReqMsg:            message,
		FromUser:          fromUser,
		ToUser:            toUser,
	}
	return webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeAddFriend, req, &webhook.CommonCallbackResp{}, &s.hooks.BeforeAddFriend))
}

func (s *Server) afterAddFriend(ctx context.Context, from, to, message string, fromUser, toUser callbackUser) {
	if s.hooks.Client == nil {
		return
	}
	req := &beforeAddFriendReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterAddFriend},
		FromUserID:        from,
		ToUserID:          to,
		ReqMsg:            message,
		FromUser:          fromUser,
		ToUser:            toUser,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterAddFriend, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterAddFriend)
}

func (s *Server) beforeAddFriendAgree(ctx context.Context, from, to, handleMsg string, handleResult int32) error {
	if s.hooks.Client == nil || !s.hooks.BeforeAddFriendAgree.Enable {
		return nil
	}
	req := &friendAgreeReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeAddFriendAgree},
		FromUserID:        from,
		ToUserID:          to,
		HandleResult:      handleResult,
		HandleMsg:         handleMsg,
	}
	return webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeAddFriendAgree, req, &webhook.CommonCallbackResp{}, &s.hooks.BeforeAddFriendAgree))
}

func (s *Server) afterAddFriendAgree(ctx context.Context, from, to, handleMsg string, handleResult int32) {
	if s.hooks.Client == nil {
		return
	}
	req := &friendAgreeReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterAddFriendAgree},
		FromUserID:        from,
		ToUserID:          to,
		HandleResult:      handleResult,
		HandleMsg:         handleMsg,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterAddFriendAgree, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterAddFriendAgree)
}

func (s *Server) beforeSetFriendRemark(ctx context.Context, owner, friend, remark string) (string, error) {
	if s.hooks.Client == nil || !s.hooks.BeforeSetFriendRemark.Enable {
		return remark, nil
	}
	req := &setRemarkReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeSetFriendRemark},
		OwnerUserID:       owner,
		FriendUserID:      friend,
		Remark:            remark,
	}
	var resp setRemarkResp
	if err := webhook.Proceed(s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeSetFriendRemark, req, &resp, &s.hooks.BeforeSetFriendRemark)); err != nil {
		return "", err
	}
	if resp.Remark != nil {
		return *resp.Remark, nil
	}
	return remark, nil
}

func (s *Server) afterSetFriendRemark(ctx context.Context, owner, friend, remark string) {
	if s.hooks.Client == nil {
		return
	}
	req := &setRemarkReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterSetFriendRemark},
		OwnerUserID:       owner,
		FriendUserID:      friend,
		Remark:            remark,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterSetFriendRemark, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterSetFriendRemark)
}

func (s *Server) afterDeleteFriend(ctx context.Context, owner, friend string) {
	if s.hooks.Client == nil {
		return
	}
	req := &deleteFriendReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterDeleteFriend},
		OwnerUserID:       owner,
		FriendUserID:      friend,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterDeleteFriend, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterDeleteFriend)
}
