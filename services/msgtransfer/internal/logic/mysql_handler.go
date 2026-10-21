package logic

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/config"
)

// MySQLHandler consumes toMysql and writes messages into MySQL.
type MySQLHandler struct {
	db      MsgDB
	webhook *webhook.Client
	after   config.AfterMsgSaveDB
}

func NewMySQLHandler(db MsgDB, client *webhook.Client, after config.AfterMsgSaveDB) *MySQLHandler {
	return &MySQLHandler{db: db, webhook: client, after: after}
}

// Handle inserts one toMysql batch and acks it after MySQL accepts the rows.
func (h *MySQLHandler) Handle(ctx context.Context, msg *mq.Message) error {
	body := &msgpb.MsgDataToMySQL{}
	if err := proto.Unmarshal(msg.Value, body); err != nil {
		slog.Error("unmarshal toMysql message", "err", err, "key", msg.Key)
		return msg.Ack(ctx)
	}
	if len(body.MsgData) == 0 {
		slog.Error("toMysql payload is empty", "key", msg.Key)
		return msg.Ack(ctx)
	}
	if err := h.db.InsertMessages(ctx, body.ConversationId, body.MsgData); err != nil {
		slog.Error("insert messages", "err", err, "conversationID", body.ConversationId, "lastSeq", body.LastSeq)
		return err
	}
	for _, item := range body.MsgData {
		h.webhookAfterMsgSaveDB(ctx, item)
	}
	return msg.Ack(ctx)
}
