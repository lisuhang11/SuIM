package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// 写下这些人的可见上限。序号表在消息服务，conversation.max_seq 留在这里。
func (s *Server) SetConversationMaxSeq(ctx context.Context, req *conversationpb.SetConversationMaxSeqReq) (*conversationpb.SetConversationMaxSeqResp, error) {
	conversationID, owners, err := seqTarget(req.GetConversationId(), req.GetOwnerUserIds(), req.GetMaxSeq())
	if err != nil {
		return nil, err
	}
	if s.seq == nil {
		return nil, errorcode.ErrInternal.WithMsg("message seq is not configured")
	}
	if err := s.seq.SetUserMaxSeq(ctx, conversationID, owners, req.GetMaxSeq()); err != nil {
		return nil, err
	}
	changed, err := s.store.SetOwnerSeq(ctx, conversationID, owners, true, req.GetMaxSeq(), s.now())
	if err != nil {
		return nil, err
	}
	s.notifyChanged(ctx, conversationID, changed)
	return &conversationpb.SetConversationMaxSeqResp{}, nil
}

// 写下这些人的可见下限。
