package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const orderDesc = 1

func (s *Server) GetMaxSeq(ctx context.Context, req *msgpb.GetMaxSeqReq) (*msgpb.GetMaxSeqResp, error) {
	// 1. 汇总这个用户已经有消息的会话，再加上仍在其中的群会话。
	ids, err := s.conversationIDs(ctx, req.GetUserId())
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

func (s *Server) GetConversationsHasReadAndMaxSeq(ctx context.Context, req *msgpb.GetConversationsHasReadAndMaxSeqReq) (*msgpb.GetConversationsHasReadAndMaxSeqResp, error) {
	ids := req.GetConversationIds()
	var err error
	if len(ids) == 0 {
		ids, err = s.conversationIDs(ctx, req.GetUserId())
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query conversations")
		}
	}
	out := &msgpb.GetConversationsHasReadAndMaxSeqResp{Seqs: map[string]*msgpb.Seqs{}}
	if s.store == nil || len(ids) == 0 {
		return out, nil
	}
	state, err := s.store.SeqState(ctx, req.GetUserId(), ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query read seq")
	}
	for id, item := range state {
		out.Seqs[id] = &msgpb.Seqs{MaxSeq: item.MaxSeq, HasReadSeq: item.HasReadSeq}
	}
	return out, nil
}

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

func (s *Server) GetSeqMessage(ctx context.Context, req *msgpb.GetSeqMessageReq) (*msgpb.PushMessages, error) {
	out := &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{}, NotificationMsgs: map[string]*msgpb.PullMsgs{}}
	for _, item := range req.GetConversations() {
		if item == nil || item.ConversationId == "" {
			continue
		}
		if !canRead(item.ConversationId, req.GetUserId()) {
			return nil, errorcode.ErrNoPermission
		}
		if s.store == nil {
			continue
		}
		msgs, err := s.store.BySeqs(ctx, item.ConversationId, item.Seqs)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query messages")
		}
		putPull(out, item.ConversationId, &msgpb.PullMsgs{Msgs: msgs, IsEnd: true})
	}
	return out, nil
}

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

func canRead(conversationID, userID string) bool {
	if userID == "" {
		return false
	}
	rest := ""
	switch {
	case strings.HasPrefix(conversationID, "si_"):
		rest = strings.TrimPrefix(conversationID, "si_")
	case strings.HasPrefix(conversationID, "sn_"):
		rest = strings.TrimPrefix(conversationID, "sn_")
	default:
		return true
	}
	parts := strings.SplitN(rest, "_", 2)
	return len(parts) == 2 && (parts[0] == userID || parts[1] == userID)
}

func rangeIsEnd(order int32, begin, end, maxSeq int64) bool {
	if order == orderDesc {
		return begin <= 1
	}
	if maxSeq <= 0 {
		return true
	}
	return maxSeq <= end
}

func putPull(out *msgpb.PushMessages, conversationID string, pull *msgpb.PullMsgs) {
	if msgconv.IsNotificationConversation(conversationID) {
		out.NotificationMsgs[conversationID] = pull
		return
	}
	out.Msgs[conversationID] = pull
}

func groupConversationID(group JoinedGroup) string {
	return msgconv.GroupConversationID(group.ID, group.Super)
}
