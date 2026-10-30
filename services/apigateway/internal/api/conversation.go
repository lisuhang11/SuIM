package api

import (
	"net/http"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

func (s *Server) setConversations(w http.ResponseWriter, r *http.Request) {
	var req conversationpb.SetConversationsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	for _, id := range req.GetUserIds() {
		if !s.requireSelf(w, r, id) {
			return
		}
	}
	resp, err := s.conversations.SetConversations(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) fullOwnerConversationIDs(w http.ResponseWriter, r *http.Request) {
	var req conversationpb.GetFullOwnerConversationIDsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.conversations.GetFullOwnerConversationIDs(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) incrementalConversation(w http.ResponseWriter, r *http.Request) {
	var req conversationpb.GetIncrementalConversationReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.conversations.GetIncrementalConversation(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}
