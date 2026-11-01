package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// SetConversationHasReadSeq stores the cursor and tells the caller's other devices.
func (s *Server) SetConversationHasReadSeq(ctx context.Context, req *msgpb.SetConversationHasReadSeqReq) (*msgpb.SetConversationHasReadSeqResp, error) {
	userID := strings.TrimSpace(req.GetUserId())
	conversationID := strings.TrimSpace(req.GetConversationId())
	if userID == "" || conversationID == "" || req.GetHasReadSeq() < 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("has read seq is invalid")
	}
	if _, _, _, _, err := s.authorizeRead(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	maxSeq, err := s.maxSeq(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if req.GetHasReadSeq() > maxSeq {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("hasReadSeq must not be bigger than maxSeq")
	}
	if err := s.store.SetReadSeq(ctx, userID, conversationID, req.GetHasReadSeq()); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("set read seq")
	}
	if err := s.sendReadReceipt(ctx, userID, userID, "", conversationID, sessionSingle, nil, req.GetHasReadSeq(), false); err != nil {
		return nil, err
	}
	return &msgpb.SetConversationHasReadSeqResp{}, nil
}
