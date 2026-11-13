package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
)

// GetConversationsNeedClearMsg lists conversations whose timed clear is due now.
func (s *Server) GetConversationsNeedClearMsg(ctx context.Context, _ *conversationpb.GetConversationsNeedClearMsgReq) (*conversationpb.GetConversationsNeedClearMsgResp, error) {
	rows, err := s.store.FindDueDestruct(ctx, s.now().UnixMilli(), 100)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversations need clear")
	}
	out := make([]*conversationpb.Conversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, conversationPB(row))
	}
	return &conversationpb.GetConversationsNeedClearMsgResp{Conversations: out}, nil
}

// ClearUserConversationMsg raises each due conversation's visible floor and bumps latest_msg_destruct_time.
func (s *Server) ClearUserConversationMsg(ctx context.Context, req *conversationpb.ClearUserConversationMsgReq) (*conversationpb.ClearUserConversationMsgResp, error) {
	ts := req.GetTimestamp()
	if ts <= 0 {
		ts = s.now().UnixMilli()
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.store.FindDueDestruct(ctx, ts, limit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversations need clear")
	}
	when := time.UnixMilli(ts).UTC()
	for _, row := range rows {
		if row == nil || !row.IsMsgDestruct || row.MsgDestructTime == 0 {
			continue
		}
		cutoff := ts - row.MsgDestructTime*1000
		var seq int64
		if s.history != nil {
			seq, err = s.history.LastSeqByTime(ctx, row.ConversationID, cutoff)
			if err != nil {
				return nil, errorcode.ErrInternal.WithMsg("query last message seq")
			}
		}
		minSeq := int64(-1)
		if seq > 0 {
			minSeq = seq + 1
			if s.seq != nil {
				if err := s.seq.SetUserMinSeq(ctx, row.ConversationID, []string{row.OwnerUserID}, minSeq); err != nil {
					return nil, errorcode.ErrInternal.WithMsg("set user min seq")
				}
			}
		}
		if err := s.store.MarkDestructed(ctx, row.OwnerUserID, row.ConversationID, minSeq, when); err != nil {
			return nil, errorcode.ErrInternal.WithMsg("mark conversation destructed")
		}
		s.notifyChanged(ctx, row.ConversationID, []string{row.OwnerUserID})
	}
	return &conversationpb.ClearUserConversationMsgResp{Count: int32(len(rows))}, nil
}
