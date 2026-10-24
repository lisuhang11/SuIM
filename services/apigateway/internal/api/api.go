// Package api is the HTTP entry in front of the user, relation, group, and third services.
//
// Public:
//
//	POST /user/register
//	POST /user/login
//
// Token required. Send it in the token header, or as Authorization: Bearer.
// The token user must be the actor in the request body.
//
//	POST /user/get
//	POST /user/get_users
//	POST /user/search
//	POST /user/update
//	POST /user/delete
//	POST /user/change_password
//	POST /user/logout
//	POST /friend/send_request
//	POST /friend/respond
//	POST /friend/recv_requests
//	POST /friend/sent_requests
//	POST /friend/delete
//	POST /friend/update
//	POST /friend/is_friend
//	POST /friend/block
//	POST /friend/unblock
//	POST /friend/is_black
//	POST /friend/incremental_friends
//	POST /friend/list
//	POST /friend/friend_ids
//	POST /friend/blacks
//	POST /group/create
//	POST /group/dismiss
//	POST /group/transfer
//	POST /group/update
//	POST /group/get
//	POST /group/get_groups
//	POST /group/invite
//	POST /group/kick
//	POST /group/quit
//	POST /group/members
//	POST /group/member_ids
//	POST /group/joined
//	POST /group/incremental_joined
//	POST /group/joined_ids
//	POST /group/incremental_members
//	POST /group/incremental_members_batch
//	POST /group/full_member_ids
//	POST /object/part_limit
//	POST /object/initiate_multipart_upload
//	POST /object/auth_sign
//	POST /object/complete_multipart_upload
//	POST /object/initiate_form_data
//	POST /object/complete_form_data
//
// Part bytes are PUT to the presigned MinIO URL returned by third.
// A form upload POSTs the file to the MinIO URL in initiate_form_data.
// This gateway does not receive the file body.
//
// Bodies and data use the protobuf JSON names, for example user_id.
// Responses are {"code":0,"msg":"","data":{...}}. code 0 is success.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/proto/userpb"
)

const maxBody = 1 << 20

type session struct {
	userID string
	token  string
}

type ctxKey struct{}

type Server struct {
	users   userpb.UserServiceClient
	friends relationpb.RelationServiceClient
	groups  grouppb.GroupServiceClient
	third   thirdpb.ThirdServiceClient
}

func New(users userpb.UserServiceClient, friends relationpb.RelationServiceClient, groups grouppb.GroupServiceClient, third thirdpb.ThirdServiceClient) *Server {
	return &Server{users: users, friends: friends, groups: groups, third: third}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)

	mux.HandleFunc("POST /user/register", s.register)
	mux.HandleFunc("POST /user/login", s.login)

	mux.HandleFunc("POST /user/get", s.auth(s.getUser))
	mux.HandleFunc("POST /user/get_users", s.auth(s.getUsers))
	mux.HandleFunc("POST /user/search", s.auth(s.searchUsers))
	mux.HandleFunc("POST /user/update", s.auth(s.updateUser))
	mux.HandleFunc("POST /user/delete", s.auth(s.deleteUser))
	mux.HandleFunc("POST /user/change_password", s.auth(s.changePassword))
	mux.HandleFunc("POST /user/logout", s.auth(s.logout))

	mux.HandleFunc("POST /friend/send_request", s.auth(s.sendFriendRequest))
	mux.HandleFunc("POST /friend/respond", s.auth(s.respondFriendApply))
	mux.HandleFunc("POST /friend/recv_requests", s.auth(s.recvFriendRequests))
	mux.HandleFunc("POST /friend/sent_requests", s.auth(s.sentFriendRequests))
	mux.HandleFunc("POST /friend/delete", s.auth(s.deleteFriend))
	mux.HandleFunc("POST /friend/update", s.auth(s.updateFriend))
	mux.HandleFunc("POST /friend/is_friend", s.auth(s.isFriend))
	mux.HandleFunc("POST /friend/block", s.auth(s.blockUser))
	mux.HandleFunc("POST /friend/unblock", s.auth(s.unblockUser))
	mux.HandleFunc("POST /friend/is_black", s.auth(s.isBlack))
	mux.HandleFunc("POST /friend/incremental_friends", s.auth(s.incrementalFriends))
	mux.HandleFunc("POST /friend/list", s.auth(s.friendList))
	mux.HandleFunc("POST /friend/friend_ids", s.auth(s.fullFriendUserIDs))
	mux.HandleFunc("POST /friend/blacks", s.auth(s.blackList))

	mux.HandleFunc("POST /group/create", s.auth(s.createGroup))
	mux.HandleFunc("POST /group/dismiss", s.auth(s.dismissGroup))
	mux.HandleFunc("POST /group/transfer", s.auth(s.transferGroupOwner))
	mux.HandleFunc("POST /group/update", s.auth(s.updateGroupInfo))
	mux.HandleFunc("POST /group/get_groups", s.auth(s.getGroupsInfo))
	mux.HandleFunc("POST /group/invite", s.auth(s.inviteUserToGroup))
	mux.HandleFunc("POST /group/kick", s.auth(s.kickGroupMember))
	mux.HandleFunc("POST /group/quit", s.auth(s.quitGroup))
	mux.HandleFunc("POST /group/members", s.auth(s.getGroupMembers))
	mux.HandleFunc("POST /group/member_ids", s.auth(s.getGroupMemberUserIDs))
	mux.HandleFunc("POST /group/joined", s.auth(s.getJoinedGroups))
	mux.HandleFunc("POST /group/incremental_joined", s.auth(s.incrementalJoinedGroups))
	mux.HandleFunc("POST /group/joined_ids", s.auth(s.fullJoinGroupIDs))
	mux.HandleFunc("POST /group/incremental_members", s.auth(s.incrementalGroupMembers))
	mux.HandleFunc("POST /group/incremental_members_batch", s.auth(s.batchIncrementalGroupMembers))
	mux.HandleFunc("POST /group/full_member_ids", s.auth(s.fullGroupMemberUserIDs))

	mux.HandleFunc("POST /object/part_limit", s.auth(s.partLimit))
	mux.HandleFunc("POST /object/initiate_multipart_upload", s.auth(s.initiateMultipartUpload))
	mux.HandleFunc("POST /object/auth_sign", s.auth(s.authSign))
	mux.HandleFunc("POST /object/complete_multipart_upload", s.auth(s.completeMultipartUpload))
	mux.HandleFunc("POST /object/initiate_form_data", s.auth(s.initiateFormData))
	mux.HandleFunc("POST /object/complete_form_data", s.auth(s.completeFormData))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		mux.ServeHTTP(sw, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.code, "dur", time.Since(start))
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, map[string]string{"status": "ok"})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := tokenFrom(r)
		if token == "" {
			writeErr(w, errorcode.ErrTokenRequired)
			return
		}
		resp, err := s.users.ValidateToken(r.Context(), &userpb.ValidateTokenReq{Token: token})
		if err != nil {
			writeErr(w, err)
			return
		}
		userID := resp.GetUser().GetUserId()
		if userID == "" {
			writeErr(w, errorcode.ErrUserInvalidToken)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, session{userID: userID, token: token})
		next(w, r.WithContext(ctx))
	}
}

func tokenFrom(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("token")); token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return ""
}

func current(r *http.Request) session {
	v, _ := r.Context().Value(ctxKey{}).(session)
	return v
}

func (s *Server) actor(w http.ResponseWriter, r *http.Request, id string) (string, bool) {
	self := current(r).userID
	if id != "" && id != self {
		writeErr(w, errorcode.ErrNoPermission)
		return "", false
	}
	return self, true
}

func (s *Server) requireSelf(w http.ResponseWriter, r *http.Request, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || id == current(r).userID {
		return true
	}
	writeErr(w, errorcode.ErrNoPermission)
	return false
}

func (s *Server) requireEither(w http.ResponseWriter, r *http.Request, a, b string) bool {
	self := current(r).userID
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == self || b == self || a == "" || b == "" {
		return true
	}
	writeErr(w, errorcode.ErrNoPermission)
	return false
}

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errorcode.ErrInvalidJSON.WithMsg("request body is too large")
		}
		return errorcode.ErrInvalidJSON
	}
	return nil
}

type envelope struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func writeOK(w http.ResponseWriter, data any) {
	write(w, http.StatusOK, 0, "", data)
}

func writeErr(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		st = status.New(codes.Internal, "internal error")
	}
	code, found := errorcode.CodeOf(err)
	if !found {
		code = errorcode.Internal
	}
	write(w, httpStatus(st.Code()), int32(code), st.Message(), nil)
}

func write(w http.ResponseWriter, statusCode int, code int32, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Msg: msg, Data: data})
}

func httpStatus(code codes.Code) int {
	switch code {
	case codes.InvalidArgument, codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.Canceled, codes.DeadlineExceeded:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}
