package logic

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestMarkConversationSingleNotifiesPeer(t *testing.T) {
	hist := &fakeHistory{max: 5, read: 2, seqTime: 1}
	prod := &fakeProducer{}
	srv := New(hist, prod, nil, nil)
	_, err := srv.MarkConversationAsRead(context.Background(), &msgpb.MarkConversationAsReadReq{
		UserId: "a", ConversationId: "si_a_b", HasReadSeq: 5, Seqs: []int64{4},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hist.read != 5 || hist.readUser != "a" || hist.readConv != "si_a_b" {
		t.Fatalf("cursor %+v", hist)
	}
	if len(hist.marked) != 3 || hist.marked[0] != 3 || hist.marked[2] != 5 || hist.markUser != "a" {
		t.Fatalf("marked %v", hist.marked)
	}
	msg := published(t, prod)
	if msg.ContentType != contentHasReadReceipt || msg.SendId != "a" || msg.RecvId != "b" || msg.Options["history"] != true {
		t.Fatalf("receipt %+v", msg)
	}
	if prod.key != "n_a_b" {
		t.Fatalf("key %s", prod.key)
	}
	tips := receiptTips(t, msg.Content)
	if tips.ConversationID != "si_a_b" || tips.HasReadSeq != 5 || tips.MarkAsReadUserID != "a" {
		t.Fatalf("tips %+v", tips)
	}
}

func TestMarkConversationRejectsPastMax(t *testing.T) {
	prod := &fakeProducer{}
	srv := New(&fakeHistory{max: 2}, prod, nil, nil)
	_, err := srv.MarkConversationAsRead(context.Background(), &msgpb.MarkConversationAsReadReq{
		UserId: "a", ConversationId: "si_a_b", HasReadSeq: 3,
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.MsgInvalidArgs || prod.n != 0 {
		t.Fatalf("code %d published %d err %v", code, prod.n, err)
	}
}

func TestMarkConversationGroupKeepsMessageFlags(t *testing.T) {
	hist := &fakeHistory{max: 4}
	prod := &fakeProducer{}
	groups := &fakeGroups{speakers: map[string]GroupSpeaker{
		"g1:a": {Found: true, InGroup: true},
	}}
	srv := New(hist, prod, nil, groups)
	_, err := srv.MarkConversationAsRead(context.Background(), &msgpb.MarkConversationAsReadReq{
		UserId: "a", ConversationId: "g_g1", HasReadSeq: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hist.read != 4 || len(hist.marked) != 0 {
		t.Fatalf("cursor %d marked %v", hist.read, hist.marked)
	}
	msg := published(t, prod)
	if msg.SendId != "a" || msg.RecvId != "a" || msg.Options["history"] != false || prod.key != "n_a_a" {
		t.Fatalf("receipt key=%s msg=%+v", prod.key, msg)
	}
}

func TestMarkConversationOutsider(t *testing.T) {
	srv := New(&fakeHistory{max: 1}, &fakeProducer{}, nil, nil)
	_, err := srv.MarkConversationAsRead(context.Background(), &msgpb.MarkConversationAsReadReq{
		UserId: "c", ConversationId: "si_a_b", HasReadSeq: 1,
	})
	if code, _ := errorcode.CodeOf(err); code != errorcode.NoPermission {
		t.Fatalf("code %d err %v", code, err)
	}
}

func TestMarkMsgsUsesLastSeq(t *testing.T) {
	hist := &fakeHistory{max: 9, read: 1, seqTime: 1}
	prod := &fakeProducer{}
	srv := New(hist, prod, nil, nil)
	_, err := srv.MarkMsgsAsRead(context.Background(), &msgpb.MarkMsgsAsReadReq{
		UserId: "b", ConversationId: "si_a_b", Seqs: []int64{3, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hist.read != 2 || len(hist.marked) != 2 || hist.marked[0] != 3 {
		t.Fatalf("read %d marked %v", hist.read, hist.marked)
	}
	tips := receiptTips(t, published(t, prod).Content)
	if tips.HasReadSeq != 2 {
		t.Fatalf("tips %+v", tips)
	}
}

func published(t *testing.T, prod *fakeProducer) *msgpb.MsgData {
	t.Helper()
	if prod.n != 1 {
		t.Fatalf("published %d", prod.n)
	}
	msg := &msgpb.MsgData{}
	if err := proto.Unmarshal(prod.body, msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

type readTips struct {
	MarkAsReadUserID string  `json:"markAsReadUserID"`
	ConversationID   string  `json:"conversationID"`
	Seqs             []int64 `json:"seqs"`
	HasReadSeq       int64   `json:"hasReadSeq"`
}

func receiptTips(t *testing.T, content []byte) readTips {
	t.Helper()
	var body struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(content, &body); err != nil {
		t.Fatal(err)
	}
	var tips readTips
	if err := json.Unmarshal([]byte(body.Detail), &tips); err != nil {
		t.Fatal(err)
	}
	return tips
}
