package logic

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

func TestPrivateVoiceInviteAcceptHangup(t *testing.T) {
	rdb := openRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	secret := strings.Repeat("s", 32)
	svc := New(call.New(rdb), relations{friends: true}, livekit.Config{
		URL: "ws://127.0.0.1:7880", Key: "suim", Secret: secret,
	})
	events := make(chan call.Event, 8)
	before, err := rdb.PubSubNumSub(ctx, call.Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	go call.New(rdb).Subscribe(ctx, func(ev call.Event) { events <- ev })
	waitSubscribers(t, ctx, rdb, before[call.Channel])

	caller, callee := "va"+strconv.FormatInt(time.Now().UnixNano(), 36), "vb"+strconv.FormatInt(time.Now().UnixNano(), 36)
	var room string
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = call.New(rdb).Cancel(bg, caller, room)
		_, _ = call.New(rdb).Hangup(bg, caller, room)
		_ = rdb.Del(bg, call.Prefix+"busy:"+caller, call.Prefix+"busy:"+callee, call.Prefix+"busy:"+caller+"x").Err()
	})

	invited, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller), &rtcpb.SignalMessageAssembleReq{
		SignalReq: inviteAudio(callee),
	})
	if err != nil {
		t.Fatal(err)
	}
	room = invited.GetSignalResp().GetInvite().GetRoomId()
	if invited.GetSignalResp().GetInvite().GetToken() != "" || invited.GetSignalResp().GetInvite().GetLiveUrl() != "" || !validRoom(room) {
		t.Fatalf("invite %+v", invited.GetSignalResp().GetInvite())
	}
	ring := waitOp(t, events, room, call.OpRing)
	if len(ring.Users) != 1 || ring.Users[0] != callee || ring.Caller != caller || ring.Token != "" {
		t.Fatalf("ring %+v", ring)
	}

	_, err = svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller+"x"), &rtcpb.SignalMessageAssembleReq{
		SignalReq: inviteAudio(callee),
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.CallPeerBusy {
		t.Fatalf("busy %v", err)
	}
	_, err = svc.SignalGetTokenByRoomID(rpcctx.WithUser(ctx, caller), &rtcpb.SignalGetTokenByRoomIDReq{RoomId: room})
	if code, _ := errorcode.CodeOf(err); code != errorcode.CallState {
		t.Fatalf("token before accept %v", err)
	}

	accepted, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, callee), &rtcpb.SignalMessageAssembleReq{
		ConnId: "accepting", SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Accept{Accept: &rtcpb.SignalAcceptReq{RoomId: room}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	accept := accepted.GetSignalResp().GetAccept()
	who, gotRoom, err := livekit.Parse(secret, accept.GetToken())
	if err != nil || who != callee || gotRoom != room || accept.GetLiveUrl() != "ws://127.0.0.1:7880" {
		t.Fatalf("callee token %s room %s url %s err %v", who, gotRoom, accept.GetLiveUrl(), err)
	}
	callerEvent := waitOp(t, events, room, call.OpAccept)
	answered := waitOp(t, events, room, call.OpAnswered)
	who, gotRoom, err = livekit.Parse(secret, callerEvent.Token)
	if err != nil || who != caller || gotRoom != room || callerEvent.Token == accept.GetToken() || callerEvent.URL != accept.GetLiveUrl() {
		t.Fatalf("caller push %+v err %v", callerEvent, err)
	}
	if answered.Except != "accepting" || answered.Token != "" || len(answered.Users) != 1 || answered.Users[0] != callee {
		t.Fatalf("answered %+v", answered)
	}

	refreshed, err := svc.SignalGetTokenByRoomID(rpcctx.WithUser(ctx, caller), &rtcpb.SignalGetTokenByRoomIDReq{RoomId: room, UserId: caller})
	if err != nil {
		t.Fatal(err)
	}
	who, _, err = livekit.Parse(secret, refreshed.GetToken())
	if err != nil || who != caller || refreshed.GetLiveUrl() != accept.GetLiveUrl() {
		t.Fatalf("refresh %s err %v", who, err)
	}
	_, err = svc.SignalGetTokenByRoomID(rpcctx.WithUser(ctx, "outsider"), &rtcpb.SignalGetTokenByRoomIDReq{RoomId: room})
	if code, _ := errorcode.CodeOf(err); code != errorcode.CallNotMember {
		t.Fatalf("outsider %v", err)
	}

	if _, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller), &rtcpb.SignalMessageAssembleReq{
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_HangUp{HangUp: &rtcpb.SignalHangUpReq{RoomId: room}}},
	}); err != nil {
		t.Fatal(err)
	}
	hung := waitOp(t, events, room, call.OpHangup)
	if len(hung.Users) != 1 || hung.Users[0] != callee || hung.Token != "" {
		t.Fatalf("hangup %+v", hung)
	}
}

func TestPrivateVoiceCancelAndReject(t *testing.T) {
	rdb := openRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := New(call.New(rdb), relations{friends: true}, livekit.Config{})
	events := make(chan call.Event, 4)
	before, err := rdb.PubSubNumSub(ctx, call.Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	go call.New(rdb).Subscribe(ctx, func(ev call.Event) { events <- ev })
	waitSubscribers(t, ctx, rdb, before[call.Channel])

	caller, callee := "vc"+strconv.FormatInt(time.Now().UnixNano(), 36), "vd"+strconv.FormatInt(time.Now().UnixNano(), 36)
	t.Cleanup(func() {
		bg := context.Background()
		_ = rdb.Del(bg, call.Prefix+"busy:"+caller, call.Prefix+"busy:"+callee).Err()
	})
	invited, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller), &rtcpb.SignalMessageAssembleReq{SignalReq: inviteAudio(callee)})
	if err != nil {
		t.Fatal(err)
	}
	room := invited.GetSignalResp().GetInvite().GetRoomId()
	_ = waitOp(t, events, room, call.OpRing)
	if _, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, callee), &rtcpb.SignalMessageAssembleReq{
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Reject{Reject: &rtcpb.SignalRejectReq{RoomId: room}}},
	}); err != nil {
		t.Fatal(err)
	}
	rejected := waitOp(t, events, room, call.OpReject)
	if len(rejected.Users) != 1 || rejected.Users[0] != caller {
		t.Fatalf("reject %+v", rejected)
	}

	invited, err = svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller), &rtcpb.SignalMessageAssembleReq{SignalReq: inviteAudio(callee)})
	if err != nil {
		t.Fatal(err)
	}
	room = invited.GetSignalResp().GetInvite().GetRoomId()
	_ = waitOp(t, events, room, call.OpRing)
	if _, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller), &rtcpb.SignalMessageAssembleReq{
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Cancel{Cancel: &rtcpb.SignalCancelReq{RoomId: room}}},
	}); err != nil {
		t.Fatal(err)
	}
	cancelled := waitOp(t, events, room, call.OpCancel)
	if len(cancelled.Users) != 1 || cancelled.Users[0] != callee {
		t.Fatalf("cancel %+v", cancelled)
	}
}

func TestInviteChecksOperatorFriendshipAndVoice(t *testing.T) {
	var seen string
	svc := New(call.New(nil), relations{seen: &seen}, livekit.Config{})
	_, err := svc.SignalMessageAssemble(context.Background(), &rtcpb.SignalMessageAssembleReq{SignalReq: inviteAudio("callee-1")})
	if code, _ := errorcode.CodeOf(err); code != errorcode.NoPermission || seen != "" {
		t.Fatalf("anonymous code %v seen %q", err, seen)
	}

	_, err = svc.SignalMessageAssemble(rpcctx.WithUser(context.Background(), "caller-1"), &rtcpb.SignalMessageAssembleReq{
		SignalReq: inviteAudio("callee-1"),
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.MsgNotFriend || seen != "caller-1" {
		t.Fatalf("friend code %v seen %q", err, seen)
	}

	blocked := New(call.New(nil), relations{friends: true, blocked: true}, livekit.Config{})
	_, err = blocked.SignalMessageAssemble(rpcctx.WithUser(context.Background(), "caller-1"), &rtcpb.SignalMessageAssembleReq{
		SignalReq: inviteAudio("callee-1"),
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.MsgBlocked || err.Error() != "blocked" {
		t.Fatalf("blocked %v", err)
	}

	failed := New(call.New(nil), relations{friends: true, fail: true}, livekit.Config{})
	_, err = failed.SignalMessageAssemble(rpcctx.WithUser(context.Background(), "caller-1"), &rtcpb.SignalMessageAssembleReq{
		SignalReq: inviteAudio("callee-1"),
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.Internal || err.Error() != "check black" {
		t.Fatalf("check %v", err)
	}

	_, err = New(nil, nil, livekit.Config{}).SignalMessageAssemble(rpcctx.WithUser(context.Background(), "caller-1"), &rtcpb.SignalMessageAssembleReq{
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Invite{Invite: &rtcpb.SignalInviteReq{
			Invitation: &rtcpb.InvitationInfo{InviteeUserId: "callee-1", MediaType: call.MediaVideo},
		}}},
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.CallInvalid || err.Error() != "voice calls only" {
		t.Fatalf("video %v", err)
	}
}

type relations struct {
	friends bool
	blocked bool
	fail    bool
	seen    *string
}

func (r relations) IsFriend(context.Context, string, string) (bool, bool, error) {
	return r.friends, r.friends, nil
}

func (r relations) IsBlack(ctx context.Context, _, _ string) (bool, bool, error) {
	if r.seen != nil {
		*r.seen = rpcctx.UserID(ctx)
	}
	if r.fail {
		return false, false, errors.New("down")
	}
	return r.blocked, false, nil
}

func inviteAudio(callee string) *rtcpb.SignalReq {
	return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Invite{Invite: &rtcpb.SignalInviteReq{
		Invitation: &rtcpb.InvitationInfo{InviteeUserId: callee, MediaType: call.MediaAudio},
	}}}
}

func validRoom(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func waitOp(t *testing.T, events <-chan call.Event, callID, op string) call.Event {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.CallID == callID && ev.Op == op {
				return ev
			}
		case <-deadline:
			t.Fatalf("missing %s for %s", op, callID)
		}
	}
}

func waitSubscribers(t *testing.T, ctx context.Context, rdb redis.UniversalClient, before int64) {
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

func openRedis(t *testing.T) redis.UniversalClient {
	t.Helper()
	env := map[string]string{}
	b, err := os.ReadFile("../../../../.env")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		env[k] = v
	}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: env["REDIS_PASSWORD"]})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}
