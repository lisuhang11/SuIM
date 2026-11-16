package logic

import (
	"context"
	"testing"
	"time"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

type fakeMembers struct {
	ids []string
	err error
}

func (f fakeMembers) UserIDs(context.Context, string, string) ([]string, error) {
	return f.ids, f.err
}

type fakeOnline struct {
	platforms map[string][]int32
}

func (f fakeOnline) Get(_ context.Context, userID string) ([]int32, error) {
	return f.platforms[userID], nil
}

type fakePusher struct {
	userIDs []string
	title   string
	content string
	calls   int
}

func (f *fakePusher) Push(_ context.Context, userIDs []string, title, content string, _ *options.Opts) error {
	f.calls++
	f.userIDs = append([]string(nil), userIDs...)
	f.title = title
	f.content = content
	return nil
}

type fakeTokens struct{}

func (fakeTokens) FCMTokens(context.Context, string) ([]string, error) { return nil, nil }
func (fakeTokens) Badge(context.Context, string) (int64, error)        { return 0, nil }
func (fakeTokens) IncrBadge(context.Context, string, int64) (int64, error) {
	return 1, nil
}
func (fakeTokens) GetuiToken(context.Context) (string, error)                 { return "", nil }
func (fakeTokens) SetGetuiToken(context.Context, string, time.Duration) error { return nil }
func (fakeTokens) SetFCMToken(context.Context, string, int32, string) error   { return nil }
func (fakeTokens) DelFCMToken(context.Context, string, int32) error           { return nil }
func (fakeTokens) SetBadge(context.Context, string, int64) error              { return nil }

func TestDispatchSingleChatOffline(t *testing.T) {
	pusher := &fakePusher{}
	srv := New(fakeMembers{}, fakeOnline{}, pusher, fakeTokens{})
	body := &msgpb.PushMsgData{MsgData: &msgpb.MsgData{
		SendId: "alice", RecvId: "bob", ContentType: 101,
	}}
	if err := srv.dispatch(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if pusher.calls != 1 || len(pusher.userIDs) != 1 || pusher.userIDs[0] != "bob" {
		t.Fatalf("push users = %v, calls = %d", pusher.userIDs, pusher.calls)
	}
	if pusher.title != "[Message]" || pusher.content != "[Message]" {
		t.Fatalf("title/content = %q/%q", pusher.title, pusher.content)
	}
}

func TestDispatchSkipsOnlineReceiver(t *testing.T) {
	pusher := &fakePusher{}
	online := fakeOnline{platforms: map[string][]int32{"bob": {1}}}
	srv := New(fakeMembers{}, online, pusher, fakeTokens{})
	body := &msgpb.PushMsgData{MsgData: &msgpb.MsgData{SendId: "alice", RecvId: "bob", ContentType: 101}}
	if err := srv.dispatch(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if pusher.calls != 0 {
		t.Fatalf("unexpected push: %v", pusher.userIDs)
	}
}

func TestDispatchRespectsOfflinePushOption(t *testing.T) {
	pusher := &fakePusher{}
	srv := New(fakeMembers{}, fakeOnline{}, pusher, fakeTokens{})
	body := &msgpb.PushMsgData{MsgData: &msgpb.MsgData{
		SendId: "alice", RecvId: "bob", ContentType: 101,
		Options: map[string]bool{optOfflinePush: false},
	}}
	if err := srv.dispatch(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if pusher.calls != 0 {
		t.Fatal("offline push should be skipped")
	}
}

func TestDispatchSkipsTyping(t *testing.T) {
	pusher := &fakePusher{}
	srv := New(fakeMembers{}, fakeOnline{}, pusher, fakeTokens{})
	body := &msgpb.PushMsgData{MsgData: &msgpb.MsgData{
		SendId: "alice", RecvId: "bob", ContentType: contentTyping,
	}}
	if err := srv.dispatch(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if pusher.calls != 0 {
		t.Fatal("typing notification should be skipped")
	}
}

func TestDispatchGroupFiltersOnlineAndSender(t *testing.T) {
	pusher := &fakePusher{}
	members := fakeMembers{ids: []string{"alice", "bob", "carol", "dave"}}
	online := fakeOnline{platforms: map[string][]int32{"bob": {2}, "dave": {1}}}
	srv := New(members, online, pusher, fakeTokens{})
	body := &msgpb.PushMsgData{MsgData: &msgpb.MsgData{
		SendId: "alice", GroupId: "g1", ContentType: 101,
	}}
	if err := srv.dispatch(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if pusher.calls != 1 || len(pusher.userIDs) != 1 || pusher.userIDs[0] != "carol" {
		t.Fatalf("push users = %v, calls = %d", pusher.userIDs, pusher.calls)
	}
}
