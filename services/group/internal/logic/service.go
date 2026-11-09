package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

// 读取公开资料。结果里没有的 ID 表示这个用户不存在。
type Users interface {
	GetUsers(ctx context.Context, ids []string) (map[string]model.UserProfile, error)
}

// 投递一条通知。序号由 msgtransfer 分配，长连接由 msggateway 推送。
type MsgSender interface {
	SendNotification(ctx context.Context, msg *msgpb.MsgData) error
}

// 群 webhook 客户端和各回调开关。
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

// 群 gRPC 服务。
type Server struct {
	grouppb.UnimplementedGroupServiceServer
	store        *store.Store
	users        Users
	sender       MsgSender
	hooks        Hooks
	quota        Quota
	seq          SeqClient
	convs        GroupConversations
	memberNormal int
	memberSuper  int
	now          func() time.Time
}

// 用存储、用户查询和通知发送构造群服务。
func New(st *store.Store, users Users, sender MsgSender) *Server {
	return &Server{store: st, users: users, sender: sender, memberNormal: 200, memberSuper: 1000, now: time.Now}
}

// 设置普通群和超大群的人数上限，含群主。超大群上限不能小于普通群。
func (s *Server) UseMemberLimit(normal, super int) {
	if s != nil && normal > 0 && super >= normal {
		s.memberNormal = normal
		s.memberSuper = super
	}
}

func (s *Server) memberCap(groupType int32) int {
	if groupType == model.GroupTypeSuper {
		return s.memberSuper
	}
	return s.memberNormal
}

// 接入消息服务的序号读写。
func (s *Server) UseSeq(seq SeqClient) {
	if s != nil {
		s.seq = seq
	}
}

// 接入会话服务。
func (s *Server) UseConversations(convs GroupConversations) {
	if s != nil {
		s.convs = convs
	}
}

// 接入 Redis 计数。传入 nil 时这三项不限次数。
func (s *Server) UseQuota(q Quota) {
	if s != nil {
		s.quota = q
	}
}

// 接入 webhook 开关。客户端为 nil 时所有回调都不执行。
func (s *Server) UseHooks(hooks Hooks) {
	if s != nil {
		s.hooks = hooks
	}
}
