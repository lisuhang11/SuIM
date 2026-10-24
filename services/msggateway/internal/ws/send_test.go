package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestSendForwardsToMsg(t *testing.T) {
	messages := &fakeMessages{}
	client := newTestClient(t, New(Options{Messages: messages}))
	client.userID = "a"
	body, err := proto.Marshal(&msgpb.MsgData{SessionType: sessionSingle, RecvId: "b", Content: []byte("hi"), ClientMsgId: "c1"})
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
	if out.ClientMsgId != "c1" || out.ServerMsgId != "s1" {
		t.Fatalf("resp %+v", out)
	}
	if messages.sent == nil || messages.sent.SendId != "a" || messages.sent.RecvId != "b" {
		t.Fatalf("sent %+v", messages.sent)
	}
}

func TestSendMapsNotFriend(t *testing.T) {
	client := newTestClient(t, New(Options{Messages: &fakeMessages{sendErr: errorcode.ErrMsgNotFriend}}))
	body, err := proto.Marshal(&msgpb.MsgData{SessionType: sessionSingle, RecvId: "b", Content: []byte("hi")})
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
}

func TestPullByRangeUses1002(t *testing.T) {
	messages := &fakeMessages{pull: &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{
		"si_a_b": {Msgs: []*msgpb.MsgData{{Seq: 1, Content: []byte("hi")}}, IsEnd: false, MaxSeq: 3},
	}}}
	client := newTestClient(t, New(Options{Messages: messages}))
	reqBody, err := proto.Marshal(&msgpb.PullMessageBySeqsReq{
		SeqRanges: []*msgpb.SeqRange{{ConversationId: "si_a_b", Begin: 1, End: 1, Num: 20}},
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
	if messages.pullReq == nil || messages.pullReq.UserId != "a" {
		t.Fatalf("pull user %v", messages.pullReq)
	}
}

func TestHTTPSendText(t *testing.T) {
	messages := &fakeMessages{}
	srv := New(Options{Tokens: fakeTokens{}, Messages: messages})
	body := `{"recv_id":"b","content":" hi ","client_msg_id":"c1","session_type":1,"content_type":101}`
	req := httptest.NewRequest(http.MethodPost, "/msg/send", strings.NewReader(body))
	req.Header.Set("token", "t")
	rr := httptest.NewRecorder()
	srv.HandleSend(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if messages.sent == nil || messages.sent.SendId != "a" || string(messages.sent.Content) != `{"content":"hi"}` || messages.sent.ContentType != 101 {
		t.Fatalf("sent %+v", messages.sent)
	}
}

func TestHTTPPullText(t *testing.T) {
	srv := New(Options{
		Tokens: fakeTokens{},
		Messages: &fakeMessages{pull: &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{
			"si_a_b": {Msgs: []*msgpb.MsgData{{
				Seq: 1, SendId: "a", RecvId: "b", ClientMsgId: "c1", ServerMsgId: "s1",
				ContentType: 101, Content: []byte(`{"content":"111"}`), SendTime: 10, Status: 2,
			}}, IsEnd: true, MaxSeq: 1},
		}}},
	})
	req := httptest.NewRequest(http.MethodPost, "/msg/pull", strings.NewReader(
		`{"conversation_id":"si_a_b","limit":20}`))
	req.Header.Set("token", "t")
	rr := httptest.NewRecorder()
	srv.HandlePull(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			IsEnd  bool  `json:"is_end"`
			MaxSeq int64 `json:"max_seq"`
			Msgs   []struct {
				Text string `json:"text"`
			} `json:"msgs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != 0 || !out.Data.IsEnd || out.Data.MaxSeq != 1 || len(out.Data.Msgs) != 1 || out.Data.Msgs[0].Text != "111" {
		t.Fatalf("resp %+v", out)
	}
}

func TestHTTPSendRejectsStranger(t *testing.T) {
	srv := New(Options{Tokens: fakeTokens{}, Messages: &fakeMessages{sendErr: errorcode.ErrMsgNotFriend}})
	req := httptest.NewRequest(http.MethodPost, "/msg/send", strings.NewReader(
		`{"recv_id":"b","content":"hi","session_type":1,"content_type":101}`))
	req.Header.Set("token", "t")
	rr := httptest.NewRecorder()
	srv.HandleSend(rr, req)
	if rr.Code != http.StatusPreconditionFailed {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

type fakeMessages struct {
	sendErr error
	sent    *msgpb.MsgData
	pull    *msgpb.PushMessages
	pullReq *msgpb.PullMessageBySeqsReq
}

func (f *fakeMessages) SendMsg(_ context.Context, msg *msgpb.MsgData) (*msgpb.SendMsgResp, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	f.sent = proto.Clone(msg).(*msgpb.MsgData)
	id := msg.ClientMsgId
	if id == "" {
		id = "c1"
	}
	return &msgpb.SendMsgResp{ServerMsgId: "s1", ClientMsgId: id, SendTime: 10}, nil
}

func (f *fakeMessages) GetMaxSeq(context.Context, string) (*msgpb.GetMaxSeqResp, error) {
	return &msgpb.GetMaxSeqResp{}, nil
}

func (f *fakeMessages) PullMessageBySeqs(_ context.Context, req *msgpb.PullMessageBySeqsReq) (*msgpb.PushMessages, error) {
	f.pullReq = req
	if f.pull != nil {
		return f.pull, nil
	}
	return &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{}}, nil
}

func (f *fakeMessages) GetSeqMessage(context.Context, *msgpb.GetSeqMessageReq) (*msgpb.PushMessages, error) {
	return &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{}}, nil
}

func (f *fakeMessages) GetConversationsHasReadAndMaxSeq(context.Context, *msgpb.GetConversationsHasReadAndMaxSeqReq) (*msgpb.GetConversationsHasReadAndMaxSeqResp, error) {
	return &msgpb.GetConversationsHasReadAndMaxSeqResp{}, nil
}

func (f *fakeMessages) GetLastMessage(context.Context, *msgpb.GetLastMessageReq) (*msgpb.GetLastMessageResp, error) {
	return &msgpb.GetLastMessageResp{}, nil
}

type fakeTokens struct{}

func (fakeTokens) Validate(context.Context, string) (string, int32, error) {
	return "a", 5, nil
}

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
