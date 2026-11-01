package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

// Users confirms that every user ID exists and can load public profiles.
type Users interface {
	CheckUsers(ctx context.Context, ids []string) error
	GetUsers(ctx context.Context, ids []string) (map[string]model.UserProfile, error)
}

// Hooks carries the webhook client and the callback switches.
type Hooks struct {
	Client                *webhook.Client
	BeforeAddFriend       webhook.BeforeConfig
	AfterAddFriend        webhook.AfterConfig
	BeforeAddFriendAgree  webhook.BeforeConfig
	AfterAddFriendAgree   webhook.AfterConfig
	BeforeSetFriendRemark webhook.BeforeConfig
	AfterSetFriendRemark  webhook.AfterConfig
	AfterDeleteFriend     webhook.AfterConfig
	BeforeAddBlack        webhook.BeforeConfig
	AfterRemoveBlack      webhook.AfterConfig
}

type Server struct {
	relationpb.UnimplementedRelationServiceServer
	store *store.Store
	users Users
	hooks Hooks
	now   func() time.Time
}

func New(st *store.Store, users Users, hooks Hooks) *Server {
	return &Server{store: st, users: users, hooks: hooks, now: time.Now}
}
