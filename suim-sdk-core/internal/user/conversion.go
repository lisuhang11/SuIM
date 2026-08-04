package user

import (
	"SuIM/suim-sdk-core/sdk_struct"
)

func ServerUserToLocalUser(u *sdk_struct.ServerUser) *sdk_struct.LocalUser {
	if u == nil {
		return nil
	}
	return &sdk_struct.LocalUser{
		UserID:           u.UserID,
		Nickname:         u.Nickname,
		Email:            u.Email,
		FaceURL:          u.AvatarURL,
		Ex:               u.Ex,
		GlobalRecvMsgOpt: u.GlobalRecvMsgOpt,
		CreateTime:       u.CreateTime,
		UpdatedAt:        u.UpdatedAt,
	}
}

func LocalUserToPublicUser(u *sdk_struct.LocalUser) *sdk_struct.PublicUser {
	if u == nil {
		return nil
	}
	return &sdk_struct.PublicUser{
		UserID:     u.UserID,
		Nickname:   u.Nickname,
		FaceURL:    u.FaceURL,
		Ex:         u.Ex,
		CreateTime: u.CreateTime,
	}
}
