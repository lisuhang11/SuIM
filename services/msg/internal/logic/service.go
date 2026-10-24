package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/lisuhang11/SuIM/pkg/mq"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msg/internal/store"
)

// JoinedGroup is a group the user still belongs to.
type JoinedGroup struct {
	ID    string
	Super bool
}

// Groups lists the groups a user has joined. A nil directory skips group conversations.
type Groups interface {
	Joined(ctx context.Context, userID string) ([]JoinedGroup, error)
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
	Range(ctx context.Context, conversationID string, begin, end, limit int64) ([]*msgpb.MsgData, error)
	BySeqs(ctx context.Context, conversationID string, seqs []int64) ([]*msgpb.MsgData, error)
	Last(ctx context.Context, conversationID string) (*msgpb.MsgData, error)
}

// Server is the msg gRPC service. It publishes sends and reads history.
type Server struct {
	msgpb.UnimplementedMsgServiceServer
	store     History
	publish   mq.Producer
	relations Relations
	groups    Groups
}

func New(st History, publish mq.Producer, relations Relations, groups Groups) *Server {
	return &Server{store: st, publish: publish, relations: relations, groups: groups}
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
