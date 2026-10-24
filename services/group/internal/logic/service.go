package logic

import (
	"context"
	"time"

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

type Server struct {
	grouppb.UnimplementedGroupServiceServer
	store  *store.Store
	users  Users
	sender MsgSender
	now    func() time.Time
}

func New(st *store.Store, users Users, sender MsgSender) *Server {
	return &Server{store: st, users: users, sender: sender, now: time.Now}
}
