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
		// 1. 单聊只允许会话双方。群会话允许当前成员，以及离开时记下水位的人。
		minSeq, maxSeq, err := s.visibleSeqs(ctx, req.GetUserId(), item.ConversationId)
		if err != nil {
			return nil, err
		}
		if s.store == nil {
			continue
		}
		limit := item.Num
		if limit <= 0 || limit > 200 {
			limit = 200
		}
		begin, end := item.Begin, item.End
		// 2. begin 和 end 都为 0 时取这个人可见范围内的最新一页。
		if begin <= 0 && end <= 0 {
			end = maxSeq
			begin = end - limit + 1
			if minSeq > 0 && begin < minSeq {
				begin = minSeq
			}
			if begin < 1 {
				begin = 1
			}
		} else {
			if begin <= 0 || (minSeq > 0 && begin < minSeq) {
				begin = minSeq
			}
			if begin < 1 {
				begin = 1
			}
			if end <= 0 {
				end = begin + limit - 1
			}
			if maxSeq > 0 && end > maxSeq {
				end = maxSeq
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
