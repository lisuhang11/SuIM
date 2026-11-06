package ws

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestSignalForwardsToRtc(t *testing.T) {
	fake := &fakeSignals{out: &msgpb.CallSignal{
		Op: msgpb.CallOp_CALL_ACCEPT, CallId: "room", Url: "ws://127.0.0.1:7880", Token: "signed",
	}}
	server := New(Options{Signals: fake})
	client := newTestClient(t, server)
	client.id = "conn-7"
	body, err := proto.Marshal(&msgpb.CallSignal{Op: msgpb.CallOp_CALL_ACCEPT, CallId: "room"})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.dispatch(client, &Req{ReqIdentifier: reqSendSignalMsg, Data: body}); err != nil {
		t.Fatal(err)
	}
	if fake.user != client.userID || fake.conn != "conn-7" || fake.in.GetCallId() != "room" {
		t.Fatalf("forwarded user %s conn %s call %s", fake.user, fake.conn, fake.in.GetCallId())
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != 0 {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
	out := &msgpb.CallSignal{}
	if err := proto.Unmarshal(resp.Data, out); err != nil {
		t.Fatal(err)
	}
	if out.GetToken() != "signed" || out.GetUrl() != "ws://127.0.0.1:7880" || out.GetOp() != msgpb.CallOp_CALL_ACCEPT {
		t.Fatalf("reply %+v", out)
	}
}

func TestSignalForwardsRtcError(t *testing.T) {
	server := New(Options{Signals: &fakeSignals{err: errorcode.ErrMsgNotFriend}})
	client := newTestClient(t, server)
	body, err := proto.Marshal(&msgpb.CallSignal{
		Op: msgpb.CallOp_CALL_INVITE, PeerId: "b", Media: msgpb.CallMedia_CALL_AUDIO,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.dispatch(client, &Req{ReqIdentifier: reqSendSignalMsg, Data: body}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != int(errorcode.MsgNotFriend) || resp.ErrMsg != "not friends" {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
}

func TestSignalUnsupportedWithoutRtc(t *testing.T) {
	client := newTestClient(t, New(Options{}))
	if err := client.server.dispatch(client, &Req{ReqIdentifier: reqSendSignalMsg, Data: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != errArgs || resp.ErrMsg != "signal messages are not supported" {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
}

func TestDeliverCallSkipsAcceptingSocket(t *testing.T) {
	server := New(Options{})
	accepting := newTestClient(t, server)
	other := newTestClient(t, server)
	accepting.userID = "callee"
	other.userID = "callee"
	accepting.platformID = 1
	other.platformID = 2
	accepting.id = "accepting"
	other.id = "other"
	server.hub.Add(accepting)
	server.hub.Add(other)
	server.deliverCall(call.Event{
		Users: []string{"callee"}, Except: "accepting", Op: call.OpAnswered,
		CallID: "room", Caller: "caller", Callee: "callee", Media: call.MediaAudio,
	})
	if n := len(accepting.conn.(*memSocket).writes); n != 0 {
		t.Fatalf("accepting socket received %d frames", n)
	}
	resp := decodeResp(t, other)
	if resp.ReqIdentifier != reqCallSignal {
		t.Fatalf("frame %d", resp.ReqIdentifier)
	}
	out := &msgpb.CallSignal{}
	if err := proto.Unmarshal(resp.Data, out); err != nil {
		t.Fatal(err)
	}
	if out.GetOp() != msgpb.CallOp_CALL_ANSWERED || out.GetToken() != "" || out.GetCallerId() != "caller" {
		t.Fatalf("push %+v", out)
	}
}

type fakeSignals struct {
	user string
	conn string
	in   *msgpb.CallSignal
	out  *msgpb.CallSignal
	err  error
}

func (f *fakeSignals) Assemble(_ context.Context, userID, connID string, in *msgpb.CallSignal) (*msgpb.CallSignal, error) {
	f.user = userID
	f.conn = connID
	f.in = in
	return f.out, f.err
}
