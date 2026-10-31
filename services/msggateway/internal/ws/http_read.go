package ws

import (
	"net/http"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

type markMsgsHTTPReq struct {
	UserID         string  `json:"user_id"`
	ConversationID string  `json:"conversation_id"`
	Seqs           []int64 `json:"seqs"`
}

type markConversationHTTPReq struct {
	UserID         string  `json:"user_id"`
	ConversationID string  `json:"conversation_id"`
	HasReadSeq     int64   `json:"has_read_seq"`
	Seqs           []int64 `json:"seqs"`
}

type setReadSeqHTTPReq struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id"`
	HasReadSeq     int64  `json:"has_read_seq"`
}

// HandleMarkMsgsAsRead marks the listed sequences for the logged-in user.
func (s *Server) HandleMarkMsgsAsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.readUser(w, r)
	if !ok {
		return
	}
	var in markMsgsHTTPReq
	if err := decodeHTTP(w, r, &in); err != nil {
		return
	}
	if !sameUser(w, userID, in.UserID) || s.messages == nil {
		return
	}
	if _, err := s.messages.MarkMsgsAsRead(r.Context(), &msgpb.MarkMsgsAsReadReq{
		UserId:         userID,
		ConversationId: strings.TrimSpace(in.ConversationID),
		Seqs:           in.Seqs,
	}); err != nil {
		status, code, text := httpFromErr(err)
		writeHTTP(w, status, code, text, nil)
		return
	}
	writeHTTP(w, http.StatusOK, 0, "", map[string]any{})
}

// HandleMarkConversationAsRead advances the logged-in user's read cursor.
func (s *Server) HandleMarkConversationAsRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.readUser(w, r)
	if !ok {
		return
	}
	var in markConversationHTTPReq
	if err := decodeHTTP(w, r, &in); err != nil {
		return
	}
	if !sameUser(w, userID, in.UserID) || s.messages == nil {
		return
	}
	if _, err := s.messages.MarkConversationAsRead(r.Context(), &msgpb.MarkConversationAsReadReq{
		UserId:         userID,
		ConversationId: strings.TrimSpace(in.ConversationID),
		HasReadSeq:     in.HasReadSeq,
		Seqs:           in.Seqs,
	}); err != nil {
		status, code, text := httpFromErr(err)
		writeHTTP(w, status, code, text, nil)
		return
	}
	writeHTTP(w, http.StatusOK, 0, "", map[string]any{})
}

type readSeqHTTPReq struct {
	ConversationIDs []string `json:"conversation_ids"`
}

// HandleReadSeqs returns max seq and read cursor for the logged-in user.
func (s *Server) HandleReadSeqs(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.readUser(w, r)
	if !ok {
		return
	}
	var in readSeqHTTPReq
	if err := decodeHTTP(w, r, &in); err != nil {
		return
	}
	resp, err := s.messages.GetConversationsHasReadAndMaxSeq(r.Context(), &msgpb.GetConversationsHasReadAndMaxSeqReq{
		UserId:          userID,
		ConversationIds: in.ConversationIDs,
	})
	if err != nil {
		status, code, text := httpFromErr(err)
		writeHTTP(w, status, code, text, nil)
		return
	}
	seqs := map[string]map[string]int64{}
	for id, item := range resp.GetSeqs() {
		if item == nil {
			continue
		}
		seqs[id] = map[string]int64{
			"max_seq":      item.MaxSeq,
			"has_read_seq": item.HasReadSeq,
			"max_seq_time": item.MaxSeqTime,
		}
	}
	writeHTTP(w, http.StatusOK, 0, "", map[string]any{"seqs": seqs})
}

// HandleSetConversationHasReadSeq stores a read cursor without marking each message.
func (s *Server) HandleSetConversationHasReadSeq(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.readUser(w, r)
	if !ok {
		return
	}
	var in setReadSeqHTTPReq
	if err := decodeHTTP(w, r, &in); err != nil {
		return
	}
	if !sameUser(w, userID, in.UserID) || s.messages == nil {
		return
	}
	if _, err := s.messages.SetConversationHasReadSeq(r.Context(), &msgpb.SetConversationHasReadSeqReq{
		UserId:         userID,
		ConversationId: strings.TrimSpace(in.ConversationID),
		HasReadSeq:     in.HasReadSeq,
	}); err != nil {
		status, code, text := httpFromErr(err)
		writeHTTP(w, status, code, text, nil)
		return
	}
	writeHTTP(w, http.StatusOK, 0, "", map[string]any{})
}

func (s *Server) readUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		writeHTTP(w, http.StatusMethodNotAllowed, int(errorcode.InvalidJSON), "method not allowed", nil)
		return "", false
	}
	userID, _, ok := s.tokenUser(w, r)
	if !ok {
		return "", false
	}
	if s.messages == nil {
		writeHTTP(w, http.StatusInternalServerError, errInternal, "msg service is not configured", nil)
		return "", false
	}
	return userID, true
}

func sameUser(w http.ResponseWriter, tokenUser, bodyUser string) bool {
	bodyUser = strings.TrimSpace(bodyUser)
	if bodyUser != "" && bodyUser != tokenUser {
		writeHTTP(w, http.StatusForbidden, int(errorcode.NoPermission), errorcode.ErrNoPermission.Error(), nil)
		return false
	}
	return true
}
