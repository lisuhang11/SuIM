package logic

import (
	"context"
	"testing"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestGetSeqMessageBounds(t *testing.T) {
	hist := &fakeHistory{
		min: 4,
		max: 5,
		msgs: []*msgpb.MsgData{
			{Seq: 4, SendId: "a", RecvId: "b"},
			{Seq: 9, SendId: "a", RecvId: "b"},
		},
	}
	srv := New(hist, nil, nil, nil)
	asc, err := srv.GetSeqMessage(context.Background(), &msgpb.GetSeqMessageReq{
		UserId: "a",
		Order:  0,
		Conversations: []*msgpb.ConversationSeqs{{
			ConversationId: "si_a_b",
			Seqs:           []int64{2, 4, 9},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	pull := asc.Msgs["si_a_b"]
	if pull == nil || !pull.IsEnd || pull.EndSeq != 5 || pull.MaxSeq != 5 || len(pull.Msgs) != 1 || pull.Msgs[0].Seq != 4 {
		t.Fatalf("ascending %+v", pull)
	}

	desc, err := srv.GetSeqMessage(context.Background(), &msgpb.GetSeqMessageReq{
		UserId: "a",
		Order:  orderDesc,
		Conversations: []*msgpb.ConversationSeqs{{
			ConversationId: "si_a_b",
			Seqs:           []int64{2, 4},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	pull = desc.Msgs["si_a_b"]
	if pull == nil || !pull.IsEnd || pull.EndSeq != 4 || len(pull.Msgs) != 1 || pull.Msgs[0].Seq != 4 {
		t.Fatalf("descending %+v", pull)
	}
}

type pinnedIDs []string

func (p pinnedIDs) PinnedConversationIDs(context.Context, string) ([]string, error) {
	return []string(p), nil
}

func TestGetConversationsFillsTimeAndPinned(t *testing.T) {
	hist := &fakeHistory{max: 5, read: 2, seqTime: 99}
	srv := New(hist, nil, nil, nil)
	srv.UsePinned(pinnedIDs{"si_a_b"})
	resp, err := srv.GetConversationsHasReadAndMaxSeq(context.Background(), &msgpb.GetConversationsHasReadAndMaxSeqReq{
		UserId:          "a",
		ConversationIds: []string{"si_a_b"},
		ReturnPinned:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	item := resp.Seqs["si_a_b"]
	if item == nil || item.MaxSeq != 5 || item.HasReadSeq != 2 || item.MaxSeqTime != 99 {
		t.Fatalf("seqs %+v", item)
	}
	if len(resp.PinnedConversationIds) != 1 || resp.PinnedConversationIds[0] != "si_a_b" {
		t.Fatalf("pinned %+v", resp.PinnedConversationIds)
	}

	plain, err := srv.GetConversationsHasReadAndMaxSeq(context.Background(), &msgpb.GetConversationsHasReadAndMaxSeqReq{
		UserId:          "a",
		ConversationIds: []string{"si_a_b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.PinnedConversationIds) != 0 {
		t.Fatalf("pinned without request %+v", plain.PinnedConversationIds)
	}
}
