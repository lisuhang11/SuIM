//go:build perf || load

package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

const benchDB = "suim_bench"

type row struct {
	a, b string
	at   time.Time
	n    int
	v    uint64
}

func insertRows(ctx context.Context, db *sql.DB, query, tuple string, rows []row, argsOf func(row) []any) error {
	const batch = 500
	for start := 0; start < len(rows); start += batch {
		end := start + batch
		if end > len(rows) {
			end = len(rows)
		}
		part := rows[start:end]
		var b strings.Builder
		args := make([]any, 0, len(part)*6)
		for i, item := range part {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(tuple)
			args = append(args, argsOf(item)...)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(query, b.String()), args...); err != nil {
			return err
		}
	}
	return nil
}

func explainQuery(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) string {
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

func createBenchDB(t *testing.T, env map[string]string) {
	t.Helper()
	create := `CREATE DATABASE IF NOT EXISTS suim_bench CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`
	if err := execServer(env["MYSQL_USER"], env["MYSQL_PASSWORD"], create); err == nil {
		return
	}
	rootPass := env["MYSQL_ROOT_PASSWORD"]
	if rootPass == "" {
		t.Fatal("cannot create suim_bench")
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
		grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON suim_bench.* TO '%s'@'%s'", user, host)
		if _, err := db.ExecContext(ctx, grant); err != nil {
			t.Fatal(err)
		}
	}
	if err := hosts.Err(); err != nil {
		t.Fatal(err)
	}
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

func clearBench(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, query := range []string{
		`DELETE FROM friend WHERE owner_user_id LIKE 'b%' OR friend_user_id LIKE 'b%'`,
		`DELETE FROM friend_request WHERE from_user_id LIKE 'b%' OR to_user_id LIKE 'b%'`,
		`DELETE FROM black WHERE owner_user_id LIKE 'b%' OR block_user_id LIKE 'b%'`,
		`DELETE FROM friend_version_log WHERE owner_user_id LIKE 'b%'`,
		`DELETE FROM friend_version WHERE owner_user_id LIKE 'b%'`,
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Errorf("clear bench: %v", err)
		}
	}
}

func benchID(n int) string {
	return fmt.Sprintf("b%08d", n)
}
