package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// SetConversationMaxSeq writes the per-user visible ceiling.
// The message service keeps the seq table. This service keeps conversation.max_seq.
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

// SetConversationMinSeq writes the per-user visible floor.
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

func seqTarget(conversationID string, userIDs []string, seq int64) (string, []string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || len(conversationID) > 255 || seq < 0 {
		return "", nil, errorcode.ErrConversationInvalidArgs.WithMsg("conversation seq is invalid")
	}
	owners, err := normalizeUserIDs(userIDs)
	if err != nil {
		return "", nil, err
	}
	return conversationID, owners, nil
}
