package api

import (
	"context"
	"net/url"
	"strings"

	"SuIM/suim-sdk-core/pkg/network"
	"SuIM/suim-sdk-core/sdk_struct"
)

// SuIM REST paths under /api/v1 (caller should set ApiAddr including /api/v1 or we include it).
// Convention: ApiAddr = http://host:9000/api/v1

type getUserResp struct {
	User *sdk_struct.ServerUser `json:"user"`
}

type getUsersByIDsResp struct {
	Users map[string]*sdk_struct.ServerUser `json:"users"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResp struct {
	User         *sdk_struct.ServerUser `json:"user"`
	AccessToken  string                 `json:"access_token"`
	RefreshToken string                 `json:"refresh_token"`
}

type updateUserBody struct {
	Nickname  *string `json:"nickname,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

func GetSelfUser(ctx context.Context) (*sdk_struct.ServerUser, error) {
	var resp getUserResp
	if err := network.ApiGet(ctx, "/users/me", &resp); err != nil {
		return nil, err
	}
	return resp.User, nil
}

func GetUsersByIDs(ctx context.Context, userIDs []string) ([]*sdk_struct.ServerUser, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	q := url.Values{}
	q.Set("ids", strings.Join(userIDs, ","))
	var resp getUsersByIDsResp
	if err := network.ApiGet(ctx, network.JoinQuery("/users/batch", q), &resp); err != nil {
		return nil, err
	}
	out := make([]*sdk_struct.ServerUser, 0, len(resp.Users))
	for _, u := range resp.Users {
		if u != nil {
			out = append(out, u)
		}
	}
	return out, nil
}

func UpdateSelfUser(ctx context.Context, nickname, avatarURL *string) error {
	body := updateUserBody{Nickname: nickname, AvatarURL: avatarURL}
	return network.ApiPut(ctx, "/users/me", body, nil)
}

func Login(ctx context.Context, email, password string) (*loginResp, error) {
	var resp loginResp
	if err := network.ApiPost(ctx, "/users/login", &loginReq{Email: email, Password: password}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
