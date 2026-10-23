package api

import (
	"net/http"

	"github.com/lisuhang11/SuIM/proto/userpb"
)

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req userpb.RegisterReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.users.Register(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req userpb.LoginReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.users.Login(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	var req userpb.GetUserByIDReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.users.GetUserByID(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) {
	var req userpb.SearchUsersReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.users.SearchUsers(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) getUsers(w http.ResponseWriter, r *http.Request) {
	var req userpb.GetUsersByIDsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.users.GetUsersByIDs(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	var req userpb.UpdateUserReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	userID, ok := s.actor(w, r, req.GetUserId())
	if !ok {
		return
	}
	req.UserId = userID
	resp, err := s.users.UpdateUser(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	var req userpb.DeleteUserReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.users.DeleteUser(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var req userpb.ChangePasswordReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.users.ChangePassword(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	resp, err := s.users.Logout(r.Context(), &userpb.LogoutReq{Token: current(r).token})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}
