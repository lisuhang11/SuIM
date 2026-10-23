package ws

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const (
	textContentType = 101
	maxTextRunes    = 4000
)

type textBody struct {
	Content string `json:"content"`
}

type sendHTTPReq struct {
	RecvID      string `json:"recv_id"`
	Content     string `json:"content"`
	ClientMsgID string `json:"client_msg_id"`
	SessionType int32  `json:"session_type"`
	ContentType int32  `json:"content_type"`
}

// HandleSend accepts one single-chat text message over HTTP and publishes it to toRedis.
func (s *Server) HandleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTTP(w, http.StatusMethodNotAllowed, int(errorcode.InvalidJSON), "method not allowed", nil)
		return
	}
	token := strings.TrimSpace(r.Header.Get("token"))
	if token == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		const prefix = "Bearer "
		if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
			token = strings.TrimSpace(auth[len(prefix):])
		}
	}
	if token == "" || s.tokens == nil {
		writeHTTP(w, http.StatusUnauthorized, int(errorcode.TokenRequired), errorcode.ErrTokenRequired.Error(), nil)
		return
	}
	userID, platformID, err := s.tokens.Validate(r.Context(), token)
	if err != nil || userID == "" {
		writeHTTP(w, http.StatusUnauthorized, int(errorcode.UserInvalidToken), "token invalid", nil)
		return
	}
	var in sendHTTPReq
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeHTTP(w, http.StatusBadRequest, int(errorcode.InvalidJSON), "request body is too large", nil)
			return
		}
		writeHTTP(w, http.StatusBadRequest, int(errorcode.InvalidJSON), errorcode.ErrInvalidJSON.Error(), nil)
		return
	}
	text := strings.TrimSpace(in.Content)
	if in.SessionType != sessionSingle || in.ContentType != textContentType {
		writeHTTP(w, http.StatusBadRequest, errArgs, "only single-chat text is supported", nil)
		return
	}
	if text == "" {
		writeHTTP(w, http.StatusBadRequest, errArgs, "content is required", nil)
		return
	}
	if utf8.RuneCountInString(text) > maxTextRunes {
		writeHTTP(w, http.StatusBadRequest, errArgs, "content is too long", nil)
		return
	}
	content, err := json.Marshal(textBody{Content: text})
	if err != nil {
		writeHTTP(w, http.StatusInternalServerError, errInternal, "marshal content", nil)
		return
	}
	code, msgText, resp := s.accept(r.Context(), &msgpb.MsgData{
		SendId:           userID,
		RecvId:           strings.TrimSpace(in.RecvID),
		ClientMsgId:      strings.TrimSpace(in.ClientMsgID),
		SenderPlatformId: platformID,
		SessionType:      sessionSingle,
		MsgFrom:          100,
		ContentType:      textContentType,
		Content:          content,
	})
	if code != 0 {
		status := http.StatusBadRequest
		if code == errInternal {
			status = http.StatusInternalServerError
		}
		if code == int(errorcode.MsgNotFriend) || code == int(errorcode.MsgBlocked) {
			status = http.StatusPreconditionFailed
		}
		writeHTTP(w, status, code, msgText, nil)
		return
	}
	writeHTTP(w, http.StatusOK, 0, "", map[string]any{
		"server_msg_id": resp.GetServerMsgId(),
		"client_msg_id": resp.GetClientMsgId(),
		"send_time":     resp.GetSendTime(),
	})
}

func writeHTTP(w http.ResponseWriter, status, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": msg, "data": data})
}
