package logic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msg/internal/store"
)

func TestSingleSendRequiresFriends(t *testing.T) {
	prod := &fakeProducer{}
	srv := New(nil, prod, &fakeRelations{friend1: true}, nil)
	_, err := srv.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
		SendId: "a", RecvId: "b", SessionType: 1, Content: []byte("hi"), ClientMsgId: "c1",
	}})
	if code, _ := errorcode.CodeOf(err); code != errorcode.MsgNotFriend {
		t.Fatalf("code %d err %v", code, err)
	}
	if prod.n != 0 {
		t.Fatalf("published %d", prod.n)
	}
}

func TestSingleSendRejectsBlock(t *testing.T) {
	srv := New(nil, &fakeProducer{}, &fakeRelations{friend1: true, friend2: true, black2: true}, nil)
	_, err := srv.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
		SendId: "a", RecvId: "b", SessionType: 1, Content: []byte("hi"),
	}})
	if code, _ := errorcode.CodeOf(err); code != errorcode.MsgBlocked {
		t.Fatalf("code %d err %v", code, err)
	}
}

func TestNotificationSkipsRelationCheck(t *testing.T) {
	prod := &fakeProducer{}
	srv := New(nil, prod, &fakeRelations{}, nil)
	resp, err := srv.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
		SendId: "a", RecvId: "a", SessionType: 1, Content: []byte("hi"), ClientMsgId: "c1",
		Options: map[string]bool{"isNotNotification": false},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ClientMsgId != "c1" || prod.n != 1 || prod.key != "n_a_a" {
		t.Fatalf("publish n=%d key=%s resp=%+v", prod.n, prod.key, resp)
	}
}

func TestSingleSendPublishes(t *testing.T) {
	prod := &fakeProducer{}
	srv := New(nil, prod, &fakeRelations{friend1: true, friend2: true}, nil)
	resp, err := srv.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
		SendId: "a", RecvId: "b", SessionType: 1, Content: []byte("hi"), ClientMsgId: "c1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ServerMsgId == "" || resp.ClientMsgId != "c1" || resp.SendTime == 0 {
		t.Fatalf("resp %+v", resp)
	}
	if prod.n != 1 || prod.key != "si_a_b" {
		t.Fatalf("publish n=%d key=%s", prod.n, prod.key)
	}
}

func TestBeforeSendRejectsAndModifyRewrites(t *testing.T) {
	reject := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(webhook.CommonCallbackResp{ActionCode: webhook.ActionForbidden, ErrMsg: "no"})
	}))
	defer reject.Close()
	client := webhook.NewWebhookClient(reject.URL)
	defer client.Close()
	prod := &fakeProducer{}
	srv := New(nil, prod, &fakeRelations{friend1: true, friend2: true}, nil)
	srv.UseHooks(Hooks{Client: client, BeforeSendSingle: MsgBefore{BeforeConfig: webhook.BeforeConfig{Enable: true, Timeout: 2}}})
	_, err := srv.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
		SendId: "a", RecvId: "b", SessionType: 1, Content: []byte("hi"), ClientMsgId: "c1",
	}})
	if !errors.Is(err, errorcode.ErrWebhookRejected) || prod.n != 0 {
		t.Fatalf("err=%v published=%d", err, prod.n)
	}

	modify := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/callbackBeforeMsgModifyCommand" {
			_, _ = w.Write([]byte(`{"actionCode":0,"content":"edited"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(webhook.CommonCallbackResp{})
	}))
	defer modify.Close()
	client = webhook.NewWebhookClient(modify.URL)
	defer client.Close()
	prod = &fakeProducer{}
	srv = New(nil, prod, &fakeRelations{friend1: true, friend2: true}, nil)
	srv.UseHooks(Hooks{
		Client:           client,
		BeforeSendSingle: MsgBefore{BeforeConfig: webhook.BeforeConfig{Enable: true, Timeout: 2}},
		BeforeMsgModify:  MsgBefore{BeforeConfig: webhook.BeforeConfig{Enable: true, Timeout: 2}},
	})
	if _, err := srv.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
		SendId: "a", RecvId: "b", SessionType: 1, Content: []byte("hi"), ClientMsgId: "c1",
	}}); err != nil {
		t.Fatal(err)
	}
	stored := &msgpb.MsgData{}
	if err := proto.Unmarshal(prod.body, stored); err != nil {
		t.Fatal(err)
	}
	if string(stored.Content) != "edited" {
		t.Fatalf("content %q", stored.Content)
	}
}

func TestGroupSendHonorsMute(t *testing.T) {
	future := time.Now().Add(time.Hour).UnixMilli()
	prod := &fakeProducer{}
	groups := &fakeGroups{speakers: map[string]GroupSpeaker{
		"g:e": {Found: true, InGroup: true, Role: 20, MuteEndMs: future},
		"g:a": {Found: true, InGroup: true, Role: 100, MuteEndMs: future},
		"g:b": {Found: true, InGroup: true, Status: 3, Role: 60},
		"g:d": {Found: true, InGroup: true, Status: 3, Role: 20},
		"g:x": {Found: true},
	}}
	srv := New(nil, prod, nil, groups)
	send := func(user string, notice bool) error {
		msg := &msgpb.MsgData{SendId: user, GroupId: "g", SessionType: 2, Content: []byte("hi"), ClientMsgId: user}
		if notice {
			msg.Options = map[string]bool{"isNotNotification": false}
		}
		_, err := srv.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: msg})
		return err
	}
	if code, _ := errorcode.CodeOf(send("e", false)); code != errorcode.MsgMutedInGroup {
		t.Fatalf("member mute code %d", code)
	}
	if code, _ := errorcode.CodeOf(send("d", false)); code != errorcode.MsgMutedGroup {
		t.Fatalf("group mute code %d", code)
	}
	if code, _ := errorcode.CodeOf(send("x", false)); code != errorcode.MsgNotInGroup {
		t.Fatalf("outsider code %d", code)
	}
	if err := send("a", false); err != nil {
		t.Fatal(err)
	}
	if err := send("b", false); err != nil {
		t.Fatal(err)
	}
	if err := send("e", true); err != nil {
		t.Fatal(err)
	}
	if prod.n != 3 || prod.key != "n_g" {
		t.Fatalf("publish n=%d key=%s", prod.n, prod.key)
	}

	bare := New(nil, &fakeProducer{}, nil, nil)
	_, err := bare.SendMsg(context.Background(), &msgpb.SendMsgReq{MsgData: &msgpb.MsgData{
		SendId: "a", GroupId: "g", SessionType: 3, Content: []byte("hi"),
	}})
	if code, _ := errorcode.CodeOf(err); code != errorcode.Internal {
		t.Fatalf("missing group service code %d err %v", code, err)
	}
}

func TestPullLatestPage(t *testing.T) {
	hist := &fakeHistory{msgs: []*msgpb.MsgData{{
		Seq: 1, SendId: "a", RecvId: "b", Content: []byte(`{"content":"111"}`),
	}}, max: 1}
	srv := New(hist, nil, nil, nil)
	resp, err := srv.PullMessageBySeqs(context.Background(), &msgpb.PullMessageBySeqsReq{
		UserId:    "a",
		SeqRanges: []*msgpb.SeqRange{{ConversationId: "si_a_b", Num: 20}},
	})
	if err != nil {
		t.Fatal(err)
	}
	pull := resp.Msgs["si_a_b"]
	if pull == nil || !pull.IsEnd || pull.MaxSeq != 1 || len(pull.Msgs) != 1 {
		t.Fatalf("pull %+v", pull)
	}
	if hist.begin != 1 || hist.end != 1 {
		t.Fatalf("range %d %d", hist.begin, hist.end)
	}
}

func TestPullRejectsStranger(t *testing.T) {
	srv := New(&fakeHistory{max: 1}, nil, nil, nil)
	_, err := srv.PullMessageBySeqs(context.Background(), &msgpb.PullMessageBySeqsReq{
		UserId:    "c",
		SeqRanges: []*msgpb.SeqRange{{ConversationId: "si_a_b", Begin: 1, End: 1, Num: 20}},
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.NoPermission {
		t.Fatalf("code %d err %v", code, err)
	}
}

func TestRangeIsEnd(t *testing.T) {
	if rangeIsEnd(0, 1, 2, 3) {
		t.Fatal("ascending pull is not finished")
	}
	if !rangeIsEnd(0, 1, 3, 3) {
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

type fakeGroups struct {
	speakers map[string]GroupSpeaker
}

func (f *fakeGroups) Joined(context.Context, string) ([]JoinedGroup, error) { return nil, nil }

func (f *fakeGroups) Speaker(_ context.Context, groupID, userID string) (GroupSpeaker, error) {
	if f.speakers == nil {
		return GroupSpeaker{}, nil
	}
	return f.speakers[groupID+":"+userID], nil
}

type fakeProducer struct {
	n    int
	key  string
	body []byte
}

func (f *fakeProducer) Send(_ context.Context, key string, body []byte) error {
	f.n++
	f.key = key
	f.body = append([]byte(nil), body...)
	stored := &msgpb.MsgData{}
	_ = proto.Unmarshal(body, stored)
	return nil
}

type fakeHistory struct {
	msgs       []*msgpb.MsgData
	max        int64
	min        int64
	read       int64
	seqTime    int64
	pinned     []string
	begin, end int64
}

func (f *fakeHistory) ConversationIDs(context.Context, string) ([]string, error) { return nil, nil }
func (f *fakeHistory) MaxSeqs(context.Context, []string) (map[string]int64, error) {
	return map[string]int64{"si_a_b": f.max}, nil
}
func (f *fakeHistory) SeqState(_ context.Context, _ string, ids []string) (map[string]store.SeqState, error) {
	if f.seqTime == 0 && f.read == 0 {
		return nil, nil
	}
	out := map[string]store.SeqState{}
	for _, id := range ids {
		out[id] = store.SeqState{MaxSeq: f.max, HasReadSeq: f.read, MaxSeqTime: f.seqTime}
	}
	return out, nil
}
func (f *fakeHistory) SeqBounds(context.Context, string, string) (int64, int64, error) {
	minSeq := f.min
	if minSeq <= 0 && f.max > 0 {
		minSeq = 1
	}
	return minSeq, f.max, nil
}
func (f *fakeHistory) PinnedConversationIDs(context.Context, string) ([]string, error) {
	return f.pinned, nil
}
func (f *fakeHistory) Range(_ context.Context, _ string, begin, end, _ int64) ([]*msgpb.MsgData, error) {
	f.begin, f.end = begin, end
	return f.msgs, nil
}
func (f *fakeHistory) BySeqs(_ context.Context, _ string, seqs []int64) ([]*msgpb.MsgData, error) {
	want := map[int64]struct{}{}
	for _, seq := range seqs {
		want[seq] = struct{}{}
	}
	var out []*msgpb.MsgData
	for _, msg := range f.msgs {
		if msg == nil {
			continue
		}
		if _, ok := want[msg.Seq]; ok {
			out = append(out, msg)
		}
	}
	return out, nil
}
func (f *fakeHistory) Last(context.Context, string) (*msgpb.MsgData, error) { return nil, nil }

var _ mq.Producer = (*fakeProducer)(nil)
var _ History = (*fakeHistory)(nil)
