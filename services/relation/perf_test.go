//go:build perf

package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/lisuhang11/SuIM/pkg/config"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/logic"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

const (
	perfFriends  = 5000
	perfPins     = 20
	perfRequests = 2000
	perfRepeat   = 30
)

func TestPerfRelation(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	createBenchDB(t, env)
	cfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    benchDB,
		MaxOpenConn: 8,
		MaxIdleConn: 4,
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "relation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { clearBench(t, db) })

	whale := benchID(1)
	inbox := benchID(2)
	started := time.Now()
	seedPerf(t, ctx, db, whale, inbox)
	t.Logf("RESULT name=seed med_ms=%.3f", float64(time.Since(started).Microseconds())/1000)

	srv := logic.New(st, knownUsers{whale: {}, inbox: {}}, logic.Hooks{})
	assertPerfShape(t, ctx, srv, whale, inbox)

	report(t, "friend_list_first", perfRepeat, func() {
		resp, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: whale, Limit: 20})
		if err != nil || resp.GetTotal() != perfFriends || len(resp.GetFriends()) != 20 {
			t.Fatalf("first page total=%d len=%d err=%v", resp.GetTotal(), len(resp.GetFriends()), err)
		}
	})
	report(t, "friend_list_deep", perfRepeat, func() {
		resp, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: whale, Offset: perfFriends - 20, Limit: 20})
		if err != nil || len(resp.GetFriends()) != 20 {
			t.Fatalf("deep page len=%d err=%v", len(resp.GetFriends()), err)
		}
	})
	report(t, "friend_ids_full", perfRepeat, func() {
		resp, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: whale})
		if err != nil || len(resp.GetUserIds()) != perfFriends {
			t.Fatalf("ids %d err=%v", len(resp.GetUserIds()), err)
		}
	})
	report(t, "incremental_version_zero", perfRepeat, func() {
		resp, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: whale})
		if err != nil || !resp.GetFull() || len(resp.GetInsert()) != 0 {
			t.Fatalf("version 0 %+v err=%v", resp, err)
		}
	})
	head, err := st.FriendVersion(ctx, whale)
	if err != nil {
		t.Fatal(err)
	}
	report(t, "incremental_near", perfRepeat, func() {
		resp, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: whale, VersionId: head.ID, Version: head.Version - 50})
		if err != nil || resp.GetFull() || resp.GetSortVersion() == 0 {
			t.Fatalf("near %+v err=%v", resp, err)
		}
	})
	report(t, "incremental_far", perfRepeat, func() {
		resp, err := srv.GetIncrementalFriends(ctx, &relationpb.GetIncrementalFriendsReq{UserId: whale, VersionId: head.ID, Version: head.Version - 500})
		if err != nil || !resp.GetFull() {
			t.Fatalf("far %+v err=%v", resp, err)
		}
	})
	report(t, "version_log_all", perfRepeat, func() {
		logs, err := st.FriendChanges(ctx, whale, 0, perfFriends+10)
		if err != nil || len(logs) != perfFriends+1 {
			t.Fatalf("log len=%d err=%v", len(logs), err)
		}
	})
	report(t, "recv_requests_first", perfRepeat, func() {
		resp, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: inbox, Limit: 20, HandleResults: []int32{0}})
		if err != nil || resp.GetTotal() != perfRequests || len(resp.GetRequests()) != 20 {
			t.Fatalf("recv total=%d len=%d err=%v", resp.GetTotal(), len(resp.GetRequests()), err)
		}
	})

	t.Logf("EXPLAIN friend_ids %s", explainQuery(t, ctx, db, `SELECT friend_user_id FROM friend WHERE owner_user_id = ? ORDER BY is_pinned DESC, create_time ASC, friend_user_id ASC`, whale))
	t.Logf("EXPLAIN friend_page %s", explainQuery(t, ctx, db, `SELECT friend_user_id FROM friend WHERE owner_user_id = ? ORDER BY is_pinned DESC, create_time ASC, friend_user_id ASC LIMIT ? OFFSET ?`, whale, 20, perfFriends-20))
	t.Logf("EXPLAIN version_log %s", explainQuery(t, ctx, db, `SELECT elem_id, state, version FROM friend_version_log WHERE owner_user_id = ? AND version > ? ORDER BY version LIMIT ?`, whale, 0, 200))
	t.Logf("EXPLAIN recv_count %s", explainQuery(t, ctx, db, `SELECT COUNT(*) FROM friend_request WHERE to_user_id = ? AND handle_result IN (?)`, inbox, 0))
	t.Logf("EXPLAIN recv_page %s", explainQuery(t, ctx, db, `SELECT from_user_id FROM friend_request WHERE to_user_id = ? AND handle_result IN (?) ORDER BY create_time DESC, from_user_id ASC, to_user_id ASC LIMIT ? OFFSET ?`, inbox, 0, 20, 0))
}

func assertPerfShape(t *testing.T, ctx context.Context, srv *logic.Server, whale, inbox string) {
	t.Helper()
	page, err := srv.GetFriendList(ctx, &relationpb.GetFriendListReq{UserId: whale, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.GetTotal() != perfFriends || len(page.GetFriends()) != perfPins || !page.GetFriends()[0].GetIsPinned() || page.GetFriends()[0].GetFriendUser().GetUserId() != benchID(10001) {
		t.Fatalf("shape page total=%d first=%s pinned=%v", page.GetTotal(), page.GetFriends()[0].GetFriendUser().GetUserId(), page.GetFriends()[0].GetIsPinned())
	}
	ids, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: whale})
	if err != nil || len(ids.GetUserIds()) != perfFriends || ids.GetUserIds()[0] != benchID(10001) || ids.GetUserIds()[perfPins] != benchID(10001+perfPins) {
		t.Fatalf("shape ids %d err=%v", len(ids.GetUserIds()), err)
	}
	recv, err := srv.GetRecvFriendRequests(ctx, &relationpb.GetRecvFriendRequestsReq{UserId: inbox, Limit: 1, HandleResults: []int32{0}})
	if err != nil || recv.GetTotal() != perfRequests {
		t.Fatalf("shape requests %d err=%v", recv.GetTotal(), err)
	}
}

func seedPerf(t *testing.T, ctx context.Context, db *sql.DB, whale, inbox string) {
	t.Helper()
	clearBench(t, db)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	friends := make([]row, 0, perfFriends*2)
	logs := make([]row, 0, perfFriends+1)
	for i := 0; i < perfFriends; i++ {
		id := benchID(10001 + i)
		pinned := 0
		if i < perfPins {
			pinned = 1
		}
		created := base.Add(time.Duration(i) * time.Second)
		friends = append(friends, row{a: whale, b: id, at: created, n: pinned})
		friends = append(friends, row{a: id, b: whale, at: created, n: 0})
		logs = append(logs, row{a: whale, b: id, at: created, n: int(model.VersionInsert), v: uint64(i + 1)})
	}
	logs = append(logs, row{a: whale, b: model.SortElemID, at: base.Add(time.Duration(perfFriends) * time.Second), n: int(model.VersionUpdate), v: uint64(perfFriends + 1)})
	if err := insertRows(ctx, db, `INSERT INTO friend (owner_user_id, friend_user_id, remark, create_time, add_source, operator_user_id, is_pinned) VALUES %s`, "(?,?, '', ?, 2, ?, ?)", friends, func(r row) []any {
		return []any{r.a, r.b, r.at, r.a, r.n}
	}); err != nil {
		t.Fatal(err)
	}
	headAt := base.Add(time.Duration(perfFriends) * time.Second)
	if _, err := db.ExecContext(ctx, `INSERT INTO friend_version (id, owner_user_id, version, deleted, last_update) VALUES (?, ?, ?, 0, ?)`, "benchv000000000000000001", whale, perfFriends+1, headAt); err != nil {
		t.Fatal(err)
	}
	if err := insertRows(ctx, db, `INSERT INTO friend_version_log (owner_user_id, elem_id, state, version, last_update) VALUES %s`, "(?,?,?,?,?)", logs, func(r row) []any {
		return []any{r.a, r.b, r.n, r.v, r.at}
	}); err != nil {
		t.Fatal(err)
	}
	requests := make([]row, 0, perfRequests)
	for i := 0; i < perfRequests; i++ {
		requests = append(requests, row{a: benchID(20001 + i), b: inbox, at: base.Add(time.Duration(i) * time.Second)})
	}
	if err := insertRows(ctx, db, `INSERT INTO friend_request (from_user_id, to_user_id, handle_result, req_msg, create_time, handler_user_id, handle_msg, handle_time) VALUES %s`, "(?,?,0,'hi',?,'','',NULL)", requests, func(r row) []any {
		return []any{r.a, r.b, r.at}
	}); err != nil {
		t.Fatal(err)
	}
}

func report(t *testing.T, name string, n int, fn func()) {
	t.Helper()
	fn()
	samples := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		samples[i] = time.Since(start)
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	t.Logf("RESULT name=%s n=%d min_ms=%.3f med_ms=%.3f max_ms=%.3f", name, n, ms(sorted[0]), ms(sorted[len(sorted)/2]), ms(sorted[len(sorted)-1]))
}
