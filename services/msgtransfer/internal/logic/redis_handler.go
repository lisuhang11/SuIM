package logic

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/mq"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

type incoming struct {
	ctx context.Context
	msg *msgpb.MsgData
	raw *mq.Message
}

type readSeq struct {
	conversationID string
	userSeq        map[string]int64
}

// SingleChats creates the two rows of a single-chat conversation.
type SingleChats interface {
	Ensure(ctx context.Context, conversationID, sendID, recvID string) error
}

// RedisHandler consumes toRedis, assigns sequences, and publishes toMysql plus toPush.
type RedisHandler struct {
	transfer *Transfer
	chats    SingleChats
	batch    *batcher
	readCh   chan readSeq
	readCtx  context.Context
	stopRead context.CancelFunc
	wg       sync.WaitGroup
}

// UseSingleChats stores conversation rows after a single-chat message is sequenced.
func (h *RedisHandler) UseSingleChats(chats SingleChats) {
	if h != nil {
		h.chats = chats
	}
}

func NewRedisHandler(transfer *Transfer, batchSize, workers int, interval time.Duration) *RedisHandler {
	readCtx, stopRead := context.WithCancel(context.Background())
	h := &RedisHandler{
		transfer: transfer,
		readCh:   make(chan readSeq, 1000),
		readCtx:  readCtx,
		stopRead: stopRead,
	}
	h.batch = newBatcher(batchSize, workers, interval, h.do)
	return h
}

// Start runs the batch workers and the read-seq writer until ctx is canceled.
func (h *RedisHandler) Start(ctx context.Context) {
	h.batch.Start(ctx)
	h.wg.Add(1)
	go h.runReadSeq()
}

// Close stops the read-seq writer. Call it after consumers have stopped.
func (h *RedisHandler) Close() {
	for {
		select {
		case item := <-h.readCh:
			if err := h.transfer.SetHasReadSeqToDB(context.Background(), item.conversationID, item.userSeq); err != nil {
				slog.Error("set read seq", "err", err, "conversationID", item.conversationID)
			}
		default:
			h.stopRead()
			h.wg.Wait()
			return
		}
	}
}

// Handle queues one toRedis message. The batch worker acks it after toMysql and toPush succeed.
func (h *RedisHandler) Handle(ctx context.Context, msg *mq.Message) error {
	return h.batch.Put(ctx, msg)
}

func (h *RedisHandler) runReadSeq() {
	defer h.wg.Done()
	for {
		select {
		case <-h.readCtx.Done():
			return
		case item := <-h.readCh:
			if err := h.transfer.SetHasReadSeqToDB(context.Background(), item.conversationID, item.userSeq); err != nil {
				slog.Error("set read seq", "err", err, "conversationID", item.conversationID)
			}
		}
	}
}

func (h *RedisHandler) do(ctx context.Context, batch []*mq.Message) error {
	parsed := make([]*incoming, 0, len(batch))
	for _, raw := range batch {
		msg := &msgpb.MsgData{}
		if err := proto.Unmarshal(raw.Value, msg); err != nil {
			slog.Error("unmarshal toRedis message", "err", err, "key", raw.Key)
			_ = raw.Ack(ctx)
			continue
		}
		parsed = append(parsed, &incoming{ctx: raw.Context(), msg: msg, raw: raw})
	}
	if len(parsed) == 0 {
		return nil
	}
	if err := h.applyReadReceipts(ctx, parsed); err != nil {
		return err
	}
	storage, notStorage, storageN, notStorageN := splitMessages(parsed)
	key := batch[0].Key
	if err := h.handleChat(ctx, key, storage, notStorage); err != nil {
		return err
	}
	if err := h.handleNotification(ctx, key, storageN, notStorageN); err != nil {
		return err
	}
	for _, item := range parsed {
		if err := item.raw.Ack(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (h *RedisHandler) applyReadReceipts(ctx context.Context, msgs []*incoming) error {
	conversationUserSeq := map[string]map[string]int64{}
	for _, item := range msgs {
		if item.msg.ContentType != model.ContentHasReadReceipt {
			continue
		}
		var elem struct {
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal(item.msg.Content, &elem); err != nil {
			slog.Warn("unmarshal read receipt", "err", err)
			continue
		}
		var tips struct {
			MarkAsReadUserID string  `json:"markAsReadUserID"`
			ConversationID   string  `json:"conversationID"`
			Seqs             []int64 `json:"seqs"`
			HasReadSeq       int64   `json:"hasReadSeq"`
		}
		if err := json.Unmarshal([]byte(elem.Detail), &tips); err != nil {
			slog.Warn("unmarshal read receipt detail", "err", err)
			continue
		}
		if tips.ConversationID == "" || tips.HasReadSeq < 0 {
			continue
		}
		for _, seq := range tips.Seqs {
			if tips.HasReadSeq < seq {
				tips.HasReadSeq = seq
			}
		}
		users := conversationUserSeq[tips.ConversationID]
		if users == nil {
			users = map[string]int64{}
			conversationUserSeq[tips.ConversationID] = users
		}
		if users[tips.MarkAsReadUserID] < tips.HasReadSeq {
			users[tips.MarkAsReadUserID] = tips.HasReadSeq
		}
	}
	for conversationID, userSeq := range conversationUserSeq {
		if err := h.transfer.SetHasReadSeqToDB(ctx, conversationID, userSeq); err != nil {
			return err
		}
	}
	return nil
}

func splitMessages(msgs []*incoming) (storage, notStorage, storageN, notStorageN []*incoming) {
	for _, item := range msgs {
		opt := msgOptions(item.msg.Options)
		if opt.notNotification() {
			if opt.history() {
				storage = append(storage, item)
			} else {
				notStorage = append(notStorage, item)
			}
			continue
		}
		if opt.sendMsg() {
			clone := proto.Clone(item.msg).(*msgpb.MsgData)
			clone.Options = map[string]bool{
				model.OptNotNotification: true,
				model.OptOfflinePush:     opt.offlinePush(),
				model.OptUnreadCount:     opt.unreadCount(),
			}
			storage = append(storage, &incoming{ctx: item.ctx, msg: clone, raw: item.raw})
			if item.msg.Options == nil {
				item.msg.Options = map[string]bool{}
			}
			item.msg.Options[model.OptOfflinePush] = false
			item.msg.Options[model.OptUnreadCount] = false
		}
		if opt.history() {
			storageN = append(storageN, item)
		} else {
			notStorageN = append(notStorageN, item)
		}
	}
	return storage, notStorage, storageN, notStorageN
}

func (h *RedisHandler) handleChat(ctx context.Context, key string, storage, notStorage []*incoming) error {
	if err := h.push(ctx, key, notStorage, ChatConversationID); err != nil {
		return err
	}
	return h.persist(ctx, key, storage, ChatConversationID)
}

func (h *RedisHandler) handleNotification(ctx context.Context, key string, storage, notStorage []*incoming) error {
	if err := h.push(ctx, key, notStorage, NotificationConversationID); err != nil {
		return err
	}
	return h.persist(ctx, key, storage, NotificationConversationID)
}

func (h *RedisHandler) persist(ctx context.Context, key string, list []*incoming, idOf func(*msgpb.MsgData) string) error {
	groups := map[string][]*incoming{}
	var order []string
	for _, item := range list {
		id := idOf(item.msg)
		if id == "" {
			slog.Error("empty conversation id", "sessionType", item.msg.SessionType, "sendID", item.msg.SendId)
			continue
		}
		if _, ok := groups[id]; !ok {
			order = append(order, id)
		}
		groups[id] = append(groups[id], item)
	}
	for _, id := range order {
		items := groups[id]
		msgs := make([]*msgpb.MsgData, len(items))
		for i, item := range items {
			msgs[i] = item.msg
		}
		lastSeq, isNew, userSeq, err := h.transfer.BatchInsertChat2Cache(ctx, id, msgs)
		if err != nil {
			return err
		}
		if err := h.ensureSingle(ctx, id, msgs); err != nil {
			return err
		}
		if isNew {
			slog.Info("new conversation", "conversationID", id, "sessionType", msgs[0].SessionType)
		}
		if len(userSeq) > 0 {
			select {
			case h.readCh <- readSeq{conversationID: id, userSeq: userSeq}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err := h.transfer.MsgToMySQL(ctx, key, id, msgs, lastSeq); err != nil {
			return err
		}
		if err := h.push(ctx, key, items, func(*msgpb.MsgData) string { return id }); err != nil {
			return err
		}
	}
	return nil
}

func (h *RedisHandler) ensureSingle(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error {
	if h.chats == nil || len(msgs) == 0 || msgs[0] == nil {
		return nil
	}
	msg := msgs[0]
	if msg.SessionType != model.SessionSingle || msg.SendId == "" || msg.RecvId == "" || msg.SendId == msg.RecvId {
		return nil
	}
	return h.chats.Ensure(ctx, conversationID, msg.SendId, msg.RecvId)
}

func (h *RedisHandler) push(ctx context.Context, key string, list []*incoming, idOf func(*msgpb.MsgData) string) error {
	for _, item := range list {
		msgCtx := item.ctx
		if msgCtx == nil {
			msgCtx = ctx
		}
		if err := h.transfer.MsgToPush(msgCtx, key, idOf(item.msg), item.msg); err != nil {
			return err
		}
	}
	return nil
}
