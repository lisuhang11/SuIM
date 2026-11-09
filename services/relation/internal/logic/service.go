package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/relationcache"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

// 投递一条通知。发送端为 nil 时不发通知。
type MsgSender interface {
	SendNotification(ctx context.Context, msg *msgpb.MsgData) error
}

// 确认用户 ID 都存在，并能读取公开资料。
type Users interface {
	CheckUsers(ctx context.Context, ids []string) error
	GetUsers(ctx context.Context, ids []string) (map[string]model.UserProfile, error)
}

// 关系 webhook 客户端和各回调开关。
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

// 查询群规则是否禁止这两人加好友。
type Groups interface {
	MemberFriendAllowed(ctx context.Context, user1, user2 string) (bool, error)
}

// 关系 gRPC 服务。好友、申请和黑名单都由调用者自己的会话操作。
type Server struct {
	relationpb.UnimplementedRelationServiceServer
	store  *store.Store
	users  Users
	groups Groups
	sender MsgSender
	hooks  Hooks
	lists  *relationcache.Lists
	now    func() time.Time
}

// 接入消息发送。传入 nil 时不发通知，只给进程内测试使用。
func (s *Server) UseSender(sender MsgSender) {
	if s != nil {
		s.sender = sender
	}
}

// 接入群规则。传入 nil 时不检查，只给进程内测试使用。
func (s *Server) UseGroups(groups Groups) {
	if s != nil {
		s.groups = groups
	}
}

// 接入 Redis 和进程内的好友缓存。传入 nil 时每次都读 MySQL，进程内测试就是这样。
func (s *Server) UseCache(lists *relationcache.Lists) {
	if s != nil {
		s.lists = lists
	}
}

// 用存储、用户查询和 webhook 开关构造关系服务。
func New(st *store.Store, users Users, hooks Hooks) *Server {
	return &Server{store: st, users: users, hooks: hooks, now: time.Now}
}
