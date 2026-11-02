package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func (s *Server) GetSeqMessage(ctx context.Context, req *msgpb.GetSeqMessageReq) (*msgpb.PushMessages, error) {
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	out := &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{}, NotificationMsgs: map[string]*msgpb.PullMsgs{}}
	for _, item := range req.GetConversations() {
		if item == nil || item.ConversationId == "" {
			continue
		}
		minSeq, maxSeq, err := s.visibleSeqs(ctx, userID, item.ConversationId)
		if err != nil {
			return nil, err
		}
		if s.store == nil {
			continue
		}
		wanted, isEnd, endSeq := seqWindow(req.Order, minSeq, maxSeq, item.Seqs)
		var msgs []*msgpb.MsgData
		if len(wanted) > 0 {
			if len(wanted) > 200 {
				wanted = wanted[:200]
			}
			msgs, err = s.store.BySeqs(ctx, item.ConversationId, wanted)
			if err != nil {
				return nil, errorcode.ErrInternal.WithMsg("query messages")
			}
		}
		putPull(out, item.ConversationId, &msgpb.PullMsgs{
			Msgs:   msgs,
			IsEnd:  isEnd,
			EndSeq: endSeq,
			MaxSeq: maxSeq,
		})
	}
	return out, nil
}
