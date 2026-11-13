package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
	svcconfig "github.com/lisuhang11/SuIM/services/rtc/internal/config"
)

type memAgentDispatch struct {
	jobs []AgentJob
}

func (m *memAgentDispatch) Dispatch(_ context.Context, job AgentJob) error {
	m.jobs = append(m.jobs, job)
	return nil
}

// JoinOnAccept must carry the linked user's preferred voice, same as bot_invite.
func TestJoinOnAcceptDispatchesPreferredVoice(t *testing.T) {
	rdb := openRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatch := &memAgentDispatch{}
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	svc := New(call.New(rdb), relations{friends: true}, livekit.Config{
		URL: "ws://127.0.0.1:7880", Key: "suim", Secret: secret,
	})
	svc.UseRedis(rdb)
	svc.UseAgents(svcconfig.AgentsConfig{Enabled: true, JoinOnAccept: true, Identity: "agent:assistant"}, dispatch)

	caller, callee := "aj"+strconv.FormatInt(time.Now().UnixNano(), 36), "ak"+strconv.FormatInt(time.Now().UnixNano(), 36)
	voice := "zh-CN-YunxiNeural"
	if err := rdb.Set(ctx, AgentVoiceKeyPrefix+caller, voice, 0).Err(); err != nil {
		t.Fatal(err)
	}
	events := make(chan call.Event, 4)
	before, err := rdb.PubSubNumSub(ctx, call.Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	go call.New(rdb).Subscribe(ctx, func(ev call.Event) { events <- ev })
	waitSubscribers(t, ctx, rdb, before[call.Channel])
	t.Cleanup(func() {
		bg := context.Background()
		_ = rdb.Del(bg, call.Prefix+"busy:"+caller, call.Prefix+"busy:"+callee, AgentVoiceKeyPrefix+caller).Err()
	})

	invited, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller), &rtcpb.SignalMessageAssembleReq{SignalReq: inviteAudio(callee)})
	if err != nil {
		t.Fatal(err)
	}
	room := invited.GetSignalResp().GetInvite().GetRoomId()
	_ = waitOp(t, events, room, call.OpRing)
	if _, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, callee), &rtcpb.SignalMessageAssembleReq{
		ConnId: "accepting", SignalReq: &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Accept{Accept: &rtcpb.SignalAcceptReq{RoomId: room}}},
	}); err != nil {
		t.Fatal(err)
	}
	_ = waitOp(t, events, room, call.OpAccept)
	if len(dispatch.jobs) != 1 {
		t.Fatalf("jobs %d", len(dispatch.jobs))
	}
	job := dispatch.jobs[0]
	if job.Reason != "human_accept" || job.LinkedUserID != caller || job.VoiceID != voice {
		body, _ := json.Marshal(job)
		t.Fatalf("job %s", body)
	}
}

func TestInviteBotAutoAcceptAndDispatch(t *testing.T) {
	rdb := openRedis(t)
	ctx := context.Background()
	bot := "suim-agent"
	caller := "ab" + strconv.FormatInt(time.Now().UnixNano(), 36)
	dispatch := &memAgentDispatch{}
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	svc := New(call.New(rdb), relations{friends: false}, livekit.Config{
		URL: "ws://127.0.0.1:7880", Key: "suim", Secret: secret,
	})
	svc.UseAgents(svcconfig.AgentsConfig{
		Enabled:    true,
		BotUserIDs: []string{bot},
		Identity:   "agent:assistant",
	}, dispatch)

	t.Cleanup(func() {
		bg := context.Background()
		_ = rdb.Del(bg, call.Prefix+"busy:"+caller, call.Prefix+"busy:"+bot).Err()
	})

	invited, err := svc.SignalMessageAssemble(rpcctx.WithUser(ctx, caller), &rtcpb.SignalMessageAssembleReq{
		SignalReq: inviteAudio(bot),
	})
	if err != nil {
		t.Fatal(err)
	}
	accept := invited.GetSignalResp().GetAccept()
	if accept == nil {
		t.Fatal("expected accept reply for bot invite")
	}
	room := accept.GetRoomId()
	if !validRoom(room) {
		t.Fatalf("room %q", room)
	}
	if accept.GetLiveUrl() == "" || accept.GetToken() == "" {
		t.Fatalf("bot invite must return livekit credentials")
	}
	if len(dispatch.jobs) != 1 {
		t.Fatalf("jobs %d", len(dispatch.jobs))
	}
	job := dispatch.jobs[0]
	if job.RoomID != room || job.LinkedUserID != caller || job.CalleeID != bot || job.Reason != "bot_invite" || job.Identity != "agent:assistant" {
		body, _ := json.Marshal(job)
		t.Fatalf("job %s", body)
	}
	kind, err := call.New(rdb).Kind(ctx, room)
	if err != nil || kind != call.KindSingle {
		t.Fatalf("kind %v %v", kind, err)
	}
}
