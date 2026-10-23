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
	"github.com/lisuhang11/SuIM/proto/relationpb"
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
			removed <- struct{}{}
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
	if err != nil || !full.GetFull() || full.GetVersion() == 0 || full.GetVersionId() == "" || len(full.GetInsert()) != 1 {
		t.Fatalf("full friends %+v err=%v", full, err)
	}
	if full.GetInsert()[0].GetFriendUser().GetUserId() != b || full.GetInsert()[0].GetFriendUser().GetNickname() != b {
		t.Fatalf("friend info %+v", full.GetInsert()[0])
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
	if err != nil || changed.GetFull() || len(changed.GetUpdate()) != 1 || changed.GetUpdate()[0].GetRemark() != "Ada" || changed.GetVersion() <= full.GetVersion() {
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
	if err != nil || !reset.GetFull() || len(reset.GetInsert()) != 1 {
		t.Fatalf("version id mismatch %+v err=%v", reset, err)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	deleted, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: a, VersionId: pinnedSync.GetVersionId(), Version: pinnedSync.GetVersion()})
	if err != nil || deleted.GetFull() || len(deleted.GetDelete()) != 1 || deleted.GetDelete()[0] != b {
		t.Fatalf("delete sync %+v err=%v", deleted, err)
	}

	blocked, err := srv.GetIncrementalBlacks(ctx, &relationpb.GetIncrementalBlacksReq{UserId: a})
	if err != nil || !blocked.GetFull() || len(blocked.GetInsert()) != 0 {
		t.Fatalf("empty blacks %+v err=%v", blocked, err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	blocked, err = srv.GetIncrementalBlacks(ctx, &relationpb.GetIncrementalBlacksReq{UserId: a})
	if err != nil || !blocked.GetFull() || len(blocked.GetInsert()) != 1 || blocked.GetInsert()[0].GetBlackUser().GetUserId() != b {
		t.Fatalf("full blacks %+v err=%v", blocked, err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	unblocked, err := srv.GetIncrementalBlacks(ctx, &relationpb.GetIncrementalBlacksReq{UserId: a, VersionId: blocked.GetVersionId(), Version: blocked.GetVersion()})
	if err != nil || unblocked.GetFull() || len(unblocked.GetDelete()) != 1 || unblocked.GetDelete()[0] != b {
		t.Fatalf("unblock sync %+v err=%v", unblocked, err)
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
		if _, err := db.ExecContext(ctx, `DELETE FROM black_version_log WHERE owner_user_id = ?`, id); err != nil {
			t.Errorf("cleanup black version log: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM black_version WHERE owner_user_id = ?`, id); err != nil {
			t.Errorf("cleanup black version: %v", err)
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
