package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// 把已读游标推到指定序号。单聊还会把游标之间的每条标成已读。
func (s *Server) MarkConversationAsRead(ctx context.Context, req *msgpb.MarkConversationAsReadReq) (*msgpb.MarkConversationAsReadResp, error) {
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	conversationID := strings.TrimSpace(req.GetConversationId())
	if conversationID == "" || req.GetHasReadSeq() < 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("has read seq is invalid")
	}
	kind, session, peer, _, err := s.authorizeRead(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	maxSeq, err := s.maxSeq(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if req.GetHasReadSeq() > maxSeq {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("hasReadSeq must not be bigger than maxSeq")
	}
	current, err := s.readSeq(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	if kind == kindSingle {
		// 2. 补上已读游标之间漏掉的序号，避免客户端跳着标。
		seqs := fillReadSeqs(current, req.GetHasReadSeq(), req.GetSeqs())
		if len(seqs) > 0 {
			if err := s.store.MarkRead(ctx, conversationID, userID, seqs); err != nil {
				return nil, errorcode.ErrInternal.WithMsg("mark messages read")
			}
		}
		if req.GetHasReadSeq() > current {
			if err := s.store.SetReadSeq(ctx, userID, conversationID, req.GetHasReadSeq()); err != nil {
				return nil, errorcode.ErrInternal.WithMsg("set read seq")
			}
			current = req.GetHasReadSeq()
		}
		s.afterSingleRead(ctx, conversationID, userID, req.GetSeqs(), sessionSingle)
		if err := s.sendReadReceipt(ctx, userID, peer, "", conversationID, sessionSingle, seqs, current, true); err != nil {
			return nil, err
		}
		return &msgpb.MarkConversationAsReadResp{}, nil
	}
	if req.GetHasReadSeq() > current {
		if err := s.store.SetReadSeq(ctx, userID, conversationID, req.GetHasReadSeq()); err != nil {
			return nil, errorcode.ErrInternal.WithMsg("set read seq")
		}
		current = req.GetHasReadSeq()
	}
	// 3. 群和通知会话只推进自己的游标，回执发给自己，不改消息上的 is_read。
	if err := s.sendReadReceipt(ctx, userID, userID, "", conversationID, sessionSingle, nil, current, false); err != nil {
		return nil, err
	}
	if kind == kindGroup {
		s.afterGroupRead(ctx, userID, req.GetHasReadSeq(), session)
	}
	return &msgpb.MarkConversationAsReadResp{}, nil
}
