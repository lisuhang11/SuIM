package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/proto/userpb"
)

type userStub struct {
	userpb.UnimplementedUserServiceServer
	loggedOut string
}

func (s *userStub) Register(context.Context, *userpb.RegisterReq) (*userpb.RegisterResp, error) {
	return &userpb.RegisterResp{User: &userpb.UserInfo{UserId: "new", Nickname: "Ada"}}, nil
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
	if req.GetKeyword() == "missing" {
		return &userpb.SearchUsersResp{Users: []*userpb.UserInfo{}}, nil
	}
	return &userpb.SearchUsersResp{Users: []*userpb.UserInfo{{UserId: req.GetKeyword(), Nickname: "Ada"}}}, nil
}

func (s *userStub) GetUserByID(_ context.Context, req *userpb.GetUserByIDReq) (*userpb.GetUserByIDResp, error) {
	if req.GetUserId() == "missing" {
		return nil, errorcode.ErrUserNotFound
	}
	return &userpb.GetUserByIDResp{User: &userpb.UserInfo{UserId: req.GetUserId(), Nickname: "Ada"}}, nil
}

func (s *userStub) Logout(_ context.Context, req *userpb.LogoutReq) (*userpb.LogoutResp, error) {
	s.loggedOut = req.GetToken()
	return &userpb.LogoutResp{}, nil
}

func (s *userStub) UpdateUser(context.Context, *userpb.UpdateUserReq) (*userpb.UpdateUserResp, error) {
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
	uploads := &thirdStub{}
	userLis := serve(t, func(gs *grpc.Server) { userpb.RegisterUserServiceServer(gs, users) })
	relLis := serve(t, func(gs *grpc.Server) { relationpb.RegisterRelationServiceServer(gs, friends) })
	groupLis := serve(t, func(gs *grpc.Server) { grouppb.RegisterGroupServiceServer(gs, groups) })
	thirdLis := serve(t, func(gs *grpc.Server) { thirdpb.RegisterThirdServiceServer(gs, uploads) })
	userConn := dial(t, userLis)
	relConn := dial(t, relLis)
	groupConn := dial(t, groupLis)
	thirdConn := dial(t, thirdLis)
	httpSrv := httptest.NewServer(New(
		userpb.NewUserServiceClient(userConn),
		relationpb.NewRelationServiceClient(relConn),
		grouppb.NewGroupServiceClient(groupConn),
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
	res = post(t, httpSrv.URL+"/user/get", "good", `{"user_id":"u1"}`)
	if res.Status != http.StatusOK || res.Code != 0 || !bytes.Contains(res.Data, []byte(`"user_id":"u1"`)) {
		t.Fatalf("get user %+v", res)
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
		req.Header.Set("token", token)
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

func dial(t *testing.T, lis net.Listener) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
