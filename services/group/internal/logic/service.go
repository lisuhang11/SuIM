package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/proto/group"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

// Users loads public profiles. A missing ID is absent from the map.
type Users interface {
	GetUsers(ctx context.Context, ids []string) (map[string]model.UserProfile, error)
}

// MsgSender publishes one notification. msgtransfer assigns the seq and msggateway pushes it.
type MsgSender interface {
	SendNotification(ctx context.Context, msg *msgpb.MsgData) error
}

// Hooks carries the group webhook client and callback switches.
type Hooks struct {
	Client                   *webhook.Client
	BeforeCreateGroup        webhook.BeforeConfig
	AfterCreateGroup         webhook.AfterConfig
	BeforeInviteJoinGroup    webhook.BeforeConfig
	BeforeJoinGroup          webhook.BeforeConfig
	BeforeMembersJoinGroup   webhook.BeforeConfig
	AfterJoinGroup           webhook.AfterConfig
	AfterQuitGroup           webhook.AfterConfig
	AfterKickGroup           webhook.AfterConfig
	AfterDismissGroup        webhook.AfterConfig
	AfterTransferGroupOwner  webhook.AfterConfig
	BeforeSetGroupInfo       webhook.BeforeConfig
	AfterSetGroupInfo        webhook.AfterConfig
	BeforeSetGroupInfoEx     webhook.BeforeConfig
	AfterSetGroupInfoEx      webhook.AfterConfig
	BeforeSetGroupMemberInfo webhook.BeforeConfig
	AfterSetGroupMemberInfo  webhook.AfterConfig
}

type Server struct {
	grouppb.UnimplementedGroupServiceServer
	store  *store.Store
	users  Users
	sender MsgSender
	hooks  Hooks
	now    func() time.Time
}

func New(st *store.Store, users Users, sender MsgSender) *Server {
	return &Server{store: st, users: users, sender: sender, now: time.Now}
}

// UseHooks attaches webhook switches. A nil client leaves every callback disabled.
func (s *Server) UseHooks(hooks Hooks) {
	if s != nil {
		s.hooks = hooks
	}
}
