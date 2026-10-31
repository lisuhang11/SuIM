package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const (
	contentHasReadReceipt int32 = 2200

	kindSingle = 1
	kindGroup  = 2
	kindNotice = 4
)

// MarkMsgsAsRead marks the listed sequences. The last sequence is the new read cursor.
func (s *Server) MarkMsgsAsRead(ctx context.Context, req *msgpb.MarkMsgsAsReadReq) (*msgpb.MarkMsgsAsReadResp, error) {
	userID := strings.TrimSpace(req.GetUserId())
	conversationID := strings.TrimSpace(req.GetConversationId())
	if userID == "" || conversationID == "" || len(req.GetSeqs()) == 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("seqs are required")
	}
	kind, session, peer, groupID, err := s.authorizeRead(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	maxSeq, err := s.maxSeq(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	// 1. 和 OpenIM 一样，列表最后一个序号当作已读游标，不另取最大值。
	hasReadSeq := req.GetSeqs()[len(req.GetSeqs())-1]
	if hasReadSeq > maxSeq {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("hasReadSeq must not be bigger than maxSeq")
	}
	if err := s.store.MarkRead(ctx, conversationID, userID, req.GetSeqs()); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("mark messages read")
	}
	current, err := s.readSeq(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}
	if hasReadSeq > current {
		if err := s.store.SetReadSeq(ctx, userID, conversationID, hasReadSeq); err != nil {
			return nil, errorcode.ErrInternal.WithMsg("set read seq")
		}
		current = hasReadSeq
	}
	s.afterSingleRead(ctx, conversationID, userID, req.GetSeqs(), session)
	recvID := peer
	if kind != kindSingle {
		recvID = groupID
		if recvID == "" {
			recvID = userID
		}
	}
	if err := s.sendReadReceipt(ctx, userID, recvID, groupID, conversationID, session, req.GetSeqs(), current, kind == kindSingle); err != nil {
		return nil, err
	}
	return &msgpb.MarkMsgsAsReadResp{}, nil
}

// MarkConversationAsRead advances the read cursor. Single chat also marks each sequence.
func (s *Server) MarkConversationAsRead(ctx context.Context, req *msgpb.MarkConversationAsReadReq) (*msgpb.MarkConversationAsReadResp, error) {
	userID := strings.TrimSpace(req.GetUserId())
	conversationID := strings.TrimSpace(req.GetConversationId())
	if userID == "" || conversationID == "" || req.GetHasReadSeq() < 0 {
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

// SetConversationHasReadSeq stores the cursor and tells the caller's other devices.
func (s *Server) SetConversationHasReadSeq(ctx context.Context, req *msgpb.SetConversationHasReadSeqReq) (*msgpb.SetConversationHasReadSeqResp, error) {
	userID := strings.TrimSpace(req.GetUserId())
	conversationID := strings.TrimSpace(req.GetConversationId())
	if userID == "" || conversationID == "" || req.GetHasReadSeq() < 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("has read seq is invalid")
	}
	if _, _, _, _, err := s.authorizeRead(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	maxSeq, err := s.maxSeq(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if req.GetHasReadSeq() > maxSeq {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("hasReadSeq must not be bigger than maxSeq")
	}
	if err := s.store.SetReadSeq(ctx, userID, conversationID, req.GetHasReadSeq()); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("set read seq")
	}
	if err := s.sendReadReceipt(ctx, userID, userID, "", conversationID, sessionSingle, nil, req.GetHasReadSeq(), false); err != nil {
		return nil, err
	}
	return &msgpb.SetConversationHasReadSeqResp{}, nil
}

func (s *Server) maxSeq(ctx context.Context, conversationID string) (int64, error) {
	if s.store == nil {
		return 0, errorcode.ErrInternal.WithMsg("message store is not configured")
	}
	seqs, err := s.store.MaxSeqs(ctx, []string{conversationID})
	if err != nil {
		return 0, errorcode.ErrInternal.WithMsg("query max seq")
	}
	return seqs[conversationID], nil
}

func (s *Server) readSeq(ctx context.Context, userID, conversationID string) (int64, error) {
	state, err := s.store.SeqState(ctx, userID, []string{conversationID})
	if err != nil {
		return 0, errorcode.ErrInternal.WithMsg("query read seq")
	}
	return state[conversationID].HasReadSeq, nil
}

// fillReadSeqs covers every sequence after the stored cursor, then adds any extra the client named.
func fillReadSeqs(current, target int64, extra []int64) []int64 {
	var seqs []int64
	seen := map[int64]struct{}{}
	for seq := current + 1; seq <= target; seq++ {
		seqs = append(seqs, seq)
		seen[seq] = struct{}{}
	}
	for _, seq := range extra {
		if seq <= 0 {
			continue
		}
		if _, ok := seen[seq]; ok {
			continue
		}
		seqs = append(seqs, seq)
		seen[seq] = struct{}{}
	}
	return seqs
}

func (s *Server) authorizeRead(ctx context.Context, userID, conversationID string) (kind int, session int32, peer, groupID string, err error) {
	switch {
	case strings.HasPrefix(conversationID, "si_"):
		a, b, ok := pairIDs("si_", conversationID)
		if !ok || (userID != a && userID != b) {
			return 0, 0, "", "", errorcode.ErrNoPermission
		}
		peer = b
		if peer == userID {
			peer = a
		}
		return kindSingle, sessionSingle, peer, "", nil
	case strings.HasPrefix(conversationID, "sg_"):
		return s.authorizeGroupRead(ctx, userID, strings.TrimPrefix(conversationID, "sg_"), sessionSuper)
	case strings.HasPrefix(conversationID, "g_"):
		return s.authorizeGroupRead(ctx, userID, strings.TrimPrefix(conversationID, "g_"), sessionGroup)
	case strings.HasPrefix(conversationID, "sn_"):
		a, b, ok := pairIDs("sn_", conversationID)
		if !ok || (userID != a && userID != b) {
			return 0, 0, "", "", errorcode.ErrNoPermission
		}
		return kindNotice, 4, userID, "", nil
	case strings.HasPrefix(conversationID, "n_"):
		if strings.Count(conversationID, "_") == 1 {
			return s.authorizeGroupRead(ctx, userID, strings.TrimPrefix(conversationID, "n_"), 4)
		}
		a, b, ok := pairIDs("n_", conversationID)
		if !ok || (userID != a && userID != b) {
			return 0, 0, "", "", errorcode.ErrNoPermission
		}
		return kindNotice, 4, userID, "", nil
	default:
		return 0, 0, "", "", errorcode.ErrMsgInvalidArgs.WithMsg("conversation id is invalid")
	}
}

func (s *Server) authorizeGroupRead(ctx context.Context, userID, groupID string, session int32) (int, int32, string, string, error) {
	if groupID == "" {
		return 0, 0, "", "", errorcode.ErrMsgInvalidArgs.WithMsg("group id is invalid")
	}
	if s.groups == nil {
		return 0, 0, "", "", errorcode.ErrInternal.WithMsg("group service is not configured")
	}
	speaker, err := s.groups.Speaker(ctx, groupID, userID)
	if err != nil {
		return 0, 0, "", "", errorcode.ErrInternal.WithMsg("check group")
	}
	if !speaker.Found || !speaker.InGroup {
		return 0, 0, "", "", errorcode.ErrMsgNotInGroup
	}
	kind := kindGroup
	if session == 4 {
		kind = kindNotice
	}
	return kind, session, userID, groupID, nil
}

func pairIDs(prefix, conversationID string) (string, string, bool) {
	rest := strings.TrimPrefix(conversationID, prefix)
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// sendReadReceipt publishes content type 2200. reliable keeps it in the notification history.
func (s *Server) sendReadReceipt(ctx context.Context, sendID, recvID, groupID, conversationID string, session int32, seqs []int64, hasReadSeq int64, reliable bool) error {
	tips, err := json.Marshal(struct {
		MarkAsReadUserID string  `json:"markAsReadUserID"`
		ConversationID   string  `json:"conversationID"`
		Seqs             []int64 `json:"seqs"`
		HasReadSeq       int64   `json:"hasReadSeq"`
	}{sendID, conversationID, seqs, hasReadSeq})
	if err != nil {
		return errorcode.ErrInternal.WithMsg("marshal read receipt")
	}
	content, err := json.Marshal(struct {
		Detail string `json:"detail"`
	}{Detail: string(tips)})
	if err != nil {
		return errorcode.ErrInternal.WithMsg("marshal read receipt")
	}
	now := time.Now().UnixMilli()
	msg := &msgpb.MsgData{
		SendId:      sendID,
		RecvId:      recvID,
		GroupId:     groupID,
		SessionType: session,
		MsgFrom:     200,
		ContentType: contentHasReadReceipt,
		Content:     content,
		SendTime:    now,
		CreateTime:  now,
		Options: map[string]bool{
			msgconvOptNotNotification: false,
			"isSendMsg":               false,
			"history":                 reliable,
			"unreadCount":             false,
			"offlinePush":             false,
		},
	}
	_, err = s.SendMsg(ctx, &msgpb.SendMsgReq{MsgData: msg})
	return err
}

const msgconvOptNotNotification = "isNotNotification"

type singleReadCallback struct {
	webhook.CommonCallbackReq
	ConversationID string  `json:"conversationID"`
	UserID         string  `json:"userID"`
	Seqs           []int64 `json:"Seqs"`
	ContentType    int32   `json:"contentType"`
}

type groupReadCallback struct {
	webhook.CommonCallbackReq
	SendID       string `json:"sendID"`
	ReceiveID    string `json:"receiveID"`
	UnreadMsgNum int64  `json:"unreadMsgNum"`
	ContentType  int64  `json:"contentType"`
}

func (s *Server) afterSingleRead(ctx context.Context, conversationID, userID string, seqs []int64, contentType int32) {
	if s.hooks.Client == nil || !s.hooks.AfterSingleRead.Enable {
		return
	}
	req := singleReadCallback{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterSingleMsgRead},
		ConversationID:    conversationID,
		UserID:            userID,
		Seqs:              seqs,
		ContentType:       contentType,
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterSingleMsgRead, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterSingleRead.AfterConfig)
}

func (s *Server) afterGroupRead(ctx context.Context, userID string, hasReadSeq int64, contentType int32) {
	if s.hooks.Client == nil || !s.hooks.AfterGroupRead.Enable {
		return
	}
	req := groupReadCallback{
		CommonCallbackReq: webhook.CommonCallbackReq{CallbackCommand: webhook.CommandAfterGroupMsgRead},
		SendID:            userID,
		ReceiveID:         userID,
		UnreadMsgNum:      hasReadSeq,
		ContentType:       int64(contentType),
	}
	s.hooks.Client.AsyncPost(ctx, webhook.CommandAfterGroupMsgRead, req, &webhook.CommonCallbackResp{}, &s.hooks.AfterGroupRead.AfterConfig)
}
