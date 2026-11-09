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

// 分配序号，并在 Redis 里保留最近的消息。
type SeqCache interface {
	Malloc(ctx context.Context, conversationID string, n int64) (int64, error)
	ClientSeq(ctx context.Context, conversationID, clientMsgID string) (int64, bool, error)
	RememberClientSeq(ctx context.Context, conversationID, clientMsgID string, seq int64) error
	SetMessages(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error
	SetUserReadSeq(ctx context.Context, conversationID string, userSeq map[string]int64) error
}

// 在 MySQL 里保存消息和已读序号。
type MsgDB interface {
	InsertMessages(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error
	SetUserReadSeq(ctx context.Context, conversationID string, userSeq map[string]int64) error
	SaveMaxSeq(ctx context.Context, conversationID string, maxSeq int64) error
}

// 分配序号、写入 Redis，并投递 toMysql 和 toPush。
type Transfer struct {
	cache   SeqCache
	db      MsgDB
	toMySQL mq.Producer
	toPush  mq.Producer
}

// 用序号缓存和两条下游队列构造传输器。
func NewTransfer(cache SeqCache, db MsgDB, toMySQL, toPush mq.Producer) *Transfer {
	return &Transfer{cache: cache, db: db, toMySQL: toMySQL, toPush: toPush}
}

// 分配序号、把消息写入 Redis，并记下发送方已读序号。lastSeq 是这批新序号之前的会话最大值。isNew 只在第一批为真。
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
			if msg.SendTime == 0 {
				msg.SendTime = sendTime
			}
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

// 把已读回执写入 MySQL。
func (t *Transfer) SetHasReadSeqToDB(ctx context.Context, conversationID string, userSeq map[string]int64) error {
	return t.db.SetUserReadSeq(ctx, conversationID, userSeq)
}

// 把已编号的消息交给 MySQL 消费者。
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

// 把一条消息交给推送。
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
