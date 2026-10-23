package ws

import (
	"context"
	"errors"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/mq"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msggateway/internal/history"
)

func TestSingleSendRequiresFriends(t *testing.T) {
	relations := &fakeRelations{friend1: true}
	prod := &fakeProducer{}
	client := newTestClient(t, New(Options{Relations: relations, Publish: prod}))

	msg := &msgpb.MsgData{SessionType: sessionSingle, RecvId: "b", Content: []byte("hi"), ClientMsgId: "c1"}
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.server.sendMsg(client, &Req{ReqIdentifier: reqSendMsg, Data: body}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != int(errorcode.MsgNotFriend) {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
	if prod.n != 0 {
		t.Fatalf("published %d", prod.n)
	}
}

func TestSingleSendRejectsBlock(t *testing.T) {
	relations := &fakeRelations{friend1: true, friend2: true, black2: true}
	client := newTestClient(t, New(Options{Relations: relations, Publish: &fakeProducer{}}))
	msg := &msgpb.MsgData{SessionType: sessionSingle, RecvId: "b", Content: []byte("hi")}
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.server.sendMsg(client, &Req{ReqIdentifier: reqSendMsg, Data: body}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != int(errorcode.MsgBlocked) {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
}

func TestSingleSendPublishes(t *testing.T) {
	relations := &fakeRelations{friend1: true, friend2: true}
	prod := &fakeProducer{}
	client := newTestClient(t, New(Options{Relations: relations, Publish: prod}))
	client.userID = "a"
	msg := &msgpb.MsgData{SessionType: sessionSingle, RecvId: "b", Content: []byte("hi"), ClientMsgId: "c1"}
	body, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.server.sendMsg(client, &Req{ReqIdentifier: reqSendMsg, SendID: "a", Data: body}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != 0 {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
	out := &msgpb.SendMsgResp{}
	if err := proto.Unmarshal(resp.Data, out); err != nil {
		t.Fatal(err)
	}
	if out.ServerMsgId == "" || out.ClientMsgId != "c1" || out.SendTime == 0 {
		t.Fatalf("resp %+v", out)
	}
	if prod.n != 1 || prod.key != "si_a_b" {
		t.Fatalf("publish n=%d key=%s", prod.n, prod.key)
	}
}

func TestPullByRangeUses1002(t *testing.T) {
	hist := &fakeHistory{msgs: []*msgpb.MsgData{{Seq: 1, Content: []byte("hi")}}, max: 3}
	client := newTestClient(t, New(Options{History: hist}))
	reqBody, err := proto.Marshal(&msgpb.PullMessageBySeqsReq{
		UserId: "a",
		Order:  orderAsc,
		SeqRanges: []*msgpb.SeqRange{{
			ConversationId: "si_a_b",
			Begin:          1,
			End:            1,
			Num:            20,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.server.dispatch(client, &Req{ReqIdentifier: reqPullMsgByRange, Data: reqBody}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != 0 {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
	out := &msgpb.PushMessages{}
	if err := proto.Unmarshal(resp.Data, out); err != nil {
		t.Fatal(err)
	}
	pull := out.Msgs["si_a_b"]
	if pull == nil || pull.IsEnd || len(pull.Msgs) != 1 || string(pull.Msgs[0].Content) != "hi" {
		t.Fatalf("pull %+v", pull)
	}
}

func TestRangeIsEnd(t *testing.T) {
	if rangeIsEnd(orderAsc, 1, 2, 3) {
		t.Fatal("ascending pull is not finished")
	}
	if !rangeIsEnd(orderAsc, 1, 3, 3) {
		t.Fatal("ascending pull is finished")
	}
	if rangeIsEnd(orderDesc, 4, 6, 6) {
		t.Fatal("descending pull is not finished")
	}
	if !rangeIsEnd(orderDesc, 1, 3, 6) {
		t.Fatal("descending pull is finished")
	}
}

type fakeRelations struct {
	friend1, friend2 bool
	black1, black2   bool
}

func (f *fakeRelations) IsFriend(context.Context, string, string) (bool, bool, error) {
	return f.friend1, f.friend2, nil
}

func (f *fakeRelations) IsBlack(context.Context, string, string) (bool, bool, error) {
	return f.black1, f.black2, nil
}

type fakeProducer struct {
	n   int
	key string
}

func (f *fakeProducer) Send(_ context.Context, key string, _ []byte) error {
	f.n++
	f.key = key
	return nil
}

type fakeHistory struct {
	msgs []*msgpb.MsgData
	max  int64
}

func (f *fakeHistory) ConversationIDs(context.Context, string) ([]string, error) { return nil, nil }
func (f *fakeHistory) MaxSeqs(context.Context, []string) (map[string]int64, error) {
	return map[string]int64{"si_a_b": f.max}, nil
}
func (f *fakeHistory) SeqState(context.Context, string, []string) (map[string]history.SeqState, error) {
	return nil, nil
}
func (f *fakeHistory) Range(context.Context, string, int64, int64, int64) ([]*msgpb.MsgData, error) {
	return f.msgs, nil
}
func (f *fakeHistory) BySeqs(context.Context, string, []int64) ([]*msgpb.MsgData, error) {
	return f.msgs, nil
}
func (f *fakeHistory) Last(context.Context, string) (*msgpb.MsgData, error) { return nil, nil }

type memSocket struct {
	mu     sync.Mutex
	writes [][]byte
}

func (m *memSocket) Read() (int, []byte, error) { return 0, nil, errors.New("closed") }
func (m *memSocket) Write(_ int, data []byte) error {
	m.mu.Lock()
	m.writes = append(m.writes, append([]byte(nil), data...))
	m.mu.Unlock()
	return nil
}
func (m *memSocket) Close() error { return nil }

func newTestClient(t *testing.T, server *Server) *Client {
	t.Helper()
	return &Client{
		userID:  "a",
		encoder: jsonEncoder{},
		conn:    &memSocket{},
		server:  server,
	}
}

func decodeResp(t *testing.T, client *Client) Resp {
	t.Helper()
	sock := client.conn.(*memSocket)
	sock.mu.Lock()
	defer sock.mu.Unlock()
	if len(sock.writes) != 1 {
		t.Fatalf("writes %d", len(sock.writes))
	}
	var resp Resp
	if err := client.encoder.Decode(sock.writes[0], &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

var _ mq.Producer = (*fakeProducer)(nil)
