package ws

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/mq"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// HandlePush delivers one toPush record to the users connected to this process.
// An offline recipient is acked here and catches up later with GetMaxSeq.
func (s *Server) HandlePush(ctx context.Context, msg *mq.Message) error {
	body := &msgpb.PushMsgData{}
	if err := proto.Unmarshal(msg.Value, body); err != nil {
		slog.Error("unmarshal toPush message", "err", err, "key", msg.Key)
		return msg.Ack(ctx)
	}
	s.Deliver(ctx, body.GetMsgData(), body.GetConversationId())
	return msg.Ack(ctx)
}
