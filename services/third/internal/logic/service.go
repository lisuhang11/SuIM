package logic

import (
	"context"
	"time"

	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

const (
	defaultBatch = 20
	maxSignBatch = 100
)

// DefaultLimits match MinIO multipart bounds: 5 MiB, 5 GiB, 10000 parts.
var DefaultLimits = Limits{
	MinPartSize: 5 << 20,
	MaxPartSize: 5 << 30,
	MaxNumSize:  10000,
}

// Limits controls how a file is sliced.
type Limits struct {
	MinPartSize int64
	MaxPartSize int64
	MaxNumSize  int64
}

// SessionStore keeps an unfinished upload until it expires.
type SessionStore interface {
	CreateSession(ctx context.Context, sess *model.Session) error
	GetSession(ctx context.Context, id string) (*model.Session, error)
	DeleteSession(ctx context.Context, id string) error
}

// ObjectStore records finished names. One hash may have many names.
type ObjectStore interface {
	FindByHash(ctx context.Context, hash string) (*model.Object, error)
	FindByName(ctx context.Context, name string) (*model.Object, error)
	InsertObject(ctx context.Context, obj *model.Object) error
}

// Parts talks to object storage. Part bytes are uploaded by the client.
type Parts interface {
	Create(ctx context.Context, key, contentType string) (uploadID string, err error)
	Sign(ctx context.Context, key, uploadID string, part int, expire time.Duration) (string, error)
	List(ctx context.Context, key, uploadID string) ([]model.StoredPart, error)
	Finish(ctx context.Context, key, uploadID string, parts []model.StoredPart) error
	Abort(ctx context.Context, key, uploadID string) error
	Stat(ctx context.Context, key string) (int64, error)
}

// Options configures the slice plan and how long a session stays open.
type Options struct {
	Limits Limits
	Expire time.Duration
	Now    func() time.Time
}

// Server implements the multipart upload RPCs. Clients PUT part bytes to MinIO.
type Server struct {
	thirdpb.UnimplementedThirdServiceServer
	sessions SessionStore
	objects  ObjectStore
	parts    Parts
	limits   Limits
	expire   time.Duration
	now      func() time.Time
}

func New(sessions SessionStore, objects ObjectStore, parts Parts, opts Options) *Server {
	limits := opts.Limits
	if limits.MinPartSize == 0 && limits.MaxPartSize == 0 && limits.MaxNumSize == 0 {
		limits = DefaultLimits
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	expire := opts.Expire
	if expire <= 0 {
		expire = 24 * time.Hour
	}
	return &Server{
		sessions: sessions,
		objects:  objects,
		parts:    parts,
		limits:   limits,
		expire:   expire,
		now:      now,
	}
}
