package logic

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/mq"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

// SeqCache assigns sequences and keeps recent messages in Redis.
type SeqCache interface {
	Malloc(ctx context.Context, conversationID string, n int64) (int64, error)
	ClientSeq(ctx context.Context, conversationID, clientMsgID string) (int64, bool, error)
	RememberClientSeq(ctx context.Context, conversationID, clientMsgID string, seq int64) error
	SetMessages(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error
	SetUserReadSeq(ctx context.Context, conversationID string, userSeq map[string]int64) error
}

// MsgDB stores messages and read sequences in MySQL.
type MsgDB interface {
	InsertMessages(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error
	SetUserReadSeq(ctx context.Context, conversationID string, userSeq map[string]int64) error
	SaveMaxSeq(ctx context.Context, conversationID string, maxSeq int64) error
}

// Transfer assigns sequences, fills Redis, and publishes toMysql and toPush.
type Transfer struct {
	cache   SeqCache
	db      MsgDB
	toMySQL mq.Producer
	toPush  mq.Producer
}

func NewTransfer(cache SeqCache, db MsgDB, toMySQL, toPush mq.Producer) *Transfer {
	return &Transfer{cache: cache, db: db, toMySQL: toMySQL, toPush: toPush}
}

// BatchInsertChat2Cache assigns seq, writes the messages into Redis, and records the sender read seq.
// lastSeq is the conversation max before the newly allocated numbers. isNew is true for the first batch.
func (t *Transfer) BatchInsertChat2Cache(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) (lastSeq int64, isNew bool, userSeq map[string]int64, err error) {
	if len(msgs) == 0 {
		return 0, false, nil, fmt.Errorf("no messages")
	}
	fresh := make([]*msgpb.MsgData, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			return 0, false, nil, fmt.Errorf("nil message")
		}
		if msg.ClientMsgId == "" {
			fresh = append(fresh, msg)
			continue
		}
		seq, ok, err := t.cache.ClientSeq(ctx, conversationID, msg.ClientMsgId)
		if err != nil {
			return 0, false, nil, err
		}
		if ok {
			msg.Seq = seq
			continue
		}
		fresh = append(fresh, msg)
	}
	var before int64
	if len(fresh) > 0 {
		before, err = t.cache.Malloc(ctx, conversationID, int64(len(fresh)))
		if err != nil {
			return 0, false, nil, err
		}
		sendTime := time.Now().UnixMilli()
		seq := before
		for _, msg := range fresh {
			seq++
			msg.Seq = seq
			msg.SendTime = sendTime
			if msg.ServerMsgId == "" {
				msg.ServerMsgId, err = serverMsgID(conversationID, msg.ClientMsgId)
				if err != nil {
					return 0, false, nil, err
				}
			}
			if msg.Status == 0 || msg.Status == model.StatusSending {
				msg.Status = model.StatusSuccess
			}
			if err = t.cache.RememberClientSeq(ctx, conversationID, msg.ClientMsgId, msg.Seq); err != nil {
				return 0, false, nil, err
			}
		}
		lastSeq = before
		isNew = before == 0 && len(fresh) == len(msgs)
	}
	userSeq = map[string]int64{}
	var maxSeq int64
	for _, msg := range msgs {
		if msg.Seq > maxSeq {
			maxSeq = msg.Seq
		}
		if msg.SendId != "" && msg.Seq > userSeq[msg.SendId] {
			userSeq[msg.SendId] = msg.Seq
		}
	}
	if len(fresh) == 0 {
		lastSeq = maxSeq
	}
	if err = t.cache.SetMessages(ctx, conversationID, msgs); err != nil {
		return 0, false, nil, err
	}
	if err = t.cache.SetUserReadSeq(ctx, conversationID, userSeq); err != nil {
		return 0, false, nil, err
	}
	if err = t.db.SaveMaxSeq(ctx, conversationID, maxSeq); err != nil {
		return 0, false, nil, err
	}
	return lastSeq, isNew, userSeq, nil
}

// SetHasReadSeqToDB stores read receipts.
func (t *Transfer) SetHasReadSeqToDB(ctx context.Context, conversationID string, userSeq map[string]int64) error {
	return t.db.SetUserReadSeq(ctx, conversationID, userSeq)
}

// MsgToMySQL publishes sequenced messages for the MySQL consumer.
func (t *Transfer) MsgToMySQL(ctx context.Context, key, conversationID string, msgs []*msgpb.MsgData, lastSeq int64) error {
	if len(msgs) == 0 {
		return nil
	}
	body, err := proto.Marshal(&msgpb.MsgDataToMySQL{
		ConversationId: conversationID,
		MsgData:        msgs,
		LastSeq:        lastSeq,
	})
	if err != nil {
		return err
	}
	return t.toMySQL.Send(ctx, key, body)
}

// MsgToPush publishes one message for the push service.
func (t *Transfer) MsgToPush(ctx context.Context, key, conversationID string, msg *msgpb.MsgData) error {
	body, err := proto.Marshal(&msgpb.PushMsgData{
		MsgData:        msg,
		ConversationId: conversationID,
	})
	if err != nil {
		return err
	}
	return t.toPush.Send(ctx, key, body)
}
