package logic

import (
	"context"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/localcache"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

// 用户 webhook 客户端和各回调开关。
type Hooks struct {
	Client                 *webhook.Client
	BeforeUserRegister     webhook.BeforeConfig
	AfterUserRegister      webhook.AfterConfig
	BeforeUpdateUserInfo   webhook.BeforeConfig
	AfterUpdateUserInfo    webhook.AfterConfig
	BeforeUpdateUserInfoEx webhook.BeforeConfig
	AfterUpdateUserInfoEx  webhook.AfterConfig
}

// 用户 gRPC 服务。资料修改和删除都由令牌里的用户发起。
type Server struct {
	userpb.UnimplementedUserServiceServer
	store     *store.Store
	issuer    *token.Issuer
	tokens    localcache.Cache[*model.Token]
	profiles  *Profiles
	hooks     Hooks
	now       func() time.Time
	newID     func() (string, error)
	smsSender SMSSender
	captcha   CaptchaChecker
	policy    SMSPolicy
	pepper    string
}

// 用存储、令牌签发和令牌缓存构造用户服务。
func New(st *store.Store, issuer *token.Issuer, tokens localcache.Cache[*model.Token]) *Server {
	return &Server{
		store:  st,
		issuer: issuer,
		tokens: tokens,
		now:    time.Now,
		newID:  NewID,
	}
}

// 打开用户行缓存。传入 nil 时每次都读 MySQL。
func (s *Server) UseProfiles(profiles *Profiles) {
	if s != nil {
		s.profiles = profiles
	}
}

func (s *Server) userByID(ctx context.Context, id string) (*model.User, error) {
	if s.profiles == nil {
		return s.store.GetUserByID(ctx, id)
	}
	return s.profiles.Get(ctx, id)
}

func (s *Server) dropProfile(ctx context.Context, userID string) {
	if s.profiles != nil {
		s.profiles.Drop(ctx, userID)
	}
}

// 接入短信发送、人机校验、频率策略和验证码哈希盐。
func (s *Server) UseSMS(sender SMSSender, captcha CaptchaChecker, policy SMSPolicy, pepper string) {
	if s == nil {
		return
	}
	s.smsSender = sender
	s.captcha = captcha
	s.policy = policy
	s.pepper = pepper
}

// 接入 webhook 开关。客户端为 nil 时所有回调都不执行。
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

// 返回给资料主人以外的人的名片，不含账号和手机号。
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

// 修改类接口使用的会话用户。请求里的用户 ID 为空时就是令牌用户，填了别人则在读写之前拒绝。
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
