package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/relationcache"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/logic"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

func TestOneSidedDeleteThenReapply(t *testing.T) {
	a, b := prefixed("od-a"), prefixed("od-b")
	ctx, srv, cfg := openRelation(t, a, b)

	becomeFriends(t, ctx, srv, a, b)
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	friend, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("after delete %+v err=%v", friend, err)
	}
	gone, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: a})
	if err != nil || gone.GetTotal() != 0 || len(gone.GetFriends()) != 0 {
		t.Fatalf("owner list %+v err=%v", gone, err)
	}
	kept, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: b})
	if err != nil || kept.GetTotal() != 1 || kept.GetFriends()[0].GetFriendUser().GetUserId() != a {
		t.Fatalf("other list %+v err=%v", kept, err)
	}

	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "please"}); err != nil {
		t.Fatalf("reapply from deleted side: %v", err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "again"}); !isCode(err, codes.AlreadyExists, errorcode.RelationRequestRepeated) {
		t.Fatalf("pending reapply code=%v err=%v", status.Code(err), err)
	}
	recv, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: b, HandleResults: []int32{0}})
	if err != nil || recv.GetTotal() != 1 || recv.GetRequests()[0].GetMessage() != "please" {
		t.Fatalf("pending %+v err=%v", recv, err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1, HandleMsg: "ok"}); err != nil {
		t.Fatal(err)
	}
	friend, err = srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || !friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("restored %+v err=%v", friend, err)
	}
	if state := versionState(t, cfg, a, b); state != model.VersionInsert {
		t.Fatalf("deleted side version state %d", state)
	}
	if state := versionState(t, cfg, b, a); state != model.VersionInsert {
		t.Fatalf("kept side version state %d", state)
	}
}

func TestRequestReopenOverwrites(t *testing.T) {
	a, b := prefixed("rw-a"), prefixed("rw-b")
	ctx, srv, cfg := openRelation(t, a, b)

	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: -1, HandleMsg: "no"}); err != nil {
		t.Fatal(err)
	}
	row := requestRow(t, cfg, a, b)
	if row.result != model.HandleRefuse || row.msg != "first" || row.handle != "no" || !row.handled {
		t.Fatalf("refused row %+v", row)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "second"}); err != nil {
		t.Fatal(err)
	}
	if n := requestCount(t, cfg, a, b); n != 1 {
		t.Fatalf("request rows %d", n)
	}
	row = requestRow(t, cfg, a, b)
	if row.result != model.HandlePending || row.msg != "second" || row.handle != "" || row.handler != "" || row.handled {
		t.Fatalf("reopened row %+v", row)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "third"}); !isCode(err, codes.AlreadyExists, errorcode.RelationRequestRepeated) {
		t.Fatalf("pending duplicate code=%v err=%v", status.Code(err), err)
	}
	if row = requestRow(t, cfg, a, b); row.msg != "second" {
		t.Fatalf("pending message changed to %q", row.msg)
	}

	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1, HandleMsg: "yes"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: b, FriendId: a}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "after"}); err != nil {
		t.Fatalf("reapply after both deleted: %v", err)
	}
	row = requestRow(t, cfg, a, b)
	if row.result != model.HandlePending || row.msg != "after" || row.handle != "" || row.handled {
		t.Fatalf("agreed row reopened %+v", row)
	}
	if n := requestCount(t, cfg, a, b); n != 1 {
		t.Fatalf("request rows after agree %d", n)
	}
}

func TestFriendPageBoundary(t *testing.T) {
	owner := prefixed("pg-owner")
	ids := make([]string, 0, 22)
	ids = append(ids, owner)
	friends := make([]string, 21)
	for i := range friends {
		friends[i] = prefixed(fmt.Sprintf("pg-%02d", i))
		ids = append(ids, friends[i])
	}
	ctx, srv, _ := openRelation(t, ids...)
	for _, friend := range friends[:20] {
		becomeFriends(t, ctx, srv, owner, friend)
	}
	full, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: owner})
	if err != nil || full.GetTotal() != 20 || len(full.GetFriends()) != 20 {
		t.Fatalf("full page %+v err=%v", full.GetTotal(), err)
	}
	rest, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: owner, Offset: 20})
	if err != nil || rest.GetTotal() != 20 || len(rest.GetFriends()) != 0 {
		t.Fatalf("past end %+v err=%v", rest, err)
	}

	last := friends[20]
	becomeFriends(t, ctx, srv, owner, last)
	page, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: owner, Limit: 20})
	if err != nil || page.GetTotal() != 21 || len(page.GetFriends()) != 20 {
		t.Fatalf("first of 21 total=%d len=%d err=%v", page.GetTotal(), len(page.GetFriends()), err)
	}
	next, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: owner, Offset: 20, Limit: 20})
	if err != nil || next.GetTotal() != 21 || len(next.GetFriends()) != 1 {
		t.Fatalf("second of 21 %+v err=%v", next, err)
	}
	seen := map[string]int{}
	for _, row := range append(page.GetFriends(), next.GetFriends()...) {
		seen[row.GetFriendUser().GetUserId()]++
	}
	if len(seen) != 21 {
		t.Fatalf("paged ids %d", len(seen))
	}
	for _, id := range friends {
		if seen[id] != 1 {
			t.Fatalf("id %s count %d", id, seen[id])
		}
	}

	pinned := true
	if _, err := srv.UpdateFriend(ctx, &relationpb.UpdateFriendReq{OwnerUserId: owner, FriendUserId: last, IsPinned: &pinned}); err != nil {
		t.Fatal(err)
	}
	page, err = srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: owner, Limit: 20})
	if err != nil || len(page.GetFriends()) != 20 || page.GetFriends()[0].GetFriendUser().GetUserId() != last || !page.GetFriends()[0].GetIsPinned() {
		t.Fatalf("pinned first %+v err=%v", page.GetFriends(), err)
	}
	next, err = srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: owner, Offset: 20, Limit: 20})
	if err != nil || len(next.GetFriends()) != 1 || next.GetFriends()[0].GetFriendUser().GetUserId() == last {
		t.Fatalf("pinned left page 2 %+v err=%v", next, err)
	}
}

func TestBlockKeepsFriendAndHoldsAgree(t *testing.T) {
	a, b := prefixed("bk-a"), prefixed("bk-b")
	ctx, srv, _ := openRelation(t, a, b)
	becomeFriends(t, ctx, srv, a, b)
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: a, ToUserId: b, Message: "back"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	friend, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("block removed the remaining row %+v err=%v", friend, err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1, HandleMsg: "ok"}); !isCode(err, codes.PermissionDenied, errorcode.RelationBlocked) {
		t.Fatalf("agree while blocked code=%v err=%v", status.Code(err), err)
	}
	pending, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: b, HandleResults: []int32{0}})
	if err != nil || pending.GetTotal() != 1 || pending.GetRequests()[0].GetMessage() != "back" {
		t.Fatalf("still pending %+v err=%v", pending, err)
	}
	kept, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: b})
	if err != nil || kept.GetTotal() != 1 {
		t.Fatalf("kept friend %+v err=%v", kept, err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: a, ToUserId: b, HandleResult: 1, HandleMsg: "ok"}); err != nil {
		t.Fatal(err)
	}
	friend, err = srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || !friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("agree after unblock %+v err=%v", friend, err)
	}
}

func TestFriendCacheFollowsDelete(t *testing.T) {
	a, b := prefixed("cf-a"), prefixed("cf-b")
	ctx, srv, _ := openRelation(t, a, b)
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: env["REDIS_PASSWORD"]})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis is not reachable: %v", err)
	}
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), relationcache.FriendIDsKey(a), relationcache.FriendIDsKey(b), relationcache.BlackIDsKey(a)).Err()
	})
	lists, stop := relationcache.NewLists(ctx, rdb, relationcache.DefaultConfig())
	t.Cleanup(stop)
	srv.UseCache(lists)

	becomeFriends(t, ctx, srv, a, b)
	friend, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || !friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("friends %+v err=%v", friend, err)
	}
	if _, err := rdb.Get(ctx, relationcache.FriendIDsKey(a)).Result(); err != nil {
		t.Fatalf("friend cache %v", err)
	}
	if _, err := srv.DeleteFriend(ctx, &relationpb.DeleteFriendReq{UserId: a, FriendId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := rdb.Get(ctx, relationcache.FriendIDsKey(a)).Result(); err != redis.Nil {
		t.Fatalf("deleted side still cached: %v", err)
	}
	friend, err = srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: a, User2: b})
	if err != nil || friend.GetInUser1Friends() || !friend.GetInUser2Friends() {
		t.Fatalf("after delete %+v err=%v", friend, err)
	}

	if _, err := srv.BlockUser(ctx, &relationpb.BlockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	black, err := srv.IsBlack(ctx, &relationpb.IsBlackReq{User1: a, User2: b})
	if err != nil || !black.GetInUser1Blacklist() || black.GetInUser2Blacklist() {
		t.Fatalf("blocked %+v err=%v", black, err)
	}
	if _, err := rdb.Get(ctx, relationcache.BlackIDsKey(a)).Result(); err != nil {
		t.Fatalf("black cache %v", err)
	}
	if _, err := srv.UnblockUser(ctx, &relationpb.UnblockUserReq{UserId: a, BlockedUserId: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := rdb.Get(ctx, relationcache.BlackIDsKey(a)).Result(); err != redis.Nil {
		t.Fatalf("unblocked list still cached: %v", err)
	}
}

func TestConcurrentFriendRequest(t *testing.T) {
	const pairs = 8
	const racers = 16
	stamp := time.Now().UnixNano()
	type pair struct{ from, to string }
	disjoint := make([]pair, pairs)
	ids := make([]string, 0, pairs*2+2)
	for i := range disjoint {
		disjoint[i] = pair{
			from: fmt.Sprintf("dl-f-%d-%d", stamp, i),
			to:   fmt.Sprintf("dl-t-%d-%d", stamp, i),
		}
		ids = append(ids, disjoint[i].from, disjoint[i].to)
	}
	sameFrom := fmt.Sprintf("dl-sf-%d", stamp)
	sameTo := fmt.Sprintf("dl-st-%d", stamp)
	ids = append(ids, sameFrom, sameTo)
	ctx, srv, cfg := openRelation(t, ids...)

	var wg sync.WaitGroup
	errs := make([]error, pairs)
	wg.Add(pairs)
	for i := range disjoint {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{
				FromUserId: disjoint[i].from, ToUserId: disjoint[i].to, Message: "disjoint",
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("disjoint %d: %v", i, err)
		}
		if n := requestCount(t, cfg, disjoint[i].from, disjoint[i].to); n != 1 {
			t.Fatalf("disjoint rows %d", n)
		}
	}

	var mu sync.Mutex
	var okN, repeated int
	var other error
	wg.Add(racers)
	for i := 0; i < racers; i++ {
		go func() {
			defer wg.Done()
			_, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{
				FromUserId: sameFrom, ToUserId: sameTo, Message: "race",
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				okN++
				return
			}
			if isCode(err, codes.AlreadyExists, errorcode.RelationRequestRepeated) {
				repeated++
				return
			}
			if other == nil {
				other = err
			}
		}()
	}
	wg.Wait()
	if other != nil {
		t.Fatalf("same pair: %v", other)
	}
	if okN != 1 || repeated != racers-1 || requestCount(t, cfg, sameFrom, sameTo) != 1 {
		t.Fatalf("same pair ok=%d repeated=%d rows=%d", okN, repeated, requestCount(t, cfg, sameFrom, sameTo))
	}
}

func prefixed(name string) string {
	return fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
}

func openRelation(t *testing.T, ids ...string) (context.Context, *logic.Server, config.MySQL) {
	t.Helper()
	ctx := context.Background()
	env := readEnvFile(t, "../../.env")
	cfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    env["MYSQL_DATABASE"],
		MaxOpenConn: 32,
		MaxIdleConn: 8,
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn)
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
	t.Cleanup(func() { cleanupRelation(t, cfg, ids...) })
	users := knownUsers{}
	for _, id := range ids {
		users[id] = struct{}{}
	}
	return ctx, logic.New(st, users, logic.Hooks{}), cfg
}

func becomeFriends(t *testing.T, ctx context.Context, srv *logic.Server, from, to string) {
	t.Helper()
	if _, err := srv.SendFriendRequest(ctx, &relationpb.SendFriendRequestReq{FromUserId: from, ToUserId: to, Message: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.RespondFriendApply(ctx, &relationpb.RespondFriendApplyReq{FromUserId: from, ToUserId: to, HandleResult: 1}); err != nil {
		t.Fatal(err)
	}
}

type requestRecord struct {
	result  int32
	msg     string
	handle  string
	handler string
	handled bool
}

func requestRow(t *testing.T, cfg config.MySQL, from, to string) requestRecord {
	t.Helper()
	db := openDB(t, cfg)
	defer db.Close()
	var row requestRecord
	var handled sql.NullTime
	err := db.QueryRowContext(context.Background(), `
		SELECT handle_result, req_msg, handle_msg, handler_user_id, handle_time
		FROM friend_request WHERE from_user_id = ? AND to_user_id = ?`, from, to).
		Scan(&row.result, &row.msg, &row.handle, &row.handler, &handled)
	if err != nil {
		t.Fatal(err)
	}
	row.handled = handled.Valid
	return row
}

func requestCount(t *testing.T, cfg config.MySQL, from, to string) int {
	t.Helper()
	db := openDB(t, cfg)
	defer db.Close()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM friend_request WHERE from_user_id = ? AND to_user_id = ?`, from, to).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func versionState(t *testing.T, cfg config.MySQL, owner, elem string) int32 {
	t.Helper()
	db := openDB(t, cfg)
	defer db.Close()
	var state int32
	if err := db.QueryRowContext(context.Background(), `SELECT state FROM friend_version_log WHERE owner_user_id = ? AND elem_id = ?`, owner, elem).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func openDB(t *testing.T, cfg config.MySQL) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Fatal(err)
	}
	return db
}
