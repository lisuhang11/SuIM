package service

import (
	"context"
	"testing"
	"time"

	"rtc/internal/config"
	apperrors "rtc/internal/errors"
	rtcnotif "rtc/internal/notification"
	"rtc/internal/types"

	messagepb "SuIM/proto/messagepb"
)

type callRepoStub struct {
	calls      map[string]*types.Call
	createErr  error
	updateErr  error
	findActive map[string]*types.Call
}

func (r *callRepoStub) Create(_ context.Context, call *types.Call) error {
	if r.createErr != nil {
		return r.createErr
	}
	if r.calls == nil {
		r.calls = map[string]*types.Call{}
	}
	cp := *call
	r.calls[call.CallID] = &cp
	return nil
}

func (r *callRepoStub) GetByID(_ context.Context, callID string) (*types.Call, error) {
	if r.calls == nil {
		return nil, nil
	}
	call, ok := r.calls[callID]
	if !ok {
		return nil, nil
	}
	cp := *call
	return &cp, nil
}

func (r *callRepoStub) Update(_ context.Context, call *types.Call) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if r.calls == nil {
		r.calls = map[string]*types.Call{}
	}
	cp := *call
	r.calls[call.CallID] = &cp
	return nil
}

func (r *callRepoStub) FindActiveByUser(_ context.Context, userID string) (*types.Call, error) {
	if r.findActive != nil {
		if call, ok := r.findActive[userID]; ok {
			cp := *call
			return &cp, nil
		}
	}
	for _, call := range r.calls {
		if call.CallerID != userID && call.CalleeID != userID {
			continue
		}
		switch call.Status {
		case types.CallStatusRinging, types.CallStatusAccepted, types.CallStatusActive:
			cp := *call
			return &cp, nil
		}
	}
	return nil, nil
}

type friendCheckerStub struct {
	mutual bool
	err    error
}

func (f *friendCheckerStub) IsMutualFriend(context.Context, string, string) (bool, error) {
	return f.mutual, f.err
}

type presenceCheckerStub struct {
	online map[string]bool
	err    error
}

func (p *presenceCheckerStub) IsUserOnline(_ context.Context, userID string) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	return p.online[userID], nil
}

type timelineWriterStub struct {
	count int
}

func (t *timelineWriterStub) WriteCallTimeline(context.Context, *types.Call) error {
	t.count++
	return nil
}

type offlinePusherStub struct{}

func (offlinePusherStub) PushCallSignal(context.Context, *types.Call, string, ...string) error {
	return nil
}

func newTestCallService(repo *callRepoStub, friends *friendCheckerStub, presence *presenceCheckerStub) *callService {
	notifier := rtcnotif.NewCallNotificationSender(func(context.Context, string, *messagepb.MsgData) error {
		return nil
	})
	return &callService{
		repo:             repo,
		notifier:         notifier,
		friends:          friends,
		presence:         presence,
		timeline:         &timelineWriterStub{},
		offline:          offlinePusherStub{},
		liveKitURL:       "ws://localhost:7880",
		liveKitAPIKey:    "devkey",
		liveKitAPISecret: "secret",
		ringTimeoutSec:   45,
	}
}

func TestSingleChatID(t *testing.T) {
	if got := singleChatID("bob", "alice"); got != "si_alice_bob" {
		t.Fatalf("singleChatID() = %q, want si_alice_bob", got)
	}
	if got := singleChatID("alice", "bob"); got != "si_alice_bob" {
		t.Fatalf("singleChatID() = %q, want si_alice_bob", got)
	}
}

func TestInviteUnavailable(t *testing.T) {
	repo := &callRepoStub{calls: map[string]*types.Call{}}
	svc := newTestCallService(repo, &friendCheckerStub{mutual: true}, &presenceCheckerStub{online: map[string]bool{"callee": false}})

	_, _, err := svc.Invite(context.Background(), "caller", "callee", "audio")
	if apperrors.GetAppError(err).Code != apperrors.CodeUnavailable {
		t.Fatalf("Invite() error = %v, want unavailable", err)
	}
	for _, call := range repo.calls {
		if call.EndReason != types.EndReasonUnavailable || call.Status != types.CallStatusEnded {
			t.Fatalf("unexpected ended call: %+v", call)
		}
	}
}

func TestInviteBusy(t *testing.T) {
	repo := &callRepoStub{
		calls: map[string]*types.Call{
			"active-1": {
				CallID:   "active-1",
				CallerID: "other",
				CalleeID: "callee",
				Status:   types.CallStatusRinging,
			},
		},
	}
	svc := newTestCallService(repo, &friendCheckerStub{mutual: true}, &presenceCheckerStub{online: map[string]bool{"callee": true}})

	_, _, err := svc.Invite(context.Background(), "caller", "callee", "audio")
	if apperrors.GetAppError(err).Code != apperrors.CodeBusy {
		t.Fatalf("Invite() error = %v, want busy", err)
	}
	var busyCount int
	for _, call := range repo.calls {
		if call.EndReason == types.EndReasonBusy {
			busyCount++
		}
	}
	if busyCount != 1 {
		t.Fatalf("expected one busy call record, got %d", busyCount)
	}
}

func TestInviteAcceptHappyPath(t *testing.T) {
	repo := &callRepoStub{calls: map[string]*types.Call{}}
	svc := newTestCallService(repo, &friendCheckerStub{mutual: true}, &presenceCheckerStub{online: map[string]bool{"callee": true}})

	call, token, err := svc.Invite(context.Background(), "caller", "callee", "audio")
	if err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	if token == "" {
		t.Fatal("Invite() token is empty")
	}
	if call.Status != types.CallStatusRinging {
		t.Fatalf("Invite() status = %q, want ringing", call.Status)
	}

	accepted, calleeToken, err := svc.Accept(context.Background(), "callee", call.CallID)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if calleeToken == "" {
		t.Fatal("Accept() token is empty")
	}
	if accepted.Status != types.CallStatusAccepted {
		t.Fatalf("Accept() status = %q, want accepted", accepted.Status)
	}
	if accepted.AnsweredAt == 0 {
		t.Fatal("Accept() answered_at is zero")
	}
}

func TestInviteRejectsVideo(t *testing.T) {
	repo := &callRepoStub{calls: map[string]*types.Call{}}
	svc := newTestCallService(repo, &friendCheckerStub{mutual: true}, &presenceCheckerStub{online: map[string]bool{"callee": true}})

	_, _, err := svc.Invite(context.Background(), "caller", "callee", "video")
	if apperrors.GetAppError(err).Code != apperrors.CodeValidation {
		t.Fatalf("Invite(video) error = %v, want validation", err)
	}
}

func TestTimeoutTransition(t *testing.T) {
	repo := &callRepoStub{calls: map[string]*types.Call{}}
	svc := newTestCallService(repo, &friendCheckerStub{mutual: true}, &presenceCheckerStub{online: map[string]bool{"callee": true}})
	svc.ringTimeoutSec = 1

	call, _, err := svc.Invite(context.Background(), "caller", "callee", "")
	if err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	svc.handleTimeout(context.Background(), call.CallID)

	updated, err := repo.GetByID(context.Background(), call.CallID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if updated.Status != types.CallStatusEnded || updated.EndReason != types.EndReasonTimeout {
		t.Fatalf("timeout call = %+v, want ended/timeout", updated)
	}
}

func TestNewCallServiceConfig(t *testing.T) {
	cfg := &config.Config{
		LiveKitURL:       "ws://test",
		LiveKitAPIKey:    "k",
		LiveKitAPISecret: "s",
		RingTimeoutSec:   30,
	}
	repo := &callRepoStub{}
	svc := NewCallService(repo, cfg, rtcnotif.NewCallNotificationSender(nil), nil, nil, nil, nil).(*callService)
	if svc.liveKitURL != "ws://test" || svc.ringTimeoutSec != 30 {
		t.Fatalf("unexpected service config: %+v", svc)
	}
}
