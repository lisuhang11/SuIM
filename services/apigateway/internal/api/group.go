package api

import (
	"net/http"

	"github.com/lisuhang11/SuIM/proto/group"
)

// 创建群聊
func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var req grouppb.CreateGroupReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	// 请求中的 CreatorUserId 必须与当前登录用户一致
	if !s.requireSelf(w, r, req.GetCreatorUserId()) {
		return
	}
	// 调用rpc创建群聊
	resp, err := s.groups.CreateGroup(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

// 解散群聊
func (s *Server) dismissGroup(w http.ResponseWriter, r *http.Request) {
	var req grouppb.DismissGroupReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	// 请求中的 OpUserId 必须与当前登录用户一致
	if !s.requireSelf(w, r, req.GetOpUserId()) {
		return
	}
	// 调用rpc解散群聊
	resp, err := s.groups.DismissGroup(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

// 转让群主
func (s *Server) transferGroupOwner(w http.ResponseWriter, r *http.Request) {
	var req grouppb.TransferGroupOwnerReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	// 请求中的 OpUserId 必须与当前登录用户一致
	if !s.requireSelf(w, r, req.GetOpUserId()) {
		return
	}
	// 调用rpc转让群主
	resp, err := s.groups.TransferGroupOwner(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

// 更新群信息
func (s *Server) updateGroupInfo(w http.ResponseWriter, r *http.Request) {
	var req grouppb.UpdateGroupInfoReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	// 请求中的 OpUserId 必须与当前登录用户一致
	if !s.requireSelf(w, r, req.GetOpUserId()) {
		return
	}
	// 调用rpc更新群信息
	resp, err := s.groups.UpdateGroupInfo(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}
func (s *Server) getGroupsInfo(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetGroupsInfoReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.groups.GetGroupsInfo(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) inviteUserToGroup(w http.ResponseWriter, r *http.Request) {
	var req grouppb.InviteUserToGroupReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetOpUserId()) {
		return
	}
	resp, err := s.groups.InviteUserToGroup(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) kickGroupMember(w http.ResponseWriter, r *http.Request) {
	var req grouppb.KickGroupMemberReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetOpUserId()) {
		return
	}
	resp, err := s.groups.KickGroupMember(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) quitGroup(w http.ResponseWriter, r *http.Request) {
	var req grouppb.QuitGroupReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.groups.QuitGroup(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) getGroupMembers(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetGroupMembersReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.groups.GetGroupMembers(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) getGroupMemberUserIDs(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetGroupMemberUserIDsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.groups.GetGroupMemberUserIDs(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) incrementalJoinedGroups(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetIncrementalJoinGroupsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.groups.GetIncrementalJoinGroups(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) fullJoinGroupIDs(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetFullJoinGroupIDsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.groups.GetFullJoinGroupIDs(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) incrementalGroupMembers(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetIncrementalGroupMembersReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.groups.GetIncrementalGroupMembers(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) batchIncrementalGroupMembers(w http.ResponseWriter, r *http.Request) {
	var req grouppb.BatchGetIncrementalGroupMembersReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.groups.BatchGetIncrementalGroupMembers(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) fullGroupMemberUserIDs(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetFullGroupMemberUserIDsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	resp, err := s.groups.GetFullGroupMemberUserIDs(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}

func (s *Server) getJoinedGroups(w http.ResponseWriter, r *http.Request) {
	var req grouppb.GetJoinedGroupsReq
	if err := decode(w, r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if !s.requireSelf(w, r, req.GetUserId()) {
		return
	}
	resp, err := s.groups.GetJoinedGroups(r.Context(), &req)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, resp)
}
