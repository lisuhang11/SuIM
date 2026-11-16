package api

import (
	"net/http"

	pushpb "github.com/lisuhang11/SuIM/proto/push"
)

func (s *Server) fcmUpdateToken(w http.ResponseWriter, r *http.Request) {
	var req pushpb.FcmUpdateTokenReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.push.FcmUpdateToken(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) delUserPushToken(w http.ResponseWriter, r *http.Request) {
	var req pushpb.DelUserPushTokenReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.push.DelUserPushToken(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) setAppBadge(w http.ResponseWriter, r *http.Request) {
	var req pushpb.SetAppBadgeReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.push.SetAppBadge(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}
