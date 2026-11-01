package logic

import (
	"context"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/localcache"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

// Hooks carries the user webhook client and callback switches.
type Hooks struct {
	Client                 *webhook.Client
	BeforeUserRegister     webhook.BeforeConfig
	AfterUserRegister      webhook.AfterConfig
	BeforeUpdateUserInfo   webhook.BeforeConfig
	AfterUpdateUserInfo    webhook.AfterConfig
	BeforeUpdateUserInfoEx webhook.BeforeConfig
	AfterUpdateUserInfoEx  webhook.AfterConfig
}

type Server struct {
	userpb.UnimplementedUserServiceServer
	store     *store.Store
	issuer    *token.Issuer
	tokens    localcache.Cache[*model.Token]
	hooks     Hooks
	now       func() time.Time
	newID     func() (string, error)
	smsSender SMSSender
	captcha   CaptchaChecker
	policy    SMSPolicy
	pepper    string
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

// UseHooks attaches webhook switches. A nil client leaves every callback disabled.
func (s *Server) UseHooks(hooks Hooks) {
	if s != nil {
		s.hooks = hooks
	}
}

func (s *Server) invalidate(ctx context.Context, hashes []string) {
	if s.tokens == nil || len(hashes) == 0 {
		return
	}
	s.tokens.Del(ctx, hashes...)
}

// toPublic is the card returned to anyone other than the profile owner.
func toPublic(u *model.User) *userpb.UserInfo {
	return &userpb.UserInfo{
		UserId:         u.UserID,
		Nickname:       u.Nickname,
		AvatarUrl:      u.AvatarURL,
		Gender:         u.Gender,
		AllowAddFriend: u.AllowAddFriend,
	}
}

func projectUser(u *model.User, viewerID string) *userpb.UserInfo {
	if viewerID != "" && viewerID == u.UserID {
		return toPB(u)
	}
	return toPublic(u)
}

// actorID is the session user for a mutating RPC. An empty requested ID means
// the token user. A different ID is rejected before any read or write.
func (s *Server) actorID(ctx context.Context, raw, requested string) (string, error) {
	claims, _, err := s.loadUsable(ctx, strings.TrimSpace(raw), model.TokenType)
	if err != nil {
		return "", err
	}
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return claims.UserID, nil
	}
	id, err := normalizeUserID(requested)
	if err != nil {
		return "", err
	}
	if id != claims.UserID {
		return "", errorcode.ErrNoPermission
	}
	return id, nil
}

func (s *Server) viewerID(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	claims, _, err := s.loadUsable(ctx, raw, model.TokenType)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
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
		HasPassword:      u.HasPassword,
	}
}
