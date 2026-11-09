package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// 把列出的序号标成已读。其中最大的序号成为新的已读游标。
func (s *Server) MarkMsgsAsRead(ctx context.Context, req *msgpb.MarkMsgsAsReadReq) (*msgpb.MarkMsgsAsReadResp, error) {
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	conversationID := strings.TrimSpace(req.GetConversationId())
	if conversationID == "" || len(req.GetSeqs()) == 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("seqs are required")
	}
	kind, session, peer, groupID, err := s.authorizeRead(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	maxSeq, err := s.maxSeq(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	// 1. 和 OpenIM 一样，列表最后一个序号当作已读游标，不另取最大值。
	hasReadSeq := req.GetSeqs()[len(req.GetSeqs())-1]
	if hasReadSeq > maxSeq {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("hasReadSeq must not be bigger than maxSeq")
	}
	if err := s.store.MarkRead(ctx, conversationID, userID, req.GetSeqs()); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("mark messages read")
	}
	current, err := s.readSeq(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	if hasReadSeq > current {
		if err := s.store.SetReadSeq(ctx, userID, conversationID, hasReadSeq); err != nil {
			return nil, errorcode.ErrInternal.WithMsg("set read seq")
		}
		current = hasReadSeq
	}
	s.afterSingleRead(ctx, conversationID, userID, req.GetSeqs(), session)
	recvID := peer
	if kind != kindSingle {
		recvID = groupID
		if recvID == "" {
			recvID = userID
		}
	}
	if err := s.sendReadReceipt(ctx, userID, recvID, groupID, conversationID, session, req.GetSeqs(), current, kind == kindSingle); err != nil {
		return nil, err
	}
	return &msgpb.MarkMsgsAsReadResp{}, nil
}
