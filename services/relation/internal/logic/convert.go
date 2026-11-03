package logic

import (
	"time"

	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

func publicUser(id string, users map[string]model.UserProfile) *relationpb.PublicUserInfo {
	info := &relationpb.PublicUserInfo{UserId: id}
	if user, ok := users[id]; ok {
		info.Nickname = user.Nickname
		info.AvatarUrl = user.AvatarURL
	}
	return info
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
