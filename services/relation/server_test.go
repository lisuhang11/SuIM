package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/logic"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

type knownUsers map[string]struct{}

func (u knownUsers) CheckUsers(_ context.Context, ids []string) error {
	for _, id := range ids {
		if _, ok := u[id]; !ok {
			return errorcode.ErrRelationUserNotFound
		}
	}
	return nil
}

func (u knownUsers) GetUsers(_ context.Context, ids []string) (map[string]model.UserProfile, error) {
	out := make(map[string]model.UserProfile, len(ids))
	for _, id := range ids {
		if _, ok := u[id]; !ok {
			continue
		}
		out[id] = model.UserProfile{UserID: id, Nickname: id}
	}
	return out, nil
}

func TestFriendApply(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}

	from := fmt.Sprintf("from-%d", time.Now().UnixNano())
	to := fmt.Sprintf("to-%d", time.Now().UnixNano())
	t.Cleanup(func() { cleanupRelation(t, mysqlCfg, from, to) })

	srv := logic.New(st, knownUsers{from: {}, to: {}}, logic.Hooks{})
	srv.UseGroups(fixedGroups{allowed: false})
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "hi"}); !isCode(err, codes.PermissionDenied, errorcode.RelationGroupFriendForbidden) {
		t.Fatalf("group forbid code=%v err=%v", status.Code(err), err)
	}
	srv.UseGroups(failGroups{})
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "hi"}); !isCode(err, codes.Internal, errorcode.Internal) {
		t.Fatalf("group down code=%v err=%v", status.Code(err), err)
	}
	srv.UseGroups(nil)

	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: from}); !isCode(err, codes.InvalidArgument, errorcode.RelationCanNotAddYourself) {
		t.Fatalf("self add code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: "missing"}); !isCode(err, codes.NotFound, errorcode.RelationUserNotFound) {
		t.Fatalf("missing user code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: "bad:id", ToUserId: to}); !isCode(err, codes.InvalidArgument, errorcode.RelationInvalidUserID) {
		t.Fatalf("bad id code=%v err=%v", status.Code(err), err)
	}

	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "hi"}); err != nil {
		t.Fatal(err)
	}
	recv, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: to})
	if err != nil || recv.GetTotal() != 1 || len(recv.GetRequests()) != 1 {
		t.Fatalf("recv list: %+v err=%v", recv, err)
	}
	gotReq := recv.GetRequests()[0]
	if gotReq.GetFromUserId() != from || gotReq.GetToUserId() != to || gotReq.GetMessage() != "hi" || gotReq.GetHandleResult() != 0 || gotReq.GetFromNickname() != from || gotReq.GetCreateTime() == 0 {
		t.Fatalf("recv item: %+v", gotReq)
	}
	sent, err := srv.GetSentFriendRequests(ctx, &relationpb.GetSentFriendRequestsReq{UserId: from, HandleResults: []int32{0}})
	if err != nil || sent.GetTotal() != 1 || sent.GetRequests()[0].GetToNickname() != to {
		t.Fatalf("sent list: %+v err=%v", sent, err)
	}
	none, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: from, HandleResults: []int32{-1}})
	if err != nil || none.GetTotal() != 0 || len(none.GetRequests()) != 0 {
		t.Fatalf("filtered recv: %+v err=%v", none, err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "again"}); !isCode(err, codes.AlreadyExists, errorcode.RelationRequestRepeated) {
		t.Fatalf("duplicate pending code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: from, ToUserId: to, HandleResult: 2}); !isCode(err, codes.InvalidArgument, errorcode.RelationInvalidHandle) {
		t.Fatalf("bad handle code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: "nobody", ToUserId: to, HandleResult: -1}); !isCode(err, codes.NotFound, errorcode.RelationRequestNotFound) {
		t.Fatalf("missing request code=%v err=%v", status.Code(err), err)
	}

	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: from, ToUserId: to, HandleResult: -1, HandleMsg: "no"}); err != nil {
		t.Fatal(err)
	}
	in1, in2, err := st.CheckIn(ctx, from, to)
	if err != nil || in1 || in2 {
		t.Fatalf("refuse created friendship in1=%v in2=%v err=%v", in1, in2, err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: from, ToUserId: to, HandleResult: 1}); !isCode(err, codes.FailedPrecondition, errorcode.RelationRequestHandled) {
		t.Fatalf("second respond code=%v err=%v", status.Code(err), err)
	}

	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "please"}); err != nil {
		t.Fatalf("reapply after refuse: %v", err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: from, ToUserId: to, HandleResult: 1, HandleMsg: "ok"}); err != nil {
		t.Fatal(err)
	}
	in1, in2, err = st.CheckIn(ctx, from, to)
	if err != nil || !in1 || !in2 {
		t.Fatalf("agree friendship in1=%v in2=%v err=%v", in1, in2, err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to}); !isCode(err, codes.AlreadyExists, errorcode.RelationAlreadyFriends) {
		t.Fatalf("already friends code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: from, ToUserId: to, HandleResult: 1}); !isCode(err, codes.FailedPrecondition, errorcode.RelationRequestHandled) {
		t.Fatalf("agree twice code=%v err=%v", status.Code(err), err)
	}
}

func TestBlackAndFriend(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}

	a := fmt.Sprintf("a-%d", time.Now().UnixNano())
	b := fmt.Sprintf("b-%d", time.Now().UnixNano())
	t.Cleanup(func() { cleanupRelation(t, mysqlCfg, a, b) })

	var reject atomic.Bool
	reject.Store(true)
	removed := make(chan struct{}, 1)
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + "callbackBeforeAddBlackCommand":
			resp := webhook.CommonCallbackResp{}
			if reject.Load() {
				resp.ActionCode = webhook.ActionForbidden
				resp.ErrMsg = "no"
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/" + "callbackAfterRemoveBlackCommand":
			select {
			case removed <- struct{}{}:
			default:
			}
			_ = json.NewEncoder(w).Encode(webhook.CommonCallbackResp{})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpSrv.Close)
	hooks := webhook.NewWebhookClient(httpSrv.URL)
	t.Cleanup(hooks.Close)
	srv := logic.New(st, knownUsers{a: {}, b: {}}, logic.Hooks{
		Client:           hooks,
		BeforeAddBlack:   webhook.BeforeConfig{Enable: true, Timeout: 2},
		AfterRemoveBlack: webhook.AfterConfig{Enable: true, Timeout: 2},
	})

	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); !isCode(err, codes.PermissionDenied, errorcode.WebhookRejected) {
		t.Fatalf("webhook reject code=%v err=%v", status.Code(err), err)
	}
	reject.Store(false)
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: "missing"}); !isCode(err, codes.NotFound, errorcode.RelationUserNotFound) {
		t.Fatalf("missing user code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); !isCode(err, codes.AlreadyExists, errorcode.RelationAlreadyBlocked) {
		t.Fatalf("duplicate black code=%v err=%v", status.Code(err), err)
	}
	black, err := srv.IsBlack(ctx, &relationpb.IsBlackReq{User1: a, User2: b})
	if err != nil || !black.GetInUser1Blacklist() || black.GetInUser2Blacklist() {
		t.Fatalf("one-way black %+v err=%v", black, err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: b, BlockedUserId: a}); err != nil {
		t.Fatal(err)
	}
	black, err = srv.IsBlack(ctx, &relationpb.IsBlackReq{User1: a, User2: b})
	if err != nil || !black.GetInUser1Blacklist() || !black.GetInUser2Blacklist() {
		t.Fatalf("mutual black %+v err=%v", black, err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-removed:
	case <-time.After(2 * time.Second):
		t.Fatal("after remove black webhook did not arrive")
	}
	black, err = srv.IsBlack(ctx, &relationpb.IsBlackReq{User1: a, User2: b})
	if err != nil || black.GetInUser1Blacklist() || !black.GetInUser2Blacklist() {
		t.Fatalf("after unblock %+v err=%v", black, err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatalf("second unblock: %v", err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: b, BlockedUserId: a}); err != nil {
		t.Fatalf("unblock reverse: %v", err)
	}

	friend, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || friend.GetInUser1Friends() || friend.GetInUser2Friends() {
		t.Fatalf("unexpected friendship %+v err=%v", friend, err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1}); err != nil {
		t.Fatal(err)
	}
	friend, err = srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || !friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("agreed friendship %+v err=%v", friend, err)
	}
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b}); !isCode(err, codes.InvalidArgument, errorcode.RelationNothingToUpdate) {
		t.Fatalf("empty update code=%v err=%v", status.Code(err), err)
	}
	long := strings.Repeat("r", 256)
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b, Remark: &long}); !isCode(err, codes.InvalidArgument, errorcode.RelationInvalidRemark) {
		t.Fatalf("long remark code=%v err=%v", status.Code(err), err)
	}
	remark := "  Ada  "
	pinned := true
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b, Remark: &remark, IsPinned: &pinned}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("mysql", mysqlDSN(mysqlCfg))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var gotRemark string
	var gotPinned int
	if err := db.QueryRowContext(ctx, `SELECT remark, is_pinned FROM friend WHERE owner_user_id = ? AND friend_user_id = ?`, a, b).Scan(&gotRemark, &gotPinned); err != nil {
		t.Fatal(err)
	}
	if gotRemark != "Ada" || gotPinned != 1 {
		t.Fatalf("remark=%q pinned=%d", gotRemark, gotPinned)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	friend, err = srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("one-sided delete %+v err=%v", friend, err)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); !isCode(err, codes.NotFound, errorcode.RelationNotFriend) {
		t.Fatalf("second delete code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b, Remark: &remark}); !isCode(err, codes.NotFound, errorcode.RelationNotFriend) {
		t.Fatalf("update missing code=%v err=%v", status.Code(err), err)
	}
}

func TestBlockedFriendRequest(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	a := fmt.Sprintf("ba-%d", time.Now().UnixNano())
	b := fmt.Sprintf("bb-%d", time.Now().UnixNano())
	t.Cleanup(func() { cleanupRelation(t, mysqlCfg, a, b) })

	var agrees atomic.Int32
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/"+webhook.CommandBeforeAddFriendAgree {
			agrees.Add(1)
		}
		_ = json.NewEncoder(w).Encode(webhook.CommonCallbackResp{})
	}))
	t.Cleanup(httpSrv.Close)
	hooks := webhook.NewWebhookClient(httpSrv.URL)
	t.Cleanup(hooks.Close)
	srv := logic.New(st, knownUsers{a: {}, b: {}}, logic.Hooks{
		Client:               hooks,
		BeforeAddFriendAgree: webhook.BeforeConfig{Enable: true, Timeout: 2},
	})

	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: a}); !isCode(err, codes.InvalidArgument, errorcode.RelationCannotBlockYourself) {
		t.Fatalf("self block code=%v err=%v", status.Code(err), err)
	}
	self, err := srv.IsBlack(ctx, &relationpb.IsBlackReq{User1: a, User2: a})
	if err != nil || self.GetInUser1Blacklist() || self.GetInUser2Blacklist() {
		t.Fatalf("self black %+v err=%v", self, err)
	}

	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	friends, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || friends.GetInUser1Friends() || friends.GetInUser2Friends() {
		t.Fatalf("block created friendship %+v err=%v", friends, err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1}); !isCode(err, codes.PermissionDenied, errorcode.RelationBlocked) {
		t.Fatalf("agree while blocked code=%v err=%v", status.Code(err), err)
	}
	if agrees.Load() != 0 {
		t.Fatalf("agree webhook ran %d times", agrees.Load())
	}
	pending, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: b, HandleResults: []int32{0}})
	if err != nil || pending.GetTotal() != 1 {
		t.Fatalf("pending after blocked agree %+v err=%v", pending, err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: -1}); err != nil {
		t.Fatalf("refuse while blocked: %v", err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "again"}); !isCode(err, codes.PermissionDenied, errorcode.RelationBlocked) {
		t.Fatalf("send while blocker code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: b, ToUserId: a, Message: "back"}); !isCode(err, codes.PermissionDenied, errorcode.RelationBlocked) {
		t.Fatalf("send while blocked code=%v err=%v", status.Code(err), err)
	}
	refused, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: b, HandleResults: []int32{-1}})
	if err != nil || refused.GetTotal() != 1 {
		t.Fatalf("refused request overwritten %+v err=%v", refused, err)
	}

	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "please"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1}); err != nil {
		t.Fatal(err)
	}
	if agrees.Load() != 1 {
		t.Fatalf("agree webhook %d", agrees.Load())
	}
	friends, err = srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || !friends.GetInUser1Friends() || !friends.GetInUser2Friends() {
		t.Fatalf("friends after unblock %+v err=%v", friends, err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	friends, err = srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || !friends.GetInUser1Friends() || !friends.GetInUser2Friends() {
		t.Fatalf("block removed friends %+v err=%v", friends, err)
	}
}

func TestIncremental(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	a := fmt.Sprintf("ia-%d", time.Now().UnixNano())
	b := fmt.Sprintf("ib-%d", time.Now().UnixNano())
	t.Cleanup(func() { cleanupRelation(t, mysqlCfg, a, b) })
	srv := logic.New(st, knownUsers{a: {}, b: {}}, logic.Hooks{})

	full, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a})
	if err != nil || !full.GetFull() || full.GetVersion() != 0 || len(full.GetInsert()) != 0 {
		t.Fatalf("empty friends %+v err=%v", full, err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1}); err != nil {
		t.Fatal(err)
	}
	full, err = srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a})
	if err != nil || !full.GetFull() || full.GetVersion() == 0 || full.GetVersionId() == "" || len(full.GetInsert()) != 0 || full.GetSortVersion() != 0 {
		t.Fatalf("full friends %+v err=%v", full, err)
	}
	same, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a, VersionId: full.GetVersionId(), Version: full.GetVersion()})
	if err != nil || same.GetFull() || len(same.GetInsert()) != 0 || len(same.GetUpdate()) != 0 || len(same.GetDelete()) != 0 || same.GetVersion() != full.GetVersion() {
		t.Fatalf("unchanged friends %+v err=%v", same, err)
	}
	remark := "Ada"
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b, Remark: &remark}); err != nil {
		t.Fatal(err)
	}
	changed, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a, VersionId: full.GetVersionId(), Version: full.GetVersion()})
	if err != nil || changed.GetFull() || changed.GetSortVersion() != 0 || len(changed.GetUpdate()) != 1 || changed.GetUpdate()[0].GetRemark() != "Ada" || changed.GetVersion() <= full.GetVersion() {
		t.Fatalf("remark sync %+v err=%v", changed, err)
	}
	pinned := true
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b, IsPinned: &pinned}); err != nil {
		t.Fatal(err)
	}
	pinnedSync, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a, VersionId: changed.GetVersionId(), Version: changed.GetVersion()})
	if err != nil || pinnedSync.GetFull() || pinnedSync.GetSortVersion() == 0 || len(pinnedSync.GetUpdate()) != 1 || !pinnedSync.GetUpdate()[0].GetIsPinned() {
		t.Fatalf("pin sync %+v err=%v", pinnedSync, err)
	}
	reset, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a, VersionId: "other-version-id", Version: pinnedSync.GetVersion()})
	if err != nil || !reset.GetFull() || len(reset.GetInsert()) != 0 {
		t.Fatalf("version id mismatch %+v err=%v", reset, err)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	deleted, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a, VersionId: pinnedSync.GetVersionId(), Version: pinnedSync.GetVersion()})
	if err != nil || deleted.GetFull() || len(deleted.GetDelete()) != 1 || deleted.GetDelete()[0] != b {
		t.Fatalf("delete sync %+v err=%v", deleted, err)
	}

	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	listed, err := srv.GetBlackList(ctx, &relationpb.GetBlackListReq{UserId: a})
	if err != nil || listed.GetTotal() != 1 || len(listed.GetBlacks()) != 1 || listed.GetBlacks()[0].GetBlackUser().GetUserId() != b {
		t.Fatalf("black list %+v err=%v", listed, err)
	}
	in, err := srv.IsBlack(ctx, &relationpb.IsBlackReq{User1: a, User2: b})
	if err != nil || !in.GetInUser1Blacklist() || in.GetInUser2Blacklist() {
		t.Fatalf("is black %+v err=%v", in, err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	listed, err = srv.GetBlackList(ctx, &relationpb.GetBlackListReq{UserId: a})
	if err != nil || listed.GetTotal() != 0 || len(listed.GetBlacks()) != 0 {
		t.Fatalf("empty black list %+v err=%v", listed, err)
	}
}

func TestFriendPageAndIDs(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	a := fmt.Sprintf("pa-%d", time.Now().UnixNano())
	b := fmt.Sprintf("pb-%d", time.Now().UnixNano())
	c := fmt.Sprintf("pc-%d", time.Now().UnixNano())
	t.Cleanup(func() { cleanupRelation(t, mysqlCfg, a, b, c) })
	srv := logic.New(st, knownUsers{a: {}, b: {}, c: {}}, logic.Hooks{})

	empty, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: a})
	if err != nil || !empty.GetEqual() || empty.GetVersion() != 0 || len(empty.GetUserIds()) != 0 {
		t.Fatalf("empty ids %+v err=%v", empty, err)
	}
	if _, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: a, Limit: 201}); !isCode(err, codes.InvalidArgument, errorcode.RelationInvalidPage) {
		t.Fatalf("bad page code=%v err=%v", status.Code(err), err)
	}
	for _, friend := range []string{b, c} {
		if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: friend, Message: "hi"}); err != nil {
			t.Fatal(err)
		}
		if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: friend, HandleResult: 1}); err != nil {
			t.Fatal(err)
		}
	}
	pinned := true
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: c, IsPinned: &pinned}); err != nil {
		t.Fatal(err)
	}
	page, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: a, Limit: 1})
	if err != nil || page.GetTotal() != 2 || len(page.GetFriends()) != 1 || page.GetFriends()[0].GetFriendUser().GetUserId() != c {
		t.Fatalf("pinned page %+v err=%v", page, err)
	}
	next, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: a, Offset: 1, Limit: 1})
	if err != nil || len(next.GetFriends()) != 1 || next.GetFriends()[0].GetFriendUser().GetUserId() != b {
		t.Fatalf("second page %+v err=%v", next, err)
	}
	ids, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: a})
	if err != nil || ids.GetEqual() || len(ids.GetUserIds()) != 2 || ids.GetUserIds()[0] != c || ids.GetUserIds()[1] != b || ids.GetVersion() == 0 {
		t.Fatalf("friend ids %+v err=%v", ids, err)
	}
	same, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: a, IdHash: idhash.Hash(ids.GetUserIds())})
	if err != nil || !same.GetEqual() || len(same.GetUserIds()) != 0 || same.GetVersionId() != ids.GetVersionId() {
		t.Fatalf("equal ids %+v err=%v", same, err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: c}); err != nil {
		t.Fatal(err)
	}
	blacks, err := srv.GetBlackList(ctx, &relationpb.GetBlackListReq{UserId: a, Limit: 1})
	if err != nil || blacks.GetTotal() != 2 || len(blacks.GetBlacks()) != 1 || blacks.GetBlacks()[0].GetBlackUser().GetUserId() != b {
		t.Fatalf("blacks %+v err=%v", blacks, err)
	}
	blacks, err = srv.GetBlackList(ctx, &relationpb.GetBlackListReq{UserId: a, Offset: 1, Limit: 1})
	if err != nil || len(blacks.GetBlacks()) != 1 || blacks.GetBlacks()[0].GetBlackUser().GetUserId() != c {
		t.Fatalf("second blacks %+v err=%v", blacks, err)
	}
}

func TestFriendNotices(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	mysqlCfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn)
	if err != nil {
		t.Skipf("mysql is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	a := fmt.Sprintf("na-%d", time.Now().UnixNano())
	b := fmt.Sprintf("nb-%d", time.Now().UnixNano())
	t.Cleanup(func() { cleanupRelation(t, mysqlCfg, a, b) })
	notices := &noticeLog{}
	srv := logic.New(st, knownUsers{a: {}, b: {}}, logic.Hooks{})
	srv.UseSender(notices)

	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: -1, HandleMsg: "no"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "please"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1, HandleMsg: "ok"}); err != nil {
		t.Fatal(err)
	}
	remark := "Ada"
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b, Remark: &remark}); err != nil {
		t.Fatal(err)
	}
	pinned := true
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: a, FriendUserId: b, IsPinned: &pinned}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}

	want := []struct {
		contentType int32
		sendID      string
		recvID      string
		sendMessage bool
		offline     bool
	}{
		{1203, a, b, false, true},
		{1202, b, a, false, true},
		{1203, a, b, false, true},
		{1201, b, a, true, true},
		{1206, a, b, false, false},
		{1205, a, b, false, false},
		{1207, a, b, false, false},
		{1208, a, b, false, false},
	}
	if len(notices.msgs) != len(want) {
		t.Fatalf("notices=%d", len(notices.msgs))
	}
	for i, item := range want {
		msg := notices.msgs[i]
		if msg.GetContentType() != item.contentType || msg.GetSendId() != item.sendID || msg.GetRecvId() != item.recvID || msg.GetSessionType() != 1 || msg.GetMsgFrom() != 200 {
			t.Fatalf("notice %d %+v", i, msg)
		}
		opts := map[string]bool{
			"isNotNotification": false,
			"isSendMsg":         item.sendMessage,
			"unreadCount":       item.sendMessage,
			"history":           false,
			"offlinePush":       item.offline,
		}
		for key, value := range opts {
			if msg.Options[key] != value {
				t.Fatalf("notice %d option %s=%v msg=%+v", i, key, msg.Options[key], msg.Options)
			}
		}
		if item.offline {
			if msg.GetOfflinePushInfo().GetTitle() == "" {
				t.Fatalf("notice %d missing offline title", i)
			}
		} else if msg.OfflinePushInfo != nil {
			t.Fatalf("notice %d unexpected offline %+v", i, msg.OfflinePushInfo)
		}
	}
	apply := noticeDetail(t, notices.msgs[0])
	if apply.FromTo.FromUserID != a || apply.FromTo.ToUserID != b || apply.Request.ReqMsg != "hi" || apply.Request.FromNickname != a {
		t.Fatalf("apply detail %+v", apply)
	}
	agreed := noticeDetail(t, notices.msgs[3])
	if agreed.FromTo.FromUserID != a || agreed.FromTo.ToUserID != b || agreed.HandleMsg != "ok" || agreed.Request.HandleResult != 1 {
		t.Fatalf("agree detail %+v", agreed)
	}
	remarked := noticeDetail(t, notices.msgs[4])
	if remarked.FriendVersion == 0 || remarked.FriendVersionID == "" {
		t.Fatalf("remark detail %+v", remarked)
	}
}

type noticeLog struct {
	msgs []*msgpb.MsgData
}

func (n *noticeLog) SendNotification(_ context.Context, msg *msgpb.MsgData) error {
	n.msgs = append(n.msgs, msg)
	return nil
}

type noticeBody struct {
	FromTo struct {
		FromUserID string `json:"fromUserID"`
		ToUserID   string `json:"toUserID"`
	} `json:"fromToUserID"`
	HandleMsg       string `json:"handleMsg"`
	FriendVersion   uint64 `json:"friendVersion"`
	FriendVersionID string `json:"friendVersionID"`
	Request         struct {
		ReqMsg       string `json:"reqMsg"`
		FromNickname string `json:"fromNickname"`
		HandleResult int32  `json:"handleResult"`
	} `json:"request"`
}

func noticeDetail(t *testing.T, msg *msgpb.MsgData) noticeBody {
	t.Helper()
	var wrap struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(msg.GetContent(), &wrap); err != nil {
		t.Fatal(err)
	}
	var body noticeBody
	if err := json.Unmarshal([]byte(wrap.Detail), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

type fixedGroups struct{ allowed bool }

func (g fixedGroups) MemberFriendAllowed(context.Context, string, string) (bool, error) {
	return g.allowed, nil
}

type failGroups struct{}

func (failGroups) MemberFriendAllowed(context.Context, string, string) (bool, error) {
	return false, fmt.Errorf("group down")
}

func isCode(err error, grpc codes.Code, code errorcode.Code) bool {
	got, ok := errorcode.CodeOf(err)
	return ok && got == code && status.Code(err) == grpc
}

func cleanupRelation(t *testing.T, cfg config.MySQL, ids ...string) {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Errorf("cleanup connect: %v", err)
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range ids {
		if _, err := db.ExecContext(ctx, `DELETE FROM friend WHERE owner_user_id = ? OR friend_user_id = ?`, id, id); err != nil {
			t.Errorf("cleanup friend: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM friend_request WHERE from_user_id = ? OR to_user_id = ?`, id, id); err != nil {
			t.Errorf("cleanup request: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM black WHERE owner_user_id = ? OR block_user_id = ?`, id, id); err != nil {
			t.Errorf("cleanup black: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM friend_version_log WHERE owner_user_id = ?`, id); err != nil {
			t.Errorf("cleanup friend version log: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM friend_version WHERE owner_user_id = ?`, id); err != nil {
			t.Errorf("cleanup friend version: %v", err)
		}
	}
}

func readEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		out[k] = v
	}
	return out
}
