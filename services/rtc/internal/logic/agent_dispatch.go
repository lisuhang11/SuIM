package logic

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
	svcconfig "github.com/lisuhang11/SuIM/services/rtc/internal/config"
)

const (
	defaultAgentStream   = "suim:agent:dispatch"
	defaultAgentIdentity = "agent:assistant"
	defaultAgentMaxLen   = 10000
)

func agentIdentity(cfg svcconfig.AgentsConfig) string {
	if strings.TrimSpace(cfg.Identity) == "" {
		return defaultAgentIdentity
	}
	return strings.TrimSpace(cfg.Identity)
}

func isAgentBot(cfg svcconfig.AgentsConfig, userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	for _, id := range cfg.BotUserIDs {
		if strings.TrimSpace(id) == userID {
			return true
		}
	}
	return false
}

// AgentJob asks a worker to join one LiveKit room as an assistant.
type AgentJob struct {
	RoomID       string `json:"room_id"`
	Media        string `json:"media"`
	LinkedUserID string `json:"linked_user_id"`
	CallerID     string `json:"caller_id"`
	CalleeID     string `json:"callee_id,omitempty"`
	GroupID      string `json:"group_id,omitempty"`
	Kind         string `json:"kind"`
	Reason       string `json:"reason"`
	Identity     string `json:"identity"`
}

// AgentDispatcher publishes jobs for LiveKit Agent workers.
type AgentDispatcher interface {
	Dispatch(ctx context.Context, job AgentJob) error
}

type redisAgentDispatcher struct {
	rdb    redis.UniversalClient
	stream string
	maxLen int64
}

// NewRedisAgentDispatcher writes jobs to a Redis stream.
func NewRedisAgentDispatcher(rdb redis.UniversalClient, stream string) AgentDispatcher {
	if rdb == nil {
		return nil
	}
	if strings.TrimSpace(stream) == "" {
		stream = defaultAgentStream
	}
	return &redisAgentDispatcher{rdb: rdb, stream: stream, maxLen: defaultAgentMaxLen}
}

func (d *redisAgentDispatcher) Dispatch(ctx context.Context, job AgentJob) error {
	if d == nil || d.rdb == nil {
		return nil
	}
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	args := &redis.XAddArgs{
		Stream: d.stream,
		Values: map[string]any{"body": string(body)},
		MaxLen: d.maxLen,
		Approx: true,
	}
	return d.rdb.XAdd(ctx, args).Err()
}

// UseAgents enables bot auto-answer and LiveKit Agent dispatch.
func (s *Service) UseAgents(cfg svcconfig.AgentsConfig, dispatch AgentDispatcher) {
	if s == nil {
		return
	}
	s.agents = cfg
	s.agentsDispatch = dispatch
}

func (s *Service) dispatchAgent(ctx context.Context, job AgentJob) {
	if s == nil || !s.agents.Enabled || s.agentsDispatch == nil {
		return
	}
	if job.RoomID == "" || job.LinkedUserID == "" {
		return
	}
	if job.Identity == "" {
		job.Identity = agentIdentity(s.agents)
	}
	if job.Kind == "" {
		job.Kind = "single"
	}
	if err := s.agentsDispatch.Dispatch(ctx, job); err != nil {
		slog.Warn("dispatch livekit agent", "err", err, "room", job.RoomID, "reason", job.Reason)
	}
}

// inviteBot answers immediately as the configured AI callee and dispatches an agent worker.
func (s *Service) inviteBot(ctx context.Context, caller, bot, media string) (*rtcpb.SignalMessageAssembleResp, error) {
	if s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	// 1. Create the ringing call, then accept as the bot so the room is live.
	sig, err := s.calls.Invite(ctx, caller, bot, media)
	if err != nil {
		return nil, err
	}
	accepted, err := s.calls.Accept(ctx, bot, sig.CallID)
	if err != nil {
		_, _ = s.calls.Cancel(ctx, caller, sig.CallID)
		return nil, err
	}
	url, token, err := s.sign(caller, accepted.CallID, accepted.Media)
	if err != nil {
		s.abort(ctx, caller, accepted)
		return nil, err
	}
	out := accepted
	out.URL = url
	out.Token = token
	s.publish(ctx, out, call.OpAccept, []string{caller}, "")
	s.dispatchAgent(ctx, AgentJob{
		RoomID:       accepted.CallID,
		Media:        accepted.Media,
		LinkedUserID: caller,
		CallerID:     caller,
		CalleeID:     bot,
		Kind:         "single",
		Reason:       "bot_invite",
		Identity:     agentIdentity(s.agents),
	})
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_Invite{
		Invite: &rtcpb.SignalInviteResp{RoomId: accepted.CallID},
	}}}, nil
}
