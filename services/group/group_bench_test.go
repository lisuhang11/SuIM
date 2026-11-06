//go:build perf || load

package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/config"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/logic"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

const (
	groupBenchDB       = "suim_group_bench"
	groupBenchMembers  = 5000
	groupBenchGroups   = 200
	groupBenchRequests = 2000
	groupBenchRepeat   = 30
)

func groupBenchGroup(n int) string { return fmt.Sprintf("g%07d", n) }
func groupBenchUser(n int) string  { return fmt.Sprintf("u%07d", n) }

type benchUsers struct{}

func (benchUsers) GetUsers(_ context.Context, ids []string) (map[string]model.UserProfile, error) {
	out := make(map[string]model.UserProfile, len(ids))
	for _, id := range ids {
		out[id] = model.UserProfile{UserID: id, Nickname: id}
	}
	return out, nil
}

func openGroupBench(t *testing.T) (context.Context, *sql.DB, *store.Store, redis.UniversalClient) {
	t.Helper()
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	createGroupBenchDB(t, env)
	cfg := config.MySQL{
		Address: []string{"127.0.0.1:3306"}, Username: env["MYSQL_USER"], Password: env["MYSQL_PASSWORD"],
		Database: groupBenchDB, MaxOpenConn: 40, MaxIdleConn: 20,
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "group.sql"))
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
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = db.Close() })
	truncateGroupBench(t, db)
	t.Cleanup(func() { truncateGroupBench(t, db) })
	rdb, err := store.NewRedis(config.Redis{Address: []string{"127.0.0.1:6379"}, Password: env["REDIS_PASSWORD"], RedisMode: "standalone"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}
	return ctx, db, st, rdb
}

func seedGroupBench(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	big := groupBenchGroup(1)
	owner := groupBenchUser(1)
	if err := insertGroups(ctx, db, base, big, owner); err != nil {
		t.Fatal(err)
	}
	if err := insertMembers(ctx, db, base, big, owner); err != nil {
		t.Fatal(err)
	}
	if err := insertVersions(ctx, db, base, big, owner); err != nil {
		t.Fatal(err)
	}
	if err := insertRequests(ctx, db, base, big); err != nil {
		t.Fatal(err)
	}
}

func insertGroups(ctx context.Context, db *sql.DB, base time.Time, big, owner string) error {
	rows := make([][]any, 0, groupBenchGroups+1)
	rows = append(rows, []any{big, "big", base, owner})
	for i := 2; i <= groupBenchGroups+1; i++ {
		rows = append(rows, []any{groupBenchGroup(i), "small", base, owner})
	}
	return batch(ctx, db, "INSERT INTO `group` (group_id, group_name, notification, introduction, face_url, create_time, status, creator_user_id, group_type, need_verification, look_member_info, apply_member_friend, notification_update_time, notification_user_id, ex) VALUES %s",
		"(?, ?, '', '', '', ?, 0, ?, 2, 0, 0, 0, NULL, '', '')", rows)
}

func insertMembers(ctx context.Context, db *sql.DB, base time.Time, big, owner string) error {
	rows := make([][]any, 0, groupBenchMembers+groupBenchGroups)
	for i := 1; i <= groupBenchMembers; i++ {
		role := model.RoleOrdinary
		if i == 1 {
			role = model.RoleOwner
		}
		rows = append(rows, []any{big, groupBenchUser(i), groupBenchUser(i), role, base.Add(time.Duration(i) * time.Millisecond), owner, owner})
	}
	for i := 2; i <= groupBenchGroups+1; i++ {
		rows = append(rows, []any{groupBenchGroup(i), owner, owner, model.RoleOwner, base, owner, owner})
	}
	return batch(ctx, db, "INSERT INTO group_member (group_id, user_id, nickname, face_url, role_level, join_time, join_source, inviter_user_id, operator_user_id, mute_end_time, ex) VALUES %s",
		"(?, ?, ?, '', ?, ?, 2, ?, ?, NULL, '')", rows)
}

func insertVersions(ctx context.Context, db *sql.DB, base time.Time, big, owner string) error {
	if _, err := db.ExecContext(ctx, "INSERT INTO group_member_version (version_id, d_id, version, deleted, last_update) VALUES (?, ?, ?, 0, ?)",
		"bbbbbbbbbbbbbbbbbbbbbbbb", big, groupBenchMembers, base); err != nil {
		return err
	}
	logs := make([][]any, 0, groupBenchMembers)
	for i := 1; i <= groupBenchMembers; i++ {
		logs = append(logs, []any{big, groupBenchUser(i), i, base})
	}
	if err := batch(ctx, db, "INSERT INTO group_member_version_log (d_id, e_id, state, version, last_update) VALUES %s", "(?, ?, 1, ?, ?)", logs); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO group_join_version (version_id, d_id, version, deleted, last_update) VALUES (?, ?, ?, 0, ?)",
		"cccccccccccccccccccccccc", owner, groupBenchGroups+1, base); err != nil {
		return err
	}
	joins := make([][]any, 0, groupBenchGroups+1)
	for i := 1; i <= groupBenchGroups+1; i++ {
		joins = append(joins, []any{owner, groupBenchGroup(i), i, base})
	}
	return batch(ctx, db, "INSERT INTO group_join_version_log (d_id, e_id, state, version, last_update) VALUES %s", "(?, ?, 1, ?, ?)", joins)
}

func insertRequests(ctx context.Context, db *sql.DB, base time.Time, big string) error {
	rows := make([][]any, 0, groupBenchRequests)
	for i := 1; i <= groupBenchRequests; i++ {
		rows = append(rows, []any{big, groupBenchUser(300000 + i), base.Add(time.Duration(i) * time.Millisecond)})
	}
	return batch(ctx, db, "INSERT INTO group_request (group_id, user_id, handle_result, req_msg, handled_msg, req_time, handle_user_id, handled_time, join_source, inviter_user_id, ex) VALUES %s",
		"(?, ?, 0, '', '', ?, '', NULL, 3, '', '')", rows)
}

func batch(ctx context.Context, db *sql.DB, query, tuple string, rows [][]any) error {
	const size = 500
	for start := 0; start < len(rows); start += size {
		end := start + size
		if end > len(rows) {
			end = len(rows)
		}
		part := rows[start:end]
		var b strings.Builder
		args := make([]any, 0, len(part)*6)
		for i, row := range part {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(tuple)
			args = append(args, row...)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(query, b.String()), args...); err != nil {
			return err
		}
	}
	return nil
}

func explainGroup(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, "EXPLAIN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("explain returned no row")
	}
	dest := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for i, col := range cols {
		got[col] = explainCell(dest[i])
	}
	return fmt.Sprintf("type=%s key=%s rows=%s extra=%s", got["type"], got["key"], got["rows"], got["Extra"])
}

func explainCell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

func timeCalls(t *testing.T, name string, n int, fn func()) {
	t.Helper()
	samples := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		samples[i] = time.Since(start)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	t.Logf("RESULT name=%s n=%d min_ms=%.3f p50_ms=%.3f max_ms=%.3f",
		name, n, ms(samples[0]), ms(percentile(samples, 0.50)), ms(samples[len(samples)-1]))
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func truncateGroupBench(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, table := range []string{
		"group_member_version_log", "group_member_version",
		"group_join_version_log", "group_join_version",
		"group_request", "group_member", "`group`",
	} {
		if _, err := db.ExecContext(ctx, "TRUNCATE TABLE "+table); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}

func createGroupBenchDB(t *testing.T, env map[string]string) {
	t.Helper()
	create := `CREATE DATABASE IF NOT EXISTS suim_group_bench CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`
	if err := execServer(env["MYSQL_USER"], env["MYSQL_PASSWORD"], create); err == nil {
		return
	}
	rootPass := env["MYSQL_ROOT_PASSWORD"]
	if rootPass == "" {
		t.Fatal("cannot create suim_group_bench")
	}
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/?parseTime=true&charset=utf8mb4&loc=UTC&timeout=5s", "root", rootPass))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, create); err != nil {
		t.Fatal(err)
	}
	user := env["MYSQL_USER"]
	if strings.Trim(user, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") != "" {
		t.Fatal("mysql user name is not usable for grant")
	}
	hosts, err := db.QueryContext(ctx, `SELECT host FROM mysql.user WHERE user = ?`, user)
	if err != nil {
		t.Fatal(err)
	}
	defer hosts.Close()
	for hosts.Next() {
		var host string
		if err := hosts.Scan(&host); err != nil {
			t.Fatal(err)
		}
		grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON suim_group_bench.* TO '%s'@'%s'", user, host)
		if _, err := db.ExecContext(ctx, grant); err != nil {
			t.Fatal(err)
		}
	}
	if err := hosts.Err(); err != nil {
		t.Fatal(err)
	}
}

func execServer(user, password, query string) error {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/?parseTime=true&charset=utf8mb4&loc=UTC&timeout=5s", user, password))
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, query)
	return err
}

func mustGroups(t *testing.T, srv *logic.Server, ctx context.Context, id string, n int) {
	t.Helper()
	resp, err := srv.GetGroupsInfo(ctx, &grouppb.GetGroupsInfoReq{GroupIds: []string{id}})
	if err != nil || len(resp.GetGroups()) != 1 || resp.GetGroups()[0].GetMemberCount() != int32(n) {
		t.Fatalf("groups %+v err=%v", resp, err)
	}
}
