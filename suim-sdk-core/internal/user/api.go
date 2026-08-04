package user

import (
	"context"

	"SuIM/suim-sdk-core/sdk_struct"
)

// GetSelfUserInfo obtains the current login user (memory → local → HTTP), OpenIM-compatible.
func (u *User) GetSelfUserInfo(ctx context.Context) (*sdk_struct.LocalUser, error) {
	return u.GetUserInfoWithCache(ctx, u.loginUserID)
}

// SetSelfInfo updates self profile then syncs local cache (OpenIM-compatible).
func (u *User) SetSelfInfo(ctx context.Context, userInfo *sdk_struct.UserInfoUpdate) error {
	if err := u.updateUserInfo(ctx, userInfo); err != nil {
		return err
	}
	_ = u.SyncLoginUserInfo(ctx)
	return nil
}

// GetUsersInfo batch-fetches public user info (memory → HTTP).
func (u *User) GetUsersInfo(ctx context.Context, userIDs []string) ([]*sdk_struct.PublicUser, error) {
	usersInfo, err := u.GetUsersInfoWithCache(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	res := make([]*sdk_struct.PublicUser, 0, len(usersInfo))
	for _, info := range usersInfo {
		res = append(res, LocalUserToPublicUser(info))
	}
	return res, nil
}
