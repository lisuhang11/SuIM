package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// 返回调用者在可见范围内能读到的最后一条。没点名会话时用他已有的会话，读不到的会话不出现。
func (s *Server) GetLastMessage(ctx context.Context, req *msgpb.GetLastMessageReq) (*msgpb.GetLastMessageResp, error) {
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
	out := &msgpb.GetLastMessageResp{Msgs: map[string]*msgpb.MsgData{}}
	for _, id := range ids {
		if s.store == nil || id == "" {
			continue
		}
		_, maxSeq, err := s.visibleSeqs(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		if maxSeq <= 0 {
			continue
		}
		msgs, err := s.store.Range(ctx, id, maxSeq, maxSeq, 1)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query last message")
		}
		if len(msgs) > 0 {
			out.Msgs[id] = msgs[0]
		}
	}
	return out, nil
}
