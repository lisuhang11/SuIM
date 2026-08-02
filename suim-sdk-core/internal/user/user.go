package user

import (
	"context"
	"encoding/json"
	"sync"

	"SuIM/suim-sdk-core/pkg/cache"
	"SuIM/suim-sdk-core/pkg/db"
	"SuIM/suim-sdk-core/pkg/sdkerrs"
	"SuIM/suim-sdk-core/sdk_struct"
	"SuIM/suim-sdk-core/suim_sdk_callback"
)

// User is the user domain module (OpenIM-style).
type User struct {
	db          *db.DataBase
	loginUserID string
	listener    func() suim_sdk_callback.OnUserListener
	userCache   *cache.UserCache[string, *sdk_struct.LocalUser]
	once        sync.Once
}

func NewUser() *User {
	return &User{}
}

func (u *User) SetDataBase(database *db.DataBase) { u.db = database }
func (u *User) SetLoginUserID(id string)           { u.loginUserID = id }
func (u *User) SetListener(listener func() suim_sdk_callback.OnUserListener) {
	u.listener = listener
}

func (u *User) UserCache() *cache.UserCache[string, *sdk_struct.LocalUser] {
	u.once.Do(func() {
		u.userCache = cache.NewUserCache(
			func(value *sdk_struct.LocalUser) string { return value.UserID },
			u.getLoginUser,
			u.GetUsersInfoFromServer,
		)
	})
	return u.userCache
}

func (u *User) getLoginUser(ctx context.Context, userID string) (*sdk_struct.LocalUser, error) {
	if u.db == nil {
		return nil, sdkerrs.ErrRecordNotFound
	}
	return u.db.GetLoginUser(ctx, userID)
}

func (u *User) GetUserInfoWithCache(ctx context.Context, cacheKey string) (*sdk_struct.LocalUser, error) {
	return u.UserCache().Fetch(ctx, cacheKey)
}

func (u *User) GetUsersInfoWithCache(ctx context.Context, cacheKeys []string) ([]*sdk_struct.LocalUser, error) {
	m, err := u.UserCache().BatchFetch(ctx, cacheKeys)
	if err != nil {
		return nil, err
	}
	out := make([]*sdk_struct.LocalUser, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out, nil
}

func (u *User) GetUsersInfoFromServer(ctx context.Context, userIDs []string) ([]*sdk_struct.LocalUser, error) {
	// Prefer /users/me when requesting only self — batch still works but me is canonical.
	if len(userIDs) == 1 && userIDs[0] == u.loginUserID {
		su, err := u.getSelfFromServer(ctx)
		if err != nil {
			return nil, err
		}
		if su == nil {
			return nil, sdkerrs.ErrUserNotFound
		}
		return []*sdk_struct.LocalUser{ServerUserToLocalUser(su)}, nil
	}
	serverUsers, err := u.getUsersInfo(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	if len(serverUsers) == 0 {
		return nil, sdkerrs.ErrUserNotFound
	}
	out := make([]*sdk_struct.LocalUser, 0, len(serverUsers))
	for _, su := range serverUsers {
		out = append(out, ServerUserToLocalUser(su))
	}
	return out, nil
}

func (u *User) GetSingleUserFromServer(ctx context.Context, userID string) (*sdk_struct.LocalUser, error) {
	list, err := u.GetUsersInfoFromServer(ctx, []string{userID})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, sdkerrs.ErrUserNotFound
	}
	return list[0], nil
}

func (u *User) SyncLoginUserInfo(ctx context.Context) error {
	remote, err := u.GetSingleUserFromServer(ctx, u.loginUserID)
	if err != nil {
		return err
	}
	local, err := u.getLoginUser(ctx, u.loginUserID)
	if u.userCache != nil {
		u.UserCache().Delete(u.loginUserID)
	}
	if err != nil {
		if err := u.db.InsertLoginUser(ctx, remote); err != nil {
			return err
		}
	} else {
		changed := local.Nickname != remote.Nickname || local.FaceURL != remote.FaceURL ||
			local.Ex != remote.Ex || local.Email != remote.Email
		if err := u.db.UpdateLoginUser(ctx, remote); err != nil {
			return err
		}
		if changed && u.listener != nil && u.listener() != nil {
			b, _ := json.Marshal(remote)
			u.listener().OnSelfInfoUpdated(string(b))
		}
	}
	u.UserCache().Store(remote.UserID, remote)
	return nil
}
