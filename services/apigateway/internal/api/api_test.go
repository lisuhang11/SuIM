package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/mw"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/proto/relation"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/proto/user"
)

type userStub struct {
	userpb.UnimplementedUserServiceServer
	loggedOut  string
	smsIP      string
	registerIP string
	seenToken  string
	seenUser   string
	loginPhone string
}

func (s *userStub) Register(context.Context, *userpb.RegisterReq) (*userpb.RegisterResp, error) {
	return &userpb.RegisterResp{User: &userpb.UserInfo{UserId: "new", Nickname: "Ada"}}, nil
}

func (s *userStub) SendSmsCode(_ context.Context, req *userpb.SendSmsCodeReq) (*userpb.SendSmsCodeResp, error) {
	s.smsIP = req.GetClientIp()
	return &userpb.SendSmsCodeResp{ExpireSeconds: 300}, nil
}

func (s *userStub) RegisterPhone(_ context.Context, req *userpb.RegisterPhoneReq) (*userpb.RegisterPhoneResp, error) {
	s.registerIP = req.GetClientIp()
	return &userpb.RegisterPhoneResp{User: &userpb.UserInfo{UserId: "p1"}, Token: "good", ExpireTimeSeconds: 10}, nil
}

func (s *userStub) LoginPhone(_ context.Context, req *userpb.LoginPhoneReq) (*userpb.LoginResp, error) {
	s.loginPhone = req.GetPhone() + ":" + req.GetCode()
	return &userpb.LoginResp{User: &userpb.UserInfo{UserId: "p1"}, Token: "phone", ExpireTimeSeconds: 10}, nil
}

func (s *userStub) Login(context.Context, *userpb.LoginReq) (*userpb.LoginResp, error) {
	return &userpb.LoginResp{User: &userpb.UserInfo{UserId: "u1"}, Token: "good", ExpireTimeSeconds: 10}, nil
}

func (s *userStub) ValidateToken(_ context.Context, req *userpb.ValidateTokenReq) (*userpb.ValidateTokenResp, error) {
	if req.GetToken() != "good" {
		return nil, errorcode.ErrUserInvalidToken
	}
	return &userpb.ValidateTokenResp{Valid: true, User: &userpb.UserInfo{UserId: "u1"}, PlatformId: 1}, nil
}

func (s *userStub) SearchUsers(_ context.Context, req *userpb.SearchUsersReq) (*userpb.SearchUsersResp, error) {
	s.seenToken = req.GetToken()
	if req.GetKeyword() == "missing" {
		return &userpb.SearchUsersResp{Users: []*userpb.UserInfo{}}, nil
	}
	return &userpb.SearchUsersResp{Users: []*userpb.UserInfo{{UserId: req.GetKeyword(), Nickname: "Ada"}}}, nil
}

func (s *userStub) GetUserByID(_ context.Context, req *userpb.GetUserByIDReq) (*userpb.GetUserByIDResp, error) {
	s.seenToken = req.GetToken()
	if req.GetUserId() == "missing" {
		return nil, errorcode.ErrUserNotFound
	}
	return &userpb.GetUserByIDResp{User: &userpb.UserInfo{UserId: req.GetUserId(), Nickname: "Ada"}}, nil
}

func (s *userStub) Logout(_ context.Context, req *userpb.LogoutReq) (*userpb.LogoutResp, error) {
	s.loggedOut = req.GetToken()
	return &userpb.LogoutResp{}, nil
}

func (s *userStub) UpdateUser(_ context.Context, req *userpb.UpdateUserReq) (*userpb.UpdateUserResp, error) {
	s.seenToken = req.GetToken()
	s.seenUser = req.GetUserId()
	return &userpb.UpdateUserResp{}, nil
}

type relationStub struct {
	relationpb.UnimplementedRelationServiceServer
	sentFrom string
}

func (s *relationStub) GetRecvFriendRequests(_ context.Context, req *relationpb.GetRecvFriendRequestsReq) (*relationpb.GetRecvFriendRequestsResp, error) {
	return &relationpb.GetRecvFriendRequestsResp{Requests: []*relationpb.FriendRequestInfo{{FromUserId: "u2", ToUserId: req.GetUserId(), Message: "hi"}}, Total: 1}, nil
}

func (s *relationStub) GetSentFriendRequests(_ context.Context, req *relationpb.GetSentFriendRequestsReq) (*relationpb.GetSentFriendRequestsResp, error) {
	return &relationpb.GetSentFriendRequestsResp{Requests: []*relationpb.FriendRequestInfo{{FromUserId: req.GetUserId(), ToUserId: "u2"}}, Total: 1}, nil
}

func (s *relationStub) SendFriendRequest(_ context.Context, req *relationpb.SendFriendRequestReq) (*relationpb.SendFriendRequestResp, error) {
	s.sentFrom = req.GetFromUserId()
	return &relationpb.SendFriendRequestResp{}, nil
}

func (s *relationStub) IsFriend(context.Context, *relationpb.IsFriendReq) (*relationpb.IsFriendResp, error) {
	return &relationpb.IsFriendResp{InUser1Friends: true}, nil
}

func (s *relationStub) IsBlack(_ context.Context, req *relationpb.IsBlackReq) (*relationpb.IsBlackResp, error) {
	if req.GetUser2() == "blocked" {
		return nil, errorcode.ErrRelationAlreadyBlocked
	}
	return &relationpb.IsBlackResp{InUser1Blacklist: true}, nil
}

type groupStub struct {
	grouppb.UnimplementedGroupServiceServer
	creator string
}

func (s *groupStub) CreateGroup(_ context.Context, req *grouppb.CreateGroupReq) (*grouppb.CreateGroupResp, error) {
	s.creator = req.GetCreatorUserId()
	return &grouppb.CreateGroupResp{GroupId: "g1"}, nil
}

type conversationStub struct {
	conversationpb.UnimplementedConversationServiceServer
	userIDs []string
}

func (s *conversationStub) SetConversations(_ context.Context, req *conversationpb.SetConversationsReq) (*conversationpb.SetConversationsResp, error) {
	s.userIDs = append([]string(nil), req.GetUserIds()...)
	return &conversationpb.SetConversationsResp{}, nil
}

type thirdStub struct {
	thirdpb.UnimplementedThirdServiceServer
	userID     string
	formUserID string
}

func (s *thirdStub) InitiateMultipartUpload(_ context.Context, req *thirdpb.InitiateMultipartUploadReq) (*thirdpb.InitiateMultipartUploadResp, error) {
	s.userID = req.GetUserId()
	return &thirdpb.InitiateMultipartUploadResp{Url: "https://cdn/" + req.GetName()}, nil
}

func (s *thirdStub) InitiateFormData(_ context.Context, req *thirdpb.InitiateFormDataReq) (*thirdpb.InitiateFormDataResp, error) {
	s.formUserID = req.GetUserId()
	return &thirdpb.InitiateFormDataResp{Id: "ticket", Url: "http://127.0.0.1:9000/suim", File: "file"}, nil
}

func TestGateway(t *testing.T) {
	users := &userStub{}
	friends := &relationStub{}
	groups := &groupStub{}
	conversations := &conversationStub{}
	uploads := &thirdStub{}
	userLis := serve(t, func(gs *grpc.Server) { userpb.RegisterUserServiceServer(gs, users) })
	relLis := serve(t, func(gs *grpc.Server) { relationpb.RegisterRelationServiceServer(gs, friends) })
	groupLis := serve(t, func(gs *grpc.Server) { grouppb.RegisterGroupServiceServer(gs, groups) })
	conversationLis := serve(t, func(gs *grpc.Server) { conversationpb.RegisterConversationServiceServer(gs, conversations) })
	thirdLis := serve(t, func(gs *grpc.Server) { thirdpb.RegisterThirdServiceServer(gs, uploads) })
	userConn := dial(t, userLis)
	relConn := dial(t, relLis)
	groupConn := dial(t, groupLis)
	conversationConn := dial(t, conversationLis)
	thirdConn := dial(t, thirdLis)
	httpSrv := httptest.NewServer(New(
		userpb.NewUserServiceClient(userConn),
		relationpb.NewRelationServiceClient(relConn),
		grouppb.NewGroupServiceClient(groupConn),
		conversationpb.NewConversationServiceClient(conversationConn),
		thirdpb.NewThirdServiceClient(thirdConn),
	).Handler())
	t.Cleanup(httpSrv.Close)

	res := post(t, httpSrv.URL+"/user/register", "", `{"username":"Ada","email":"a@b.c","password":"secret1"}`)
	if res.Code != 0 || res.Status != http.StatusOK {
		t.Fatalf("register %+v", res)
	}

	res = post(t, httpSrv.URL+"/user/get", "", `{"user_id":"u1"}`)
	if res.Status != http.StatusUnauthorized || res.Code != int32(errorcode.TokenRequired) {
		t.Fatalf("missing token %+v", res)
	}
	res = post(t, httpSrv.URL+"/user/get", "bad", `{"user_id":"u1"}`)
	if res.Status != http.StatusUnauthorized || res.Code != int32(errorcode.UserInvalidToken) {
		t.Fatalf("bad token %+v", res)
	}
	res = post(t, httpSrv.URL+"/user/get", "good", `{"user_id":"missing"}`)
	if res.Status != http.StatusNotFound || res.Code != int32(errorcode.UserNotFound) {
		t.Fatalf("missing user %+v", res)
	}
	res = post(t, httpSrv.URL+"/user/get", "good", `{"user_id":"u1","token":"spoof"}`)
	if res.Status != http.StatusOK || res.Code != 0 || !bytes.Contains(res.Data, []byte(`"user_id":"u1"`)) || users.seenToken != "good" {
		t.Fatalf("get user %+v token=%s", res, users.seenToken)
	}
	res = post(t, httpSrv.URL+"/user/search", "good", `{"keyword":"missing"}`)
	if res.Status != http.StatusOK || res.Code != 0 || bytes.Contains(res.Data, []byte(`"user_id"`)) {
		t.Fatalf("search empty %+v", res)
	}
	res = post(t, httpSrv.URL+"/user/search", "good", `{"keyword":"ada@example.com"}`)
	if res.Status != http.StatusOK || !bytes.Contains(res.Data, []byte(`"user_id":"ada@example.com"`)) {
		t.Fatalf("search %+v", res)
	}

	res = post(t, httpSrv.URL+"/user/update", "good", `{"user_id":"u2","nickname":"x"}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("update other %+v", res)
	}
	res = post(t, httpSrv.URL+"/user/update", "good", `{"user_id":"u1","nickname":"x","token":"spoof"}`)
	if res.Status != http.StatusOK || res.Code != 0 || users.seenToken != "good" || users.seenUser != "u1" {
		t.Fatalf("update self %+v token=%s user=%s", res, users.seenToken, users.seenUser)
	}
	res = post(t, httpSrv.URL+"/friend/send_request", "good", `{"from_user_id":"u2","to_user_id":"u1","message":"hi"}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("send as other %+v", res)
	}
	res = post(t, httpSrv.URL+"/friend/send_request", "good", `{"from_user_id":"u1","to_user_id":"u2","message":"hi"}`)
	if res.Status != http.StatusOK || friends.sentFrom != "u1" {
		t.Fatalf("send %+v from=%s", res, friends.sentFrom)
	}
	res = post(t, httpSrv.URL+"/friend/recv_requests", "good", `{"user_id":"u2"}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("recv other %+v", res)
	}
	res = post(t, httpSrv.URL+"/friend/recv_requests", "good", `{"user_id":"u1"}`)
	if res.Status != http.StatusOK || !bytes.Contains(res.Data, []byte(`"message":"hi"`)) {
		t.Fatalf("recv %+v", res)
	}
	res = post(t, httpSrv.URL+"/friend/sent_requests", "good", `{"user_id":"u1"}`)
	if res.Status != http.StatusOK || !bytes.Contains(res.Data, []byte(`"to_user_id":"u2"`)) {
		t.Fatalf("sent %+v", res)
	}
	res = post(t, httpSrv.URL+"/friend/is_friend", "good", `{"user1":"u2","user2":"u3"}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("is friend other %+v", res)
	}
	res = post(t, httpSrv.URL+"/friend/is_friend", "good", `{"user1":"u1","user2":"u2"}`)
	if res.Status != http.StatusOK || !bytes.Contains(res.Data, []byte(`"in_user1_friends":true`)) {
		t.Fatalf("is friend %+v", res)
	}
	res = post(t, httpSrv.URL+"/friend/is_black", "good", `{"user1":"u1","user2":"blocked"}`)
	if res.Status != http.StatusConflict || res.Code != int32(errorcode.RelationAlreadyBlocked) {
		t.Fatalf("is black %+v", res)
	}
	res = post(t, httpSrv.URL+"/user/logout", "good", `{}`)
	if res.Status != http.StatusOK || users.loggedOut != "good" {
		t.Fatalf("logout %+v token=%s", res, users.loggedOut)
	}
	res = post(t, httpSrv.URL+"/group/create", "good", `{"creator_user_id":"u2","group_name":"g","group_type":2}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("create as other %+v", res)
	}
	res = post(t, httpSrv.URL+"/group/create", "good", `{"creator_user_id":"u1","group_name":"g","group_type":2}`)
	if res.Status != http.StatusOK || groups.creator != "u1" {
		t.Fatalf("create %+v creator=%s", res, groups.creator)
	}
	res = post(t, httpSrv.URL+"/object/initiate_multipart_upload", "", `{"name":"u1/a.bin","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":1}`)
	if res.Status != http.StatusUnauthorized || res.Code != int32(errorcode.TokenRequired) {
		t.Fatalf("upload missing token %+v", res)
	}
	res = post(t, httpSrv.URL+"/object/initiate_multipart_upload", "good", `{"user_id":"u2","name":"u1/a.bin","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":1}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("upload as other %+v", res)
	}
	res = post(t, httpSrv.URL+"/object/initiate_multipart_upload", "good", `{"name":"u1/a.bin","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":1}`)
	if res.Status != http.StatusOK || uploads.userID != "u1" || !bytes.Contains(res.Data, []byte(`"url":"https://cdn/u1/a.bin"`)) {
		t.Fatalf("upload %+v user=%s", res, uploads.userID)
	}
	res = post(t, httpSrv.URL+"/object/initiate_form_data", "", `{"name":"u1/a.bin","size":1}`)
	if res.Status != http.StatusUnauthorized || res.Code != int32(errorcode.TokenRequired) {
		t.Fatalf("form missing token %+v", res)
	}
	res = post(t, httpSrv.URL+"/object/initiate_form_data", "good", `{"user_id":"u2","name":"u1/a.bin","size":1}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("form as other %+v", res)
	}
	res = post(t, httpSrv.URL+"/object/initiate_form_data", "good", `{"name":"u1/a.bin","size":1}`)
	if res.Status != http.StatusOK || uploads.formUserID != "u1" || !bytes.Contains(res.Data, []byte(`"file":"file"`)) {
		t.Fatalf("form %+v user=%s", res, uploads.formUserID)
	}
	res = post(t, httpSrv.URL+"/conversation/set", "good", `{"user_ids":["u2"],"conversation":{"conversation_id":"si_u1_u2","conversation_type":1,"user_id":"u2"}}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("set conversation as other %+v", res)
	}
	res = post(t, httpSrv.URL+"/conversation/set", "good", `{"user_ids":["u1"],"conversation":{"conversation_id":"si_u1_u2","conversation_type":1,"user_id":"u2","is_pinned":true}}`)
	if res.Status != http.StatusOK || len(conversations.userIDs) != 1 || conversations.userIDs[0] != "u1" {
		t.Fatalf("set conversation %+v ids=%v", res, conversations.userIDs)
	}
	res = post(t, httpSrv.URL+"/conversation/full_ids", "good", `{"user_id":"u2"}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("full conversation ids as other %+v", res)
	}
	res = post(t, httpSrv.URL+"/conversation/incremental", "good", `{"user_id":"u2"}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("incremental conversation as other %+v", res)
	}
	res = post(t, httpSrv.URL+"/user/login_phone", "", `{"phone":"13800138000","code":"123456","platform_id":1}`)
	if res.Status != http.StatusOK || res.Code != 0 || users.loginPhone != "13800138000:123456" || !bytes.Contains(res.Data, []byte(`"token":"phone"`)) {
		t.Fatalf("login phone %+v code=%s", res, users.loginPhone)
	}
	res = post(t, httpSrv.URL+"/user/send_sms_code", "", `{"phone":"13800138000","scene":"register","client_ip":"1.2.3.4"}`)
	if res.Status != http.StatusOK || res.Code != 0 || users.smsIP != "127.0.0.1" || !bytes.Contains(res.Data, []byte(`"expire_seconds":"300"`)) && !bytes.Contains(res.Data, []byte(`"expire_seconds":300`)) {
		t.Fatalf("send sms %+v ip=%s", res, users.smsIP)
	}
	res = post(t, httpSrv.URL+"/user/register_phone", "", `{"phone":"13800138000","code":"123456","scene":"register","agree_protocol":true,"protocol_version":"v1","platform_id":1,"client_ip":"1.2.3.4"}`)
	if res.Status != http.StatusOK || res.Code != 0 || users.registerIP != "127.0.0.1" {
		t.Fatalf("register phone %+v ip=%s", res, users.registerIP)
	}
	res = post(t, httpSrv.URL+"/user/login", "", `{`)
	if res.Status != http.StatusBadRequest || res.Code != int32(errorcode.InvalidJSON) {
		t.Fatalf("bad json %+v", res)
	}

	health, err := http.Get(httpSrv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer health.Body.Close()
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health %d", health.StatusCode)
	}
}

type result struct {
	Status int
	Code   int32
	Msg    string
	Data   []byte
}

func post(t *testing.T, url, token, body string) result {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env struct {
		Code int32           `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return result{Status: resp.StatusCode, Code: env.Code, Msg: env.Msg, Data: env.Data}
}

func serve(t *testing.T, register func(*grpc.Server)) net.Listener {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer()
	register(gs)
	go gs.Serve(lis)
	t.Cleanup(func() {
		gs.Stop()
		_ = lis.Close()
	})
	return lis
}

func TestGzipHealth(t *testing.T) {
	h := mw.GzipHandler(0, New(nil, nil, nil, nil, nil).Handler())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Encoding") != "gzip" || rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("status %d encoding %q origin %q", rr.Code, rr.Header().Get("Content-Encoding"), rr.Header().Get("Access-Control-Allow-Origin"))
	}
	zr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"status":"ok"`)) {
		t.Fatalf("body %s", body)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := New(nil, nil, nil, nil, nil).Handler()
	req := httptest.NewRequest(http.MethodOptions, "/user/get", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight %d origin %q body %s", rr.Code, rr.Header().Get("Access-Control-Allow-Origin"), rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/user/get", bytes.NewBufferString(`{}`))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized || rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing token %d origin %q", rr.Code, rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func dial(t *testing.T, lis net.Listener) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func testHTTP(t *testing.T, users userpb.UserServiceServer, friends relationpb.RelationServiceServer, groups grouppb.GroupServiceServer, conversations conversationpb.ConversationServiceServer, third thirdpb.ThirdServiceServer) string {
	t.Helper()
	if users == nil {
		users = &userStub{}
	}
	if friends == nil {
		friends = &relationStub{}
	}
	if groups == nil {
		groups = &groupStub{}
	}
	if conversations == nil {
		conversations = &conversationStub{}
	}
	if third == nil {
		third = &thirdStub{}
	}
	userLis := serve(t, func(gs *grpc.Server) { userpb.RegisterUserServiceServer(gs, users) })
	relLis := serve(t, func(gs *grpc.Server) { relationpb.RegisterRelationServiceServer(gs, friends) })
	groupLis := serve(t, func(gs *grpc.Server) { grouppb.RegisterGroupServiceServer(gs, groups) })
	conversationLis := serve(t, func(gs *grpc.Server) { conversationpb.RegisterConversationServiceServer(gs, conversations) })
	thirdLis := serve(t, func(gs *grpc.Server) { thirdpb.RegisterThirdServiceServer(gs, third) })
	httpSrv := httptest.NewServer(New(
		userpb.NewUserServiceClient(dial(t, userLis)),
		relationpb.NewRelationServiceClient(dial(t, relLis)),
		grouppb.NewGroupServiceClient(dial(t, groupLis)),
		conversationpb.NewConversationServiceClient(dial(t, conversationLis)),
		thirdpb.NewThirdServiceClient(dial(t, thirdLis)),
	).Handler())
	t.Cleanup(httpSrv.Close)
	return httpSrv.URL
}

// Empty peer must not waive the "caller is one side" check — otherwise any
// authed user can probe is_friend/is_black for unrelated accounts.
func TestRequireEitherRejectsForeignEmptyPeer(t *testing.T) {
	base := testHTTP(t, nil, nil, nil, nil, nil)
	res := post(t, base+"/friend/is_friend", "good", `{"user1":"u2","user2":""}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("empty user2 bypass %+v", res)
	}
	res = post(t, base+"/friend/is_black", "good", `{"user1":"u2","user2":""}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("empty user2 black bypass %+v", res)
	}
	res = post(t, base+"/friend/is_friend", "good", `{"user1":"u1","user2":""}`)
	if res.Status != http.StatusOK || res.Code != 0 {
		t.Fatalf("self with empty peer should still pass %+v", res)
	}
}

// user_ids entries that trim to empty are not "omitted" — reject them like a foreign owner.
func TestSetConversationRejectsEmptyOwnerID(t *testing.T) {
	conversations := &conversationStub{}
	base := testHTTP(t, nil, nil, nil, conversations, nil)
	res := post(t, base+"/conversation/set", "good",
		`{"user_ids":[""],"conversation":{"conversation_id":"si_u1_u2","conversation_type":1,"user_id":"u2","is_pinned":true}}`)
	if res.Status != http.StatusForbidden || res.Code != int32(errorcode.NoPermission) {
		t.Fatalf("empty owner id %+v ids=%v", res, conversations.userIDs)
	}
	if len(conversations.userIDs) != 0 {
		t.Fatalf("should not forward empty owners: %v", conversations.userIDs)
	}
}
