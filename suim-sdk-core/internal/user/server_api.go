package user

import (
	"context"

	"SuIM/suim-sdk-core/pkg/api"
	"SuIM/suim-sdk-core/sdk_struct"
)

func (u *User) getUsersInfo(ctx context.Context, userIDs []string) ([]*sdk_struct.ServerUser, error) {
	return api.GetUsersByIDs(ctx, userIDs)
}

func (u *User) getSelfFromServer(ctx context.Context) (*sdk_struct.ServerUser, error) {
	return api.GetSelfUser(ctx)
}

func (u *User) updateUserInfo(ctx context.Context, info *sdk_struct.UserInfoUpdate) error {
	var nickname, avatar *string
	if info != nil {
		nickname = info.Nickname
		if info.FaceURL != nil {
			avatar = info.FaceURL
		} else if info.AvatarURL != nil {
			avatar = info.AvatarURL
		}
	}
	return api.UpdateSelfUser(ctx, nickname, avatar)
}
