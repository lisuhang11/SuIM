package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func (s *Server) GetConversationsHasReadAndMaxSeq(ctx context.Context, req *msgpb.GetConversationsHasReadAndMaxSeqReq) (*msgpb.GetConversationsHasReadAndMaxSeqResp, error) {
	ids := req.GetConversationIds()
	var err error
	if len(ids) == 0 {
		ids, err = s.conversationIDs(ctx, req.GetUserId())
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query conversations")
		}
	}
	out := &msgpb.GetConversationsHasReadAndMaxSeqResp{Seqs: map[string]*msgpb.Seqs{}}
	// 1. 只有客户端要求时才查置顶。会话表不存在时仍返回序号。
	if req.GetReturnPinned() && s.store != nil {
		pinned, err := s.store.PinnedConversationIDs(ctx, req.GetUserId())
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query pinned conversations")
		}
		out.PinnedConversationIds = pinned
	}
	if s.store == nil || len(ids) == 0 {
		return out, nil
	}
	state, err := s.store.SeqState(ctx, req.GetUserId(), ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query read seq")
	}
	for id, item := range state {
		out.Seqs[id] = &msgpb.Seqs{MaxSeq: item.MaxSeq, HasReadSeq: item.HasReadSeq, MaxSeqTime: item.MaxSeqTime}
	}
	return out, nil
}
