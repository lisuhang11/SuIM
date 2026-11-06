package ws

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestSignalInviteRingsPeer(t *testing.T) {
	rdb := openOnlineRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := New(Options{Calls: call.New(rdb), Relations: fakeRelations{friends: true}})
	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	caller := newTestClient(t, server)
	callee := newTestClient(t, server)
	caller.userID = "ca" + id
	callee.userID = "cb" + id
	caller.id = "caller-conn"
	callee.id = "callee-conn"
	server.hub.Add(caller)
	server.hub.Add(callee)
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), call.Prefix+"busy:"+caller.userID, call.Prefix+"busy:"+callee.userID).Err()
	})
	before, err := rdb.PubSubNumSub(ctx, call.Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	go server.WatchCall(ctx)
	waitCallSubscribers(t, ctx, rdb, before[call.Channel])

	body, err := proto.Marshal(&msgpb.CallSignal{
		Op: msgpb.CallOp_CALL_INVITE, PeerId: callee.userID, Media: msgpb.CallMedia_CALL_AUDIO,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.dispatch(caller, &Req{ReqIdentifier: reqSendSignalMsg, Data: body}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, caller)
	if resp.ErrCode != 0 {
		t.Fatalf("invite code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
	invited := &msgpb.CallSignal{}
	if err := proto.Unmarshal(resp.Data, invited); err != nil {
		t.Fatal(err)
	}
	if invited.Op != msgpb.CallOp_CALL_INVITE || invited.CallerId != caller.userID || invited.CalleeId != callee.userID || invited.Media != msgpb.CallMedia_CALL_AUDIO {
		t.Fatalf("invite %+v", invited)
	}
	t.Cleanup(func() {
		_, _ = server.calls.Cancel(context.Background(), caller.userID, invited.CallId)
	})
	deadline := time.Now().Add(2 * time.Second)
	var ring msgpb.CallSignal
	for {
		sock := callee.conn.(*memSocket)
		sock.mu.Lock()
		n := len(sock.writes)
		var raw []byte
		if n > 0 {
			raw = append([]byte(nil), sock.writes[n-1]...)
		}
		sock.mu.Unlock()
		if n > 0 {
			var pushed Resp
			if err := callee.encoder.Decode(raw, &pushed); err != nil {
				t.Fatal(err)
			}
			if pushed.ReqIdentifier != reqCallSignal {
				t.Fatalf("frame %d", pushed.ReqIdentifier)
			}
			if err := proto.Unmarshal(pushed.Data, &ring); err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("callee was not rung")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ring.Op != msgpb.CallOp_CALL_RING || ring.CallId != invited.CallId || ring.CallerId != caller.userID {
		t.Fatalf("ring %+v", ring)
	}
}

func TestSignalAcceptIssuesLiveKitTokens(t *testing.T) {
	rdb := openOnlineRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := New(Options{
		Calls:     call.New(rdb),
		Relations: fakeRelations{friends: true},
		LiveKit: livekit.Config{
			URL:    "ws://127.0.0.1:7880",
			Key:    "suim",
			Secret: strings.Repeat("s", 32),
		},
	})
	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	caller := newTestClient(t, server)
	callee := newTestClient(t, server)
	caller.userID = "ta" + id
	callee.userID = "tb" + id
	caller.id = "caller-token"
	callee.id = "callee-token"
	server.hub.Add(caller)
	server.hub.Add(callee)
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), call.Prefix+"busy:"+caller.userID, call.Prefix+"busy:"+callee.userID).Err()
	})
	before, err := rdb.PubSubNumSub(ctx, call.Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	go server.WatchCall(ctx)
	waitCallSubscribers(t, ctx, rdb, before[call.Channel])

	invite, err := proto.Marshal(&msgpb.CallSignal{
		Op: msgpb.CallOp_CALL_INVITE, PeerId: callee.userID, Media: msgpb.CallMedia_CALL_AUDIO,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.dispatch(caller, &Req{ReqIdentifier: reqSendSignalMsg, Data: invite}); err != nil {
		t.Fatal(err)
	}
	invited := &msgpb.CallSignal{}
	resp := decodeResp(t, caller)
	if err := proto.Unmarshal(resp.Data, invited); err != nil {
		t.Fatal(err)
	}
	if invited.GetToken() != "" || invited.GetUrl() != "" {
		t.Fatalf("invite carried media credentials")
	}
	t.Cleanup(func() {
		_, _ = server.calls.Hangup(context.Background(), caller.userID, invited.GetCallId())
		_, _ = server.calls.Cancel(context.Background(), caller.userID, invited.GetCallId())
	})
	waitCallWrite(t, callee)
	clearWrites(caller)
	clearWrites(callee)

	accept, err := proto.Marshal(&msgpb.CallSignal{Op: msgpb.CallOp_CALL_ACCEPT, CallId: invited.GetCallId()})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.dispatch(callee, &Req{ReqIdentifier: reqSendSignalMsg, Data: accept}); err != nil {
		t.Fatal(err)
	}
	accepted := decodeResp(t, callee)
	if accepted.ErrCode != 0 {
		t.Fatalf("accept code %d msg %s", accepted.ErrCode, accepted.ErrMsg)
	}
	reply := &msgpb.CallSignal{}
	if err := proto.Unmarshal(accepted.Data, reply); err != nil {
		t.Fatal(err)
	}
	if reply.GetOp() != msgpb.CallOp_CALL_ACCEPT || reply.GetUrl() != "ws://127.0.0.1:7880" {
		t.Fatalf("accept reply %+v", reply)
	}
	who, room, err := livekit.Parse(strings.Repeat("s", 32), reply.GetToken())
	if err != nil || who != callee.userID || room != invited.GetCallId() {
		t.Fatalf("callee token identity %s room %s err %v", who, room, err)
	}
	if n := writeCount(callee); n != 1 {
		t.Fatalf("callee frames %d, answered push should skip the accepting socket", n)
	}

	pushed := waitCallSignal(t, caller)
	if pushed.GetOp() != msgpb.CallOp_CALL_ACCEPT || pushed.GetToken() == "" || pushed.GetToken() == reply.GetToken() {
		t.Fatalf("caller push op %s same token %v", pushed.GetOp(), pushed.GetToken() == reply.GetToken())
	}
	who, room, err = livekit.Parse(strings.Repeat("s", 32), pushed.GetToken())
	if err != nil || who != caller.userID || room != invited.GetCallId() || pushed.GetUrl() != reply.GetUrl() {
		t.Fatalf("caller token identity %s room %s err %v", who, room, err)
	}
}

func TestAuthorizeCallForwardsCaller(t *testing.T) {
	var seen string
	server := &Server{relations: callerRelations{seen: &seen}}
	if err := server.authorizeCall(context.Background(), "caller-1", "callee-1"); err != nil {
		t.Fatal(err)
	}
	if seen != "caller-1" {
		t.Fatalf("operator %q", seen)
	}
}

func TestSignalRequiresFriendship(t *testing.T) {
	server := New(Options{Calls: &call.Board{}, Relations: fakeRelations{}})
	client := newTestClient(t, server)
	body, err := proto.Marshal(&msgpb.CallSignal{
		Op: msgpb.CallOp_CALL_INVITE, PeerId: "b", Media: msgpb.CallMedia_CALL_VIDEO,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.dispatch(client, &Req{ReqIdentifier: reqSendSignalMsg, Data: body}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != int(errorcode.MsgNotFriend) {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
}

func TestSignalUnsupportedWithoutRedis(t *testing.T) {
	client := newTestClient(t, New(Options{}))
	if err := client.server.dispatch(client, &Req{ReqIdentifier: reqSendSignalMsg, Data: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	resp := decodeResp(t, client)
	if resp.ErrCode != errArgs || resp.ErrMsg != "signal messages are not supported" {
		t.Fatalf("code %d msg %s", resp.ErrCode, resp.ErrMsg)
	}
}

type callerRelations struct {
	seen *string
}

func (c callerRelations) IsFriend(context.Context, string, string) (bool, bool, error) {
	return true, true, nil
}

func (c callerRelations) IsBlack(ctx context.Context, _, _ string) (bool, bool, error) {
	*c.seen = rpcctx.UserID(ctx)
	return false, false, nil
}

type fakeRelations struct {
	friends bool
	blocked bool
}

func (f fakeRelations) IsFriend(context.Context, string, string) (bool, bool, error) {
	return f.friends, f.friends, nil
}

func (f fakeRelations) IsBlack(context.Context, string, string) (bool, bool, error) {
	return f.blocked, false, nil
}

func clearWrites(client *Client) {
	sock := client.conn.(*memSocket)
	sock.mu.Lock()
	sock.writes = nil
	sock.mu.Unlock()
}

func writeCount(client *Client) int {
	sock := client.conn.(*memSocket)
	sock.mu.Lock()
	defer sock.mu.Unlock()
	return len(sock.writes)
}

func waitCallWrite(t *testing.T, client *Client) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for writeCount(client) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("call push did not arrive")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitCallSignal(t *testing.T, client *Client) *msgpb.CallSignal {
	t.Helper()
	waitCallWrite(t, client)
	sock := client.conn.(*memSocket)
	sock.mu.Lock()
	raw := append([]byte(nil), sock.writes[len(sock.writes)-1]...)
	sock.mu.Unlock()
	var pushed Resp
	if err := client.encoder.Decode(raw, &pushed); err != nil {
		t.Fatal(err)
	}
	if pushed.ReqIdentifier != reqCallSignal {
		t.Fatalf("frame %d", pushed.ReqIdentifier)
	}
	out := &msgpb.CallSignal{}
	if err := proto.Unmarshal(pushed.Data, out); err != nil {
		t.Fatal(err)
	}
	return out
}

func waitCallSubscribers(t *testing.T, ctx context.Context, rdb redis.UniversalClient, before int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		n, err := rdb.PubSubNumSub(ctx, call.Channel).Result()
		if err == nil && n[call.Channel] > before {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscribers %v err=%v", n, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
