package logic

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
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

// 通知会话服务补会话行。不直接写会话表。
type Conversations interface {
	Ensure(ctx context.Context, conversationID, sendID, recvID string, sessionType int32) error
	EnsureGroup(ctx context.Context, groupID, callerID string, sessionType int32) error
}

// 消费 toRedis，分配序号，再投递 toMysql 和 toPush。
type RedisHandler struct {
	transfer *Transfer
	chats    Conversations
	ready    sync.Map
	batch    *batcher
	readCh   chan readSeq
	readCtx  context.Context
	stopRead context.CancelFunc
	wg       sync.WaitGroup
}

// 序号分配后再为单聊补会话行。
func (h *RedisHandler) UseSingleChats(chats Conversations) {
	if h != nil {
		h.chats = chats
	}
}

// 用序号缓存和两条下游队列构造 toRedis 消费者。
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

// 启动批处理 worker 和已读序号写回，直到上下文取消。
func (h *RedisHandler) Start(ctx context.Context) {
	h.batch.Start(ctx)
	h.wg.Add(1)
	go h.runReadSeq()
}

// 停掉已读序号写回。要等消费者先停。
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

// 把一条 toRedis 消息放入批处理。worker 在 toMysql 和 toPush 都成功后才确认。
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
		if err := h.ensureConversation(ctx, id, msgs); err != nil {
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

func (h *RedisHandler) ensureConversation(ctx context.Context, conversationID string, msgs []*msgpb.MsgData) error {
	if h.chats == nil || len(msgs) == 0 || msgs[0] == nil {
		return nil
	}
	msg := msgs[0]
	switch {
	case strings.HasPrefix(conversationID, "si_") || strings.HasPrefix(conversationID, "sn_"):
		if msg.SendId == "" || msg.RecvId == "" || msg.SendId == msg.RecvId {
			return nil
		}
		return h.chats.Ensure(ctx, conversationID, msg.SendId, msg.RecvId, msg.SessionType)
	case strings.HasPrefix(conversationID, "n_"):
		if msg.GroupId != "" || msg.SessionType == model.SessionWriteGroup || msg.SessionType == model.SessionReadGroup {
			return nil
		}
		if msg.SendId == "" || msg.RecvId == "" || msg.SendId == msg.RecvId {
			return nil
		}
		return h.ensureOnce(conversationID, func() error {
			return h.chats.Ensure(ctx, conversationID, msg.SendId, msg.RecvId, model.SessionNotification)
		})
	case strings.HasPrefix(conversationID, "g_"), strings.HasPrefix(conversationID, "sg_"):
		if msg.GroupId == "" || msg.SendId == "" {
			return nil
		}
		return h.ensureOnce(conversationID, func() error {
			return h.chats.EnsureGroup(ctx, msg.GroupId, msg.SendId, msg.SessionType)
		})
	default:
		return nil
	}
}

func (h *RedisHandler) ensureOnce(conversationID string, create func() error) error {
	if _, ok := h.ready.Load(conversationID); ok {
		return nil
	}
	if err := create(); err != nil {
		return err
	}
	h.ready.Store(conversationID, struct{}{})
	return nil
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
