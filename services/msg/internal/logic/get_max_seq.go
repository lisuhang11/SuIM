package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func (s *Server) GetMaxSeq(ctx context.Context, req *msgpb.GetMaxSeqReq) (*msgpb.GetMaxSeqResp, error) {
	// 1. 汇总这个用户已经有消息的会话，再加上仍在其中的群会话。
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	ids, err := s.conversationIDs(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversations")
	}
	maxSeq := map[string]int64{}
	if s.store != nil && len(ids) > 0 {
		maxSeq, err = s.store.MaxSeqs(ctx, ids)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query max seq")
		}
	}
	minSeq := map[string]int64{}
	for id, seq := range maxSeq {
		if seq > 0 {
			minSeq[id] = 1
		}
	}
	return &msgpb.GetMaxSeqResp{MaxSeqs: maxSeq, MinSeqs: minSeq}, nil
}
