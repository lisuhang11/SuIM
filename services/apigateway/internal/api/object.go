package api

import (
	"net/http"

	thirdpb "github.com/lisuhang11/SuIM/proto/third"
)

func (s *Server) partLimit(w http.ResponseWriter, r *http.Request) {
	var req thirdpb.PartLimitReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.third.PartLimit(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) initiateMultipartUpload(w http.ResponseWriter, r *http.Request) {
	var req thirdpb.InitiateMultipartUploadReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.third.InitiateMultipartUpload(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) authSign(w http.ResponseWriter, r *http.Request) {
	var req thirdpb.AuthSignReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.third.AuthSign(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) completeMultipartUpload(w http.ResponseWriter, r *http.Request) {
	var req thirdpb.CompleteMultipartUploadReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.third.CompleteMultipartUpload(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) initiateFormData(w http.ResponseWriter, r *http.Request) {
	var req thirdpb.InitiateFormDataReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.third.InitiateFormData(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) completeFormData(w http.ResponseWriter, r *http.Request) {
	var req thirdpb.CompleteFormDataReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.third.CompleteFormData(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}
