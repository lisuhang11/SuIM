package logic

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

func TestGroupVoiceInviteAcceptAndLeave(t *testing.T) {
	rdb := openRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	secret := strings.Repeat("s", 32)
	host, a, b := "gh"+strconv.FormatInt(time.Now().UnixNano(), 36), "ga"+strconv.FormatInt(time.Now().UnixNano(), 36), "gb"+strconv.FormatInt(time.Now().UnixNano(), 36)
	groupID := "gg" + host
	svc := New(call.New(rdb), relations{friends: true}, livekit.Config{URL: "ws://127.0.0.1:7880", Key: "suim", Secret: secret})
	svc.UseGroups(groupSet{members: map[string]map[string]bool{groupID: {host: true, a: true, b: true}}})
	events := make(chan call.Event, 8)
	before, err := rdb.PubSubNumSub(ctx, call.Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	go call.New(rdb).Subscribe(ctx, func(ev call.Event) { events <- ev })
	waitSubscribers(t, ctx, rdb, before[call.Channel])
	t.Cleanup(func() {
		bg := context.Background()
		_ = rdb.Del(bg, call.Prefix+"busy:"+host, call.Prefix+"busy:"+a, call.Prefix+"busy:"+b, call.Prefix+"group:"+groupID).Err()
	})

	invited, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, host), &rtcpb.SignalMessageAssembleReq{
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_InviteInGroup{InviteInGroup: &rtcpb.SignalInviteInGroupReq{
			GroupId: groupID, InviteeUserIds: []string{a, b}, MediaType: call.MediaAudio,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	room := invited.GetSignalResp().GetInviteInGroup()
	who, gotRoom, err := livekit.Parse(secret, room.GetToken())
	sources, serr := livekit.Sources(secret, room.GetToken())
	if err != nil || serr != nil || who != host || gotRoom != room.GetRoomId() || len(sources) != 1 || sources[0] != "microphone" {
		t.Fatalf("host token %s %s %v %v %v", who, gotRoom, sources, err, serr)
	}
	if !sameIDs(room.GetInviteeUserIds(), []string{a, b}) {
		t.Fatalf("ringing %v", room.GetInviteeUserIds())
	}
	ring := waitOp(t, events, room.GetRoomId(), call.OpRing)
	if ring.Token != "" || ring.Group != groupID || !sameIDs(ring.Users, []string{a, b}) {
		t.Fatalf("ring %+v", ring)
	}

	_, err = svc.SignalMessageAssemble(rpcctx.WithUser(ctx, "outsider"), &rtcpb.SignalMessageAssembleReq{
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_InviteInGroup{InviteInGroup: &rtcpb.SignalInviteInGroupReq{
			GroupId: groupID, InviteeUserIds: []string{a}, MediaType: call.MediaAudio,
		}}},
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.GroupNotMember {
		t.Fatalf("outsider %v", err)
	}

	accepted, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, a), &rtcpb.SignalMessageAssembleReq{
		ConnId:    "accepting",
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Accept{Accept: &rtcpb.SignalAcceptReq{RoomId: room.GetRoomId()}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	accept := accepted.GetSignalResp().GetAccept()
	if who, _, err = livekit.Parse(secret, accept.GetToken()); err != nil || who != a || accept.GetGroupId() != groupID {
		t.Fatalf("accept token %s %v %+v", who, err, accept)
	}
	joined := waitOp(t, events, room.GetRoomId(), call.OpJoined)
	if !sameIDs(joined.Users, []string{host, b}) && !sameIDs(joined.Users, []string{host}) {
		t.Fatalf("joined notify %+v", joined)
	}
	if answered := waitOp(t, events, room.GetRoomId(), call.OpAnswered); answered.Except != "accepting" || !sameIDs(answered.Users, []string{a}) {
		t.Fatalf("answered %+v", answered)
	}

	if _, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, a), &rtcpb.SignalMessageAssembleReq{
		SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_HangUp{HangUp: &rtcpb.SignalHangUpReq{RoomId: room.GetRoomId()}}},
	}); err != nil {
		t.Fatal(err)
	}
	left := waitOp(t, events, room.GetRoomId(), call.OpLeft)
	if left.Actor != a {
		t.Fatalf("left %+v", left)
	}
}

type groupSet struct {
	members map[string]map[string]bool
}

func (g groupSet) InGroup(_ context.Context, groupID, userID string) (bool, error) {
	return g.members[groupID][userID], nil
}

func sameIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		if seen[id] == 0 {
			return false
		}
		seen[id]--
	}
	return true
}
