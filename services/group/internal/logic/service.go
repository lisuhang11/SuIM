package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

// Users loads public profiles. A missing ID is absent from the map.
type Users interface {
	GetUsers(ctx context.Context, ids []string) (map[string]model.UserProfile, error)
}

type Server struct {
	grouppb.UnimplementedGroupServiceServer
	store *store.Store
	users Users
	now   func() time.Time
}

func New(st *store.Store, users Users) *Server {
	return &Server{store: st, users: users, now: time.Now}
}
