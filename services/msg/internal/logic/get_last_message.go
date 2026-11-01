package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func (s *Server) GetLastMessage(ctx context.Context, req *msgpb.GetLastMessageReq) (*msgpb.GetLastMessageResp, error) {
	ids := req.GetConversationIds()
	var err error
	if len(ids) == 0 {
		ids, err = s.conversationIDs(ctx, req.GetUserId())
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query conversations")
		}
	}
	out := &msgpb.GetLastMessageResp{Msgs: map[string]*msgpb.MsgData{}}
	for _, id := range ids {
		if s.store == nil || id == "" {
			continue
		}
		if !canRead(id, req.GetUserId()) {
			return nil, errorcode.ErrNoPermission
		}
		msg, err := s.store.Last(ctx, id)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query last message")
		}
		if msg != nil {
			out.Msgs[id] = msg
		}
	}
	return out, nil
}
