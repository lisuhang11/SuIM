package logic

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/mq"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/config"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

func TestChatConversationID(t *testing.T) {
	single := &msgpb.MsgData{SessionType: model.SessionSingle, SendId: "b", RecvId: "a"}
	if got := ChatConversationID(single); got != "si_a_b" {
		t.Fatalf("single id %s", got)
	}
	group := &msgpb.MsgData{SessionType: model.SessionReadGroup, GroupId: "g1"}
	if got := ChatConversationID(group); got != "sg_g1" {
		t.Fatalf("group id %s", got)
	}
	if got := NotificationConversationID(single); got != "n_a_b" {
		t.Fatalf("notification id %s", got)
	}
}

func TestTransferPersistsToMySQL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := newMemCache()
	db := newMemDB()
	bus := mq.NewMemory()
	pushed := &recordProd{}
	transfer := NewTransfer(cache, db, bus.Producer(mq.TopicToMySQL), pushed)
	redisH := NewRedisHandler(transfer, 1, 1, 10*time.Millisecond)
	chats := &memChats{}
	redisH.UseSingleChats(chats)
	redisH.Start(ctx)
	defer redisH.Close()

	mysqlH := NewMySQLHandler(db, nil, config.AfterMsgSaveDB{})
	go func() { _ = bus.Consumer(mq.TopicToMySQL).Subscribe(ctx, mysqlH.Handle) }()

	msg := &msgpb.MsgData{
		SendId:      "b",
		RecvId:      "a",
		SessionType: model.SessionSingle,
		ContentType: 101,
		Content:     []byte("hi"),
		ClientMsgId: "c1",
		Status:      model.StatusSending,
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := redisH.Handle(ctx, &mq.Message{Key: "si_a_b", Value: body}); err != nil {
		t.Fatal(err)
	}

	row := waitRow(t, db, "si_a_b", 1)
	if row.Status != model.StatusSuccess {
		t.Fatalf("status %d", row.Status)
	}
	if string(row.Content) != "hi" {
		t.Fatalf("content %q", row.Content)
	}
	if row.ServerMsgId == "" {
		t.Fatal("missing server msg id")
	}
	waitRead(t, db, "si_a_b", "b", 1)
	if got := pushed.count(); got != 1 {
		t.Fatalf("push count %d", got)
	}

	if err := redisH.Handle(ctx, &mq.Message{Key: "si_a_b", Value: body}); err != nil {
		t.Fatal(err)
	}
	waitCount(t, pushed, 2)
	if got := db.count("si_a_b"); got != 1 {
		t.Fatalf("retry stored %d rows", got)
	}
	if got := cache.seqOf("si_a_b"); got != 1 {
		t.Fatalf("seq after retry %d", got)
	}
	if got := chats.count(); got != 2 {
		t.Fatalf("conversation ensures %d", got)
	}
}

func TestNotStoredMessageOnlyPushes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := newMemDB()
	pushed := &recordProd{}
	transfer := NewTransfer(newMemCache(), db, &recordProd{}, pushed)
	redisH := NewRedisHandler(transfer, 1, 1, 10*time.Millisecond)
	chats := &memChats{}
	redisH.UseSingleChats(chats)
	redisH.Start(ctx)
	defer redisH.Close()

	msg := &msgpb.MsgData{
		SendId:      "a",
		RecvId:      "b",
		SessionType: model.SessionSingle,
		ContentType: 101,
		Content:     []byte("tmp"),
		ClientMsgId: "c2",
		Options:     map[string]bool{model.OptHistory: false},
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := redisH.Handle(ctx, &mq.Message{Key: "si_a_b", Value: body}); err != nil {
		t.Fatal(err)
	}
	waitCount(t, pushed, 1)
	if got := db.count("si_a_b"); got != 0 {
		t.Fatalf("stored %d", got)
	}
	if got := chats.count(); got != 0 {
		t.Fatalf("conversation ensures %d", got)
	}
}

func TestReadReceipt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := newMemDB()
	transfer := NewTransfer(newMemCache(), db, &recordProd{}, &recordProd{})
	redisH := NewRedisHandler(transfer, 1, 1, 10*time.Millisecond)
	redisH.Start(ctx)
	defer redisH.Close()

	detail := `{"detail":"{\"conversationID\":\"si_a_b\",\"hasReadSeq\":1,\"seqs\":[1,4],\"markAsReadUserID\":\"b\"}"}`
	msg := &msgpb.MsgData{
		SendId:      "b",
		RecvId:      "a",
		SessionType: model.SessionSingle,
		ContentType: model.ContentHasReadReceipt,
		Content:     []byte(detail),
		ClientMsgId: "read-1",
		Options: map[string]bool{
			model.OptHistory:         false,
			model.OptNotNotification: false,
			model.OptSendMsg:         false,
		},
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := redisH.Handle(ctx, &mq.Message{Key: "si_a_b", Value: body}); err != nil {
		t.Fatal(err)
	}
	waitRead(t, db, "si_a_b", "b", 4)
	if got := db.count("n_a_b") + db.count("si_a_b"); got != 0 {
		t.Fatalf("receipt was stored %d", got)
	}
}

func TestAllowAfterMsg(t *testing.T) {
	text := &msgpb.MsgData{ContentType: 101, SessionType: model.SessionSingle, RecvId: "a"}
	if !allowAfterMsg(text, config.AfterMsgSaveDB{}) {
		t.Fatal("text should pass")
	}
	if allowAfterMsg(&msgpb.MsgData{ContentType: model.ContentTyping}, config.AfterMsgSaveDB{}) {
		t.Fatal("typing should be denied")
	}
	if allowAfterMsg(&msgpb.MsgData{ContentType: model.NotificationBegin + 1}, config.AfterMsgSaveDB{}) {
		t.Fatal("notification should be denied")
	}
	after := config.AfterMsgSaveDB{AttentionIds: []string{"a"}}
	if !allowAfterMsg(text, after) {
		t.Fatal("attended receiver should pass")
	}
	text.RecvId = "z"
	if allowAfterMsg(text, after) {
		t.Fatal("other receiver should be denied")
	}
}

type memCache struct {
	mu     sync.Mutex
	seq    map[string]int64
	client map[string]int64
	read   map[string]int64
}

func newMemCache() *memCache {
	return &memCache{seq: map[string]int64{}, client: map[string]int64{}, read: map[string]int64{}}
}

func (m *memCache) Malloc(_ context.Context, conversationID string, n int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	before := m.seq[conversationID]
	m.seq[conversationID] = before + n
	return before, nil
}

func (m *memCache) ClientSeq(_ context.Context, conversationID, clientMsgID string) (int64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seq, ok := m.client[conversationID+"\x00"+clientMsgID]
	return seq, ok, nil
}

func (m *memCache) RememberClientSeq(_ context.Context, conversationID, clientMsgID string, seq int64) error {
	if clientMsgID == "" {
		return nil
	}
	m.mu.Lock()
	m.client[conversationID+"\x00"+clientMsgID] = seq
	m.mu.Unlock()
	return nil
}

func (m *memCache) SetMessages(context.Context, string, []*msgpb.MsgData) error { return nil }

func (m *memCache) SetUserReadSeq(_ context.Context, conversationID string, userSeq map[string]int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for userID, seq := range userSeq {
		key := conversationID + "\x00" + userID
		if seq > m.read[key] {
			m.read[key] = seq
		}
	}
	return nil
}

func (m *memCache) seqOf(conversationID string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.seq[conversationID]
}

type memDB struct {
	mu   sync.Mutex
	rows map[string]*msgpb.MsgData
	read map[string]int64
	max  map[string]int64
}

func newMemDB() *memDB {
	return &memDB{rows: map[string]*msgpb.MsgData{}, read: map[string]int64{}, max: map[string]int64{}}
}

func (m *memDB) InsertMessages(_ context.Context, conversationID string, msgs []*msgpb.MsgData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range msgs {
		key := rowKey(conversationID, msg.Seq)
		if _, ok := m.rows[key]; ok {
			continue
		}
		cloned := proto.Clone(msg).(*msgpb.MsgData)
		if cloned.Status == model.StatusSending || cloned.Status == 0 {
			cloned.Status = model.StatusSuccess
		}
		m.rows[key] = cloned
	}
	return nil
}

func (m *memDB) SetUserReadSeq(_ context.Context, conversationID string, userSeq map[string]int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for userID, seq := range userSeq {
		key := conversationID + "\x00" + userID
		if seq > m.read[key] {
			m.read[key] = seq
		}
	}
	return nil
}

func (m *memDB) SaveMaxSeq(_ context.Context, conversationID string, maxSeq int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if maxSeq > m.max[conversationID] {
		m.max[conversationID] = maxSeq
	}
	return nil
}

func (m *memDB) count(conversationID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	prefix := conversationID + ":"
	for key := range m.rows {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

func (m *memDB) row(conversationID string, seq int64) *msgpb.MsgData {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rows[rowKey(conversationID, seq)]
}

func (m *memDB) readSeq(conversationID, userID string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.read[conversationID+"\x00"+userID]
}

func rowKey(conversationID string, seq int64) string {
	return conversationID + ":" + strconv.FormatInt(seq, 10)
}

type memChats struct {
	mu sync.Mutex
	n  int
}

func (m *memChats) Ensure(context.Context, string, string, string) error {
	m.mu.Lock()
	m.n++
	m.mu.Unlock()
	return nil
}

func (m *memChats) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.n
}

type recordProd struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (p *recordProd) Send(_ context.Context, _ string, value []byte) error {
	p.mu.Lock()
	p.msgs = append(p.msgs, append([]byte(nil), value...))
	p.mu.Unlock()
	return nil
}

func (p *recordProd) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.msgs)
}

func waitRow(t *testing.T, db *memDB, conversationID string, seq int64) *msgpb.MsgData {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if row := db.row(conversationID, seq); row != nil {
			return row
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("message %s/%d was not stored", conversationID, seq)
	return nil
}

func waitRead(t *testing.T, db *memDB, conversationID, userID string, seq int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if db.readSeq(conversationID, userID) == seq {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("read seq %s %s = %d, want %d", conversationID, userID, db.readSeq(conversationID, userID), seq)
}

func waitCount(t *testing.T, p *recordProd, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if p.count() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("count %d, want %d", p.count(), n)
}
