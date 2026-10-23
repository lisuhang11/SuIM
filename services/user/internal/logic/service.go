package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/localcache"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

type Server struct {
	userpb.UnimplementedUserServiceServer
	store  *store.Store
	issuer *token.Issuer
	tokens localcache.Cache[*model.Token]
	now    func() time.Time
	newID  func() (string, error)
}

func New(st *store.Store, issuer *token.Issuer, tokens localcache.Cache[*model.Token]) *Server {
	return &Server{
		store:  st,
		issuer: issuer,
		tokens: tokens,
		now:    time.Now,
		newID:  NewID,
	}
}

func (s *Server) invalidate(ctx context.Context, hashes []string) {
	if s.tokens == nil || len(hashes) == 0 {
		return
	}
	s.tokens.Del(ctx, hashes...)
}

func toPB(u *model.User) *userpb.UserInfo {
	var birth int64
	if !u.BirthTime.IsZero() {
		birth = u.BirthTime.UnixMilli()
	}
	return &userpb.UserInfo{
		UserId:           u.UserID,
		Email:            u.Email,
		Nickname:         u.Nickname,
		AvatarUrl:        u.AvatarURL,
		AppMangerLevel:   u.AppMangerLevel,
		GlobalRecvMsgOpt: u.GlobalRecvMsgOpt,
		IsActive:         u.IsActive,
		CreateTime:       u.CreateTime.UnixMilli(),
		AreaCode:         u.AreaCode,
		PhoneNumber:      u.PhoneNumber,
		Account:          u.Account,
		Gender:           u.Gender,
		BirthTime:        birth,
		Level:            u.Level,
		AllowVibration:   u.AllowVibration,
		AllowBeep:        u.AllowBeep,
		AllowAddFriend:   u.AllowAddFriend,
		RegisterType:     u.RegisterType,
		Ex:               u.Ex,
	}
}
