package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func (s *Server) GetConversationMaxSeq(ctx context.Context, req *msgpb.GetConversationMaxSeqReq) (*msgpb.GetConversationMaxSeqResp, error) {
	conversationID := strings.TrimSpace(req.GetConversationId())
	if conversationID == "" {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("conversation id is invalid")
	}
	if s.store == nil {
		return &msgpb.GetConversationMaxSeqResp{}, nil
	}
	seqs, err := s.store.MaxSeqs(ctx, []string{conversationID})
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query max seq")
	}
	return &msgpb.GetConversationMaxSeqResp{MaxSeq: seqs[conversationID]}, nil
}
