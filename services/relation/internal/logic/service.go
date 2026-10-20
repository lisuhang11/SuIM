package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

// Users confirms that every user ID exists.
type Users interface {
	CheckUsers(ctx context.Context, ids []string) error
}

// Hooks carries the webhook client and the black-list callback switches.
type Hooks struct {
	Client           *webhook.Client
	BeforeAddBlack   webhook.BeforeConfig
	AfterRemoveBlack webhook.AfterConfig
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
