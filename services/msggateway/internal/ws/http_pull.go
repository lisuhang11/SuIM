package ws

import (
	"net/http"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

type pullHTTPReq struct {
	ConversationID string `json:"conversation_id"`
	Begin          int64  `json:"begin"`
	End            int64  `json:"end"`
	Limit          int64  `json:"limit"`
}

type pullHTTPMsg struct {
	Seq            int64  `json:"seq"`
	SendID         string `json:"send_id"`
	RecvID         string `json:"recv_id"`
	ClientMsgID    string `json:"client_msg_id"`
	ServerMsgID    string `json:"server_msg_id"`
	ContentType    int32  `json:"content_type"`
	Text           string `json:"text"`
	SendTime       int64  `json:"send_time"`
	Status         int32  `json:"status"`
	SenderNickname string `json:"sender_nickname"`
	SenderFaceURL  string `json:"sender_face_url"`
}

// HandlePull returns one page of a single-chat history. begin and end of 0 mean the latest page.
func (s *Server) HandlePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTTP(w, http.StatusMethodNotAllowed, int(errorcode.InvalidJSON), "method not allowed", nil)
		return
	}
	userID, _, ok := s.tokenUser(w, r)
	if !ok {
		return
	}
	var in pullHTTPReq
	if err := decodeHTTP(w, r, &in); err != nil {
		return
	}
	conversationID := strings.TrimSpace(in.ConversationID)
	if s.messages == nil {
		writeHTTP(w, http.StatusInternalServerError, errInternal, "msg service is not configured", nil)
		return
	}
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	resp, err := s.messages.PullMessageBySeqs(r.Context(), &msgpb.PullMessageBySeqsReq{
		UserId: userID,
		SeqRanges: []*msgpb.SeqRange{{
			ConversationId: conversationID,
			Begin:          in.Begin,
			End:            in.End,
			Num:            limit,
		}},
	})
	if err != nil {
		status, code, text := httpFromErr(err)
		writeHTTP(w, status, code, text, nil)
		return
	}
	var page *msgpb.PullMsgs
	if resp != nil {
		page = resp.Msgs[conversationID]
		if page == nil {
			page = resp.NotificationMsgs[conversationID]
		}
	}
	out := make([]pullHTTPMsg, 0)
	var isEnd bool
	var maxSeq int64
	if page != nil {
		isEnd = page.IsEnd
		maxSeq = page.MaxSeq
		for _, msg := range page.Msgs {
			if msg == nil {
				continue
			}
			out = append(out, pullHTTPMsg{
				Seq:            msg.Seq,
				SendID:         msg.SendId,
				RecvID:         msg.RecvId,
				ClientMsgID:    msg.ClientMsgId,
				ServerMsgID:    msg.ServerMsgId,
				ContentType:    msg.ContentType,
				Text:           textOf(msg.Content),
				SendTime:       msg.SendTime,
				Status:         msg.Status,
				SenderNickname: msg.SenderNickname,
				SenderFaceURL:  msg.SenderFaceUrl,
			})
		}
	}
	writeHTTP(w, http.StatusOK, 0, "", map[string]any{
		"msgs":    out,
		"is_end":  isEnd,
		"max_seq": maxSeq,
	})
}
