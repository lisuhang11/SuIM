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

// 先补上已读游标之后到目标序号的每一条，再并入客户端额外点名的序号。
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

// 发出内容类型 2200 的已读回执。reliable 为真时这份通知留在通知历史里。
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
