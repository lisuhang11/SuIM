package kafka

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/mq"
)

// bindKafkaMessage fills Message context and Ack using unexported setters via
// the mq package's exported helpers on Message.
func bindKafkaMessage(msg *mq.Message, ctx context.Context, item *pendingMessage, autoCommit bool) {
	mq.Bind(msg, ctx, func(ackCtx context.Context) error {
		if item == nil || item.session == nil || item.msg == nil {
			return nil
		}
		item.session.MarkMessage(item.msg, "")
		if !autoCommit {
			item.session.Commit()
		}
		return nil
	})
}
