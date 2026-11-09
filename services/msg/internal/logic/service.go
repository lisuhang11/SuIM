package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msg/internal/store"
)

// 用户仍在其中的一个群。
type JoinedGroup struct {
	ID    string
	Super bool
}

// 发群消息时这名成员在群里的位置。Found 为假表示没有这条成员记录。
type GroupSpeaker struct {
	Found     bool
	InGroup   bool
	Status    int32
	Role      int32
	MuteEndMs int64
}

// 列出已加入的群，并读取一名成员供禁言判断。
type Groups interface {
	Joined(ctx context.Context, userID string) ([]JoinedGroup, error)
	Speaker(ctx context.Context, groupID, userID string) (GroupSpeaker, error)
}

// 读取用户置顶的会话。会话表属于 conversation 服务。
type Pinned interface {
	PinnedConversationIDs(ctx context.Context, userID string) ([]string, error)
}

// 一个用户对一个单聊的接收设置。Found 为假表示没有这一行，此时仍然接收。
type RecvSetting struct {
	Found bool
	Opt   int32
}

// 读取接收方自己的单聊接收设置。
type SingleRecv interface {
	RecvOpt(ctx context.Context, ownerUserID, conversationID string) (RecvSetting, error)
}

// 单聊受理前检查好友和拉黑。
type Relations interface {
	IsFriend(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
	IsBlack(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
}

// 读取已经编过号的消息。写入由 msgtransfer 完成。
type History interface {
	ConversationIDs(ctx context.Context, userID string) ([]string, error)
	MaxSeqs(ctx context.Context, conversationIDs []string) (map[string]int64, error)
	SeqState(ctx context.Context, userID string, conversationIDs []string) (map[string]store.SeqState, error)
	SeqBounds(ctx context.Context, userID, conversationID string) (minSeq, maxSeq int64, err error)
	UserMaxSeq(ctx context.Context, userID, conversationID string) (int64, error)
	SetUserMaxSeq(ctx context.Context, conversationID, userID string, seq int64) error
	SetUserMinSeq(ctx context.Context, conversationID, userID string, seq int64) error
	Range(ctx context.Context, conversationID string, begin, end, limit int64) ([]*msgpb.MsgData, error)
	BySeqs(ctx context.Context, conversationID string, seqs []int64) ([]*msgpb.MsgData, error)
	Last(ctx context.Context, conversationID string) (*msgpb.MsgData, error)
	SetReadSeq(ctx context.Context, userID, conversationID string, seq int64) error
	MarkRead(ctx context.Context, conversationID, readerID string, seqs []int64) error
}

// 发送前的同步回调，以及跳过这次回调的内容类型。
type MsgBefore struct {
	webhook.BeforeConfig `yaml:",inline"`
	DeniedTypes          []int32 `yaml:"deniedTypes"`
}

// 发送后的异步回调。AttentionIds 限定哪些用户会触发它。
type MsgAfter struct {
	webhook.AfterConfig `yaml:",inline"`
	AttentionIds        []string `yaml:"attentionIds"`
	DeniedTypes         []int32  `yaml:"deniedTypes"`
}

// 消息 webhook 客户端和各回调开关。
type Hooks struct {
	Client           *webhook.Client
	BeforeSendSingle MsgBefore
	AfterSendSingle  MsgAfter
	BeforeSendGroup  MsgBefore
	AfterSendGroup   MsgAfter
	BeforeMsgModify  MsgBefore
	AfterSingleRead  MsgAfter
	AfterGroupRead   MsgAfter
}

// 消息 gRPC 服务。负责受理发送和读取历史。
type Server struct {
	msgpb.UnimplementedMsgServiceServer
	store     History
	publish   mq.Producer
	relations Relations
	groups    Groups
	pinned    Pinned
	recv      SingleRecv
	hooks     Hooks
}

// 用历史存储、队列和关系、群依赖构造消息服务。置顶和接收设置稍后注入。
func New(st History, publish mq.Producer, relations Relations, groups Groups) *Server {
	return &Server{store: st, publish: publish, relations: relations, groups: groups}
}

// 接入会话服务里的置顶列表。
func (s *Server) UsePinned(pinned Pinned) {
	if s != nil {
		s.pinned = pinned
	}
}

// 接入接收方的单聊接收设置。传入 nil 时不检查免打扰。
func (s *Server) UseRecv(recv SingleRecv) {
	if s != nil {
		s.recv = recv
	}
}

// 接入 webhook 开关。客户端为 nil 时所有回调都不执行。
func (s *Server) UseHooks(hooks Hooks) {
	if s != nil {
		s.hooks = hooks
	}
}

func (s *Server) conversationIDs(ctx context.Context, userID string) ([]string, error) {
	seen := map[string]struct{}{}
	var ids []string
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if s.store != nil {
		found, err := s.store.ConversationIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, id := range found {
			add(id)
		}
	}
	if s.groups != nil {
		joined, err := s.groups.Joined(ctx, userID)
		if err != nil {
			slog.Warn("joined groups", "err", err, "userID", userID)
		} else {
			for _, group := range joined {
				add(groupConversationID(group))
			}
		}
	}
	return ids, nil
}

func newClientMsgID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
