package api

import (
	"net/http"

	"github.com/lisuhang11/SuIM/proto/relationpb"
)

func (s *Server) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	var req relationpb.SendFriendRequestReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetFromUserId()) {
		return
	}
	resp, err := s.friends.SendFriendRequest(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) respondFriendApply(w http.ResponseWriter, r *http.Request) {
	var req relationpb.RespondFriendApplyReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetToUserId()) {
		return
	}
	resp, err := s.friends.RespondFriendApply(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) deleteFriend(w http.ResponseWriter, r *http.Request) {
	var req relationpb.DeleteFriendReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.friends.DeleteFriend(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) updateFriend(w http.ResponseWriter, r *http.Request) {
	var req relationpb.UpdateFriendReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetOwnerUserId()) {
		return
	}
	resp, err := s.friends.UpdateFriend(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) isFriend(w http.ResponseWriter, r *http.Request) {
	var req relationpb.IsFriendReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireEither(w, r, req.GetUser1(), req.GetUser2()) {
		return
	}
	resp, err := s.friends.IsFriend(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) blockUser(w http.ResponseWriter, r *http.Request) {
	var req relationpb.BlockUserReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.friends.BlockUser(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) unblockUser(w http.ResponseWriter, r *http.Request) {
	var req relationpb.UnblockUserReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.friends.UnblockUser(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) isBlack(w http.ResponseWriter, r *http.Request) {
	var req relationpb.IsBlackReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireEither(w, r, req.GetUser1(), req.GetUser2()) {
		return
	}
	resp, err := s.friends.IsBlack(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}
