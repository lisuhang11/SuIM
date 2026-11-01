package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func (s *Server) PullMessageBySeqs(ctx context.Context, req *msgpb.PullMessageBySeqsReq) (*msgpb.PushMessages, error) {
	out := &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{}, NotificationMsgs: map[string]*msgpb.PullMsgs{}}
	for _, item := range req.GetSeqRanges() {
		if item == nil || item.ConversationId == "" {
			continue
		}
		// 1. 单聊只允许会话双方拉取。
		if !canRead(item.ConversationId, req.GetUserId()) {
			return nil, errorcode.ErrNoPermission
		}
		if s.store == nil {
			continue
		}
		limit := item.Num
		if limit <= 0 || limit > 200 {
			limit = 200
		}
		maxSeqs, err := s.store.MaxSeqs(ctx, []string{item.ConversationId})
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query max seq")
		}
		maxSeq := maxSeqs[item.ConversationId]
		begin, end := item.Begin, item.End
		// 2. begin 和 end 都为 0 时取最新一页。指定了区间就按区间查。
		if begin <= 0 && end <= 0 {
			end = maxSeq
			begin = end - limit + 1
			if begin < 1 {
				begin = 1
			}
		} else {
			if begin <= 0 {
				begin = 1
			}
			if end <= 0 {
				end = begin + limit - 1
			}
		}
		var msgs []*msgpb.MsgData
		if end >= begin && maxSeq > 0 {
			msgs, err = s.store.Range(ctx, item.ConversationId, begin, end, limit)
			if err != nil {
				return nil, errorcode.ErrInternal.WithMsg("query messages")
			}
		}
		isEnd := rangeIsEnd(req.Order, begin, end, maxSeq)
		if item.Begin <= 0 && item.End <= 0 {
			isEnd = begin <= 1 || maxSeq == 0
		}
		putPull(out, item.ConversationId, &msgpb.PullMsgs{
			Msgs:   msgs,
			IsEnd:  isEnd,
			EndSeq: end,
			MaxSeq: maxSeq,
		})
	}
	return out, nil
}
