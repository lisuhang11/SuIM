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

// JoinedGroup is a group the user still belongs to.
type JoinedGroup struct {
	ID    string
	Super bool
}

// GroupSpeaker is one member's place in a group, used when a group message is sent.
type GroupSpeaker struct {
	Found     bool
	InGroup   bool
	Status    int32
	Role      int32
	MuteEndMs int64
}

// Groups lists joined groups and reads one member for the mute check.
type Groups interface {
	Joined(ctx context.Context, userID string) ([]JoinedGroup, error)
	Speaker(ctx context.Context, groupID, userID string) (GroupSpeaker, error)
}

// Relations checks friendship and blocks before a single chat is accepted.
type Relations interface {
	IsFriend(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
	IsBlack(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
}

// History reads sequenced messages. msgtransfer writes them.
type History interface {
	ConversationIDs(ctx context.Context, userID string) ([]string, error)
	MaxSeqs(ctx context.Context, conversationIDs []string) (map[string]int64, error)
	SeqState(ctx context.Context, userID string, conversationIDs []string) (map[string]store.SeqState, error)
	SeqBounds(ctx context.Context, userID, conversationID string) (minSeq, maxSeq int64, err error)
	PinnedConversationIDs(ctx context.Context, userID string) ([]string, error)
	Range(ctx context.Context, conversationID string, begin, end, limit int64) ([]*msgpb.MsgData, error)
	BySeqs(ctx context.Context, conversationID string, seqs []int64) ([]*msgpb.MsgData, error)
	Last(ctx context.Context, conversationID string) (*msgpb.MsgData, error)
	SetReadSeq(ctx context.Context, userID, conversationID string, seq int64) error
	MarkRead(ctx context.Context, conversationID, readerID string, seqs []int64) error
}

// MsgBefore is a synchronous message callback plus content types that skip it.
type MsgBefore struct {
	webhook.BeforeConfig `yaml:",inline"`
	DeniedTypes          []int32 `yaml:"deniedTypes"`
}

// MsgAfter is an asynchronous message callback. AttentionIds limits who it fires for.
type MsgAfter struct {
	webhook.AfterConfig `yaml:",inline"`
	AttentionIds        []string `yaml:"attentionIds"`
	DeniedTypes         []int32  `yaml:"deniedTypes"`
}

// Hooks carries the message webhook client and callback switches.
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

// Server is the msg gRPC service. It publishes sends and reads history.
type Server struct {
	msgpb.UnimplementedMsgServiceServer
	store     History
	publish   mq.Producer
	relations Relations
	groups    Groups
	hooks     Hooks
}

func New(st History, publish mq.Producer, relations Relations, groups Groups) *Server {
	return &Server{store: st, publish: publish, relations: relations, groups: groups}
}

// UseHooks attaches webhook switches. A nil client leaves every callback disabled.
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
