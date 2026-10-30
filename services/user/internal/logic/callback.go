package logic

import (
	"context"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

type callbackUser struct {
	UserID   string `json:"userID"`
	Nickname string `json:"nickname"`
	FaceURL  string `json:"faceURL"`
	Ex       string `json:"ex"`
}

type registerCallbackReq struct {
	webhook.CommonCallbackReq
	Users []callbackUser `json:"users"`
}

type registerCallbackResp struct {
	webhook.CommonCallbackResp
	Users []callbackUser `json:"users"`
}

type updateUserReq struct {
	webhook.CommonCallbackReq
	UserID   string  `json:"userID"`
	Nickname *string `json:"nickName"`
	FaceURL  *string `json:"faceURL"`
	Ex       *string `json:"ex"`
}

type updateUserResp struct {
	webhook.CommonCallbackResp
	Nickname *string `json:"nickName"`
	FaceURL  *string `json:"faceURL"`
	Ex       *string `json:"ex"`
}

type wrappedString struct {
	Value string `json:"value"`
}

type updateUserExReq struct {
	webhook.CommonCallbackReq
	UserID   string         `json:"userID"`
	Nickname *wrappedString `json:"nickName"`
	FaceURL  *wrappedString `json:"faceURL"`
	Ex       *wrappedString `json:"ex"`
}

type updateUserExResp struct {
	webhook.CommonCallbackResp
	Nickname *wrappedString `json:"nickName"`
	FaceURL  *wrappedString `json:"faceURL"`
	Ex       *wrappedString `json:"ex"`
}

type afterUpdateUserReq struct {
	webhook.CommonCallbackReq
	UserID   string `json:"userID"`
	Nickname string `json:"nickName"`
	FaceURL  string `json:"faceURL"`
	Ex       string `json:"ex"`
}

func (s *Server) beforeRegister(ctx context.Context, user *model.User) error {
	if s.hooks.Client == nil || !s.hooks.BeforeUserRegister.Enable {
		return nil
	}
	req := registerCallbackReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeUserRegister},
		Users:             []callbackUser{{UserID: user.UserID, Nickname: user.Nickname}},
	}
	var resp registerCallbackResp
	if err := s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeUserRegister, req, &resp, &s.hooks.BeforeUserRegister); err != nil {
		return err
	}
	if len(resp.Users) == 0 {
		return nil
	}
	nickname := resp.Users[0].Nickname
	if nickname == "" {
		return nil
	}
	if utf8.RuneCountInString(nickname) > 255 {
		return errorcode.ErrUserInvalidUsername
	}
	user.Nickname = nickname
	return nil
}

func (s *Server) afterRegister(ctx context.Context, user *model.User) {
	if s.hooks.Client == nil {
		return
	}
	req := registerCallbackReq{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterUserRegister},
		Users:             []callbackUser{{UserID: user.UserID, Nickname: user.Nickname, FaceURL: user.AvatarURL, Ex: user.Ex}},
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterUserRegister, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterUserRegister)
}

func (s *Server) beforeUpdateUser(ctx context.Context, id string, old *model.User, patch *model.UserPatch) error {
	if s.hooks.Client == nil {
		return nil
	}
	profile := patch.Nickname != nil || patch.AvatarURL != nil || patch.Ex != nil
	if profile && s.hooks.BeforeUpdateUserInfo.Enable {
		req := updateUserReq{
			CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeUpdateUserInfo},
			UserID:            id,
			Nickname:          patch.Nickname,
			FaceURL:           patch.AvatarURL,
			Ex:                patch.Ex,
		}
		var resp updateUserResp
		if err := s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeUpdateUserInfo, req, &resp, &s.hooks.BeforeUpdateUserInfo); err != nil {
			return err
		}
		if err := applyUserCallback(patch, resp.Nickname, resp.FaceURL, resp.Ex); err != nil {
			return err
		}
	}
	if patch.Ex != nil && s.hooks.BeforeUpdateUserInfoEx.Enable {
		req := updateUserExReq{
			CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandBeforeUpdateUserInfoEx},
			UserID:            id,
			Nickname:          wrap(firstString(patch.Nickname, old.Nickname)),
			FaceURL:           wrap(firstString(patch.AvatarURL, old.AvatarURL)),
			Ex:                wrap(*patch.Ex),
		}
		var resp updateUserExResp
		if err := s.hooks.Client.SyncPost(ctx, webhook.CommandBeforeUpdateUserInfoEx, req, &resp, &s.hooks.BeforeUpdateUserInfoEx); err != nil {
			return err
		}
		if err := applyUserCallback(patch, unwrap(resp.Nickname), unwrap(resp.FaceURL), unwrap(resp.Ex)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) afterUpdateUser(ctx context.Context, id string, old *model.User, patch model.UserPatch) {
	if s.hooks.Client == nil {
		return
	}
	nickname := firstString(patch.Nickname, old.Nickname)
	face := firstString(patch.AvatarURL, old.AvatarURL)
	ex := firstString(patch.Ex, old.Ex)
	if patch.Nickname != nil || patch.AvatarURL != nil || patch.Ex != nil {
		req := afterUpdateUserReq{
			CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterUpdateUserInfo},
			UserID:            id,
			Nickname:          nickname,
			FaceURL:           face,
			Ex:                ex,
		}
		s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterUpdateUserInfo, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterUpdateUserInfo)
	}
	if patch.Ex != nil {
		req := updateUserExReq{
			CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterUpdateUserInfoEx},
			UserID:            id,
			Nickname:          wrap(nickname),
			FaceURL:           wrap(face),
			Ex:                wrap(ex),
		}
		s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterUpdateUserInfoEx, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterUpdateUserInfoEx)
	}
}

func applyUserCallback(patch *model.UserPatch, nickname, face, ex *string) error {
	if nickname != nil {
		value := *nickname
		if value == "" || utf8.RuneCountInString(value) > 255 {
			return errorcode.ErrUserInvalidUsername
		}
		patch.Nickname = &value
	}
	if face != nil {
		value := *face
		if len(value) > 1024 {
			return errorcode.ErrUserInvalidAvatar
		}
		patch.AvatarURL = &value
	}
	if ex != nil {
		value := *ex
		if len(value) > 1024 {
			return errorcode.ErrUserInvalidProfile.WithMsg("ex is too long")
		}
		patch.Ex = &value
	}
	return nil
}

func firstString(value *string, fallback string) string {
	if value != nil {
		return *value
	}
	return fallback
}

func wrap(value string) *wrappedString {
	return &wrappedString{Value: value}
}

func unwrap(value *wrappedString) *string {
	if value == nil {
		return nil
	}
	return &value.Value
}
