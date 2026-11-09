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

// 默认分片边界与 MinIO 一致：最小 5 MiB，最大 5 GiB，最多 10000 片。
var DefaultLimits = Limits{
	MinPartSize: 5 << 20,
	MaxPartSize: 5 << 30,
	MaxNumSize:  10000,
}

// 控制一个文件怎样切片。
type Limits struct {
	MinPartSize int64
	MaxPartSize int64
	MaxNumSize  int64
}

// 保存未完成的上传，直到过期。
type SessionStore interface {
	CreateSession(ctx context.Context, sess *model.Session) error
	GetSession(ctx context.Context, id string) (*model.Session, error)
	DeleteSession(ctx context.Context, id string) error
}

// 登记已经完成的对象名。同一个哈希可以有多个名字。
type ObjectStore interface {
	FindByHash(ctx context.Context, hash string) (*model.Object, error)
	FindByName(ctx context.Context, name string) (*model.Object, error)
	InsertObject(ctx context.Context, obj *model.Object) error
}

// 访问对象存储。分片字节由客户端自己上传。
type Parts interface {
	Create(ctx context.Context, key, contentType string) (uploadID string, err error)
	Sign(ctx context.Context, key, uploadID string, part int, expire time.Duration) (string, error)
	List(ctx context.Context, key, uploadID string) ([]model.StoredPart, error)
	Finish(ctx context.Context, key, uploadID string, parts []model.StoredPart) error
	Abort(ctx context.Context, key, uploadID string) error
	Stat(ctx context.Context, key string) (int64, error)
	Head(ctx context.Context, key string) (size int64, etag string, err error)
	Form(ctx context.Context, key string, size int64, contentType string, expires time.Time) (*model.FormCredential, error)
}

// 分片计划和未完成上传的保留时间。
type Options struct {
	Limits Limits
	Expire time.Duration
	// 给表单上传的 ID 签名。
	Secret string
	Now    func() time.Time
}

// 分片上传 gRPC 服务。客户端把分片字节直接交给 MinIO。
type Server struct {
	thirdpb.UnimplementedThirdServiceServer
	sessions SessionStore
	objects  ObjectStore
	parts    Parts
	limits   Limits
	secret   []byte
	expire   time.Duration
	now      func() time.Time
}

// 用会话、对象记录和对象存储构造上传服务。
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
		secret:   []byte(opts.Secret),
		expire:   expire,
		now:      now,
	}
}
