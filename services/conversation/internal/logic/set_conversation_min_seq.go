package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
)

// 写下这些人的可见下限。
func (s *Server) SetConversationMinSeq(ctx context.Context, req *conversationpb.SetConversationMinSeqReq) (*conversationpb.SetConversationMinSeqResp, error) {
	conversationID, owners, err := seqTarget(req.GetConversationId(), req.GetOwnerUserIds(), req.GetMinSeq())
	if err != nil {
		return nil, err
	}
	if s.seq == nil {
		return nil, errorcode.ErrInternal.WithMsg("message seq is not configured")
	}
	if err := s.seq.SetUserMinSeq(ctx, conversationID, owners, req.GetMinSeq()); err != nil {
		return nil, err
	}
	changed, err := s.store.SetOwnerSeq(ctx, conversationID, owners, false, req.GetMinSeq(), s.now())
	if err != nil {
		return nil, err
	}
	s.notifyChanged(ctx, conversationID, changed)
	return &conversationpb.SetConversationMinSeqResp{}, nil
}
