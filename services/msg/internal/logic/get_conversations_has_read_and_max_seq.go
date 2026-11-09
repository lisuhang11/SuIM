package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// 返回调用者在这些会话上的已读序号和最大序号。没点名会话时用他已有的会话。
func (s *Server) GetConversationsHasReadAndMaxSeq(ctx context.Context, req *msgpb.GetConversationsHasReadAndMaxSeqReq) (*msgpb.GetConversationsHasReadAndMaxSeqResp, error) {
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	ids := req.GetConversationIds()
	if len(ids) == 0 {
		ids, err = s.conversationIDs(ctx, userID)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query conversations")
		}
	}
	out := &msgpb.GetConversationsHasReadAndMaxSeqResp{Seqs: map[string]*msgpb.Seqs{}}
	if req.GetReturnPinned() && s.pinned != nil {
		pinned, err := s.pinned.PinnedConversationIDs(ctx, userID)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query pinned conversations")
		}
		out.PinnedConversationIds = pinned
	}
	if s.store == nil || len(ids) == 0 {
		return out, nil
	}
	state, err := s.store.SeqState(ctx, userID, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query read seq")
	}
	for id, item := range state {
		if _, group := groupIDFromConversation(id); group {
			if _, _, err := s.visibleSeqs(ctx, userID, id); err != nil {
				if code, ok := errorcode.CodeOf(err); ok && code == errorcode.NoPermission {
					continue
				}
				return nil, err
			}
		}
		out.Seqs[id] = &msgpb.Seqs{MaxSeq: item.MaxSeq, HasReadSeq: item.HasReadSeq, MaxSeqTime: item.MaxSeqTime}
	}
	return out, nil
}
