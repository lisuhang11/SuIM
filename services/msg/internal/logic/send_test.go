package logic

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/mq"
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
	begin, end int64
}

func (f *fakeHistory) ConversationIDs(context.Context, string) ([]string, error) { return nil, nil }
func (f *fakeHistory) MaxSeqs(context.Context, []string) (map[string]int64, error) {
	return map[string]int64{"si_a_b": f.max}, nil
}
func (f *fakeHistory) SeqState(context.Context, string, []string) (map[string]store.SeqState, error) {
	return nil, nil
}
func (f *fakeHistory) Range(_ context.Context, _ string, begin, end, _ int64) ([]*msgpb.MsgData, error) {
	f.begin, f.end = begin, end
	return f.msgs, nil
}
func (f *fakeHistory) BySeqs(context.Context, string, []int64) ([]*msgpb.MsgData, error) {
	return f.msgs, nil
}
func (f *fakeHistory) Last(context.Context, string) (*msgpb.MsgData, error) { return nil, nil }

var _ mq.Producer = (*fakeProducer)(nil)
var _ History = (*fakeHistory)(nil)
