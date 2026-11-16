package logic

import (
	"context"
	"log/slog"

	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/prommetrics"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"google.golang.org/protobuf/proto"
)

// HandlePush 消费一条 toPush 记录：在线投递由网关负责，这里只补发离线通知。
// 处理失败也确认消息，避免一条通知卡住整条队列。
func (s *Server) HandlePush(ctx context.Context, msg *mq.Message) error {
	body := &msgpb.PushMsgData{}
	if err := proto.Unmarshal(msg.Value, body); err != nil {
		slog.Error("unmarshal toPush message", "err", err, "key", msg.Key)
		return msg.Ack(ctx)
	}
	if err := s.dispatch(ctx, body); err != nil {
		prommetrics.MsgOfflinePushFailed.Inc()
		slog.Warn("offline push failed", "err", err, "key", msg.Key)
	}
	return msg.Ack(ctx)
}
