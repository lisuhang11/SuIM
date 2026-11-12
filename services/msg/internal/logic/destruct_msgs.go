package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msg/internal/store"
)

// DestructMsgs hard-deletes aged messages and raises conversation min_seq.
func (s *Server) DestructMsgs(ctx context.Context, req *msgpb.DestructMsgsReq) (*msgpb.DestructMsgsResp, error) {
	if req.GetTimestamp() <= 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("timestamp is required")
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 50
	}
	st, ok := s.store.(*store.Store)
	if !ok || st == nil {
		return nil, errorcode.ErrInternal.WithMsg("message store does not support destruct")
	}
	count, err := st.DestructBefore(ctx, req.GetTimestamp(), limit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("destruct messages")
	}
	return &msgpb.DestructMsgsResp{Count: int32(count)}, nil
}

// GetLastMessageSeqByTime returns the greatest seq at or before time for one conversation.
func (s *Server) GetLastMessageSeqByTime(ctx context.Context, req *msgpb.GetLastMessageSeqByTimeReq) (*msgpb.GetLastMessageSeqByTimeResp, error) {
	if req.GetConversationId() == "" {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("conversation_id is required")
	}
	st, ok := s.store.(*store.Store)
	if !ok || st == nil {
		return nil, errorcode.ErrInternal.WithMsg("message store does not support seq by time")
	}
	seq, err := st.LastSeqByTime(ctx, req.GetConversationId(), req.GetTime())
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query last message seq")
	}
	return &msgpb.GetLastMessageSeqByTimeResp{Seq: seq}, nil
}
