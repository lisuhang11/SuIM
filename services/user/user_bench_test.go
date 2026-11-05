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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/localcache"
	"github.com/lisuhang11/SuIM/pkg/localcache/lru"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/logic"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

const (
	userBenchDB    = "suim_user_bench"
	userBenchN     = 100000
	userBenchHot   = 1
	userBenchPass  = "secret1"
	benchJWTSecret = "bench-secret"
)

func userBenchID(n int) string {
	return fmt.Sprintf("u%07d", n)
}

func openUserBench(t *testing.T) (context.Context, *sql.DB, *store.Store, redis.UniversalClient, *token.Issuer) {
	t.Helper()
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	createUserBenchDB(t, env)
	cfg := config.MySQL{
		Address: []string{"127.0.0.1:3306"}, Username: env["MYSQL_USER"], Password: env["MYSQL_PASSWORD"],
		Database: userBenchDB, MaxOpenConn: 32, MaxIdleConn: 16,
	}
	rdb, err := store.NewRedis(config.Redis{
		Address: []string{"127.0.0.1:6379"}, Password: env["REDIS_PASSWORD"], RedisMode: "standalone", PoolSize: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn, rdb)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "user.sql"))
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
	if _, err := db.ExecContext(ctx, `TRUNCATE TABLE auth_tokens`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `TRUNCATE TABLE user`); err != nil {
		t.Fatal(err)
	}
	issuer := token.NewIssuer(config.JWT{Secret: benchJWTSecret, ExpireSeconds: 3600})
	return ctx, db, st, rdb, issuer
}

func seedUserBench(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(userBenchPass), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	const batch = 500
	started := time.Now()
	for start := 1; start <= userBenchN; start += batch {
		end := start + batch - 1
		if end > userBenchN {
			end = userBenchN
		}
		var b strings.Builder
		args := make([]any, 0, (end-start+1)*5)
		b.WriteString(`INSERT INTO user (user_id, email, password_hash, nickname, is_active, account, has_password) VALUES `)
		for n := start; n <= end; n++ {
			if n > start {
				b.WriteByte(',')
			}
			b.WriteString("(?,?,?,?,1,?,1)")
			id := userBenchID(n)
			email := id + "@bench.test"
			args = append(args, id, email, string(hash), "n"+id, email)
		}
		if _, err := db.ExecContext(ctx, b.String(), args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("RESULT name=seed n=%d ms=%.3f", userBenchN, float64(time.Since(started).Microseconds())/1000)
	return string(hash)
}

func insertBenchToken(t *testing.T, ctx context.Context, db *sql.DB, issuer *token.Issuer, userID string) string {
	t.Helper()
	raw, exp, err := issuer.Sign("bench-token-00000000000000000001", userID, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	hash := token.Hash(raw)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO auth_tokens (id, user_id, platform_id, token, token_hash, token_type, expires_at, is_revoked)
		VALUES (?, ?, 1, ?, ?, ?, ?, 0)`,
		"bench-token-00000000000000000001", userID, raw, hash, model.TokenType, exp.UTC()); err != nil {
		t.Fatal(err)
	}
	return raw
}

func deleteBenchTokenKeys(t *testing.T, ctx context.Context, rdb redis.UniversalClient, raws ...string) {
	t.Helper()
	if len(raws) == 0 {
		return
	}
	keys := make([]string, 0, len(raws)*2)
	for _, raw := range raws {
		if raw == "" {
			continue
		}
		hash := token.Hash(raw)
		keys = append(keys, "suim:token:"+hash, "suim:token:deny:"+hash)
	}
	if len(keys) == 0 {
		return
	}
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		t.Errorf("delete token keys: %v", err)
	}
}

func explainUser(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) string {
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
		switch x := dest[i].(type) {
		case nil:
			got[col] = ""
		case []byte:
			got[col] = string(x)
		default:
			got[col] = fmt.Sprint(x)
		}
	}
	return fmt.Sprintf("type=%s key=%s rows=%s extra=%s", got["type"], got["key"], got["rows"], got["Extra"])
}

type tokenHits struct {
	hit, miss, failed atomic.Int64
}

func (h *tokenHits) IncrGetHit()      { h.hit.Add(1) }
func (h *tokenHits) IncrGetSuccess()  { h.miss.Add(1) }
func (h *tokenHits) IncrGetFailed()   { h.failed.Add(1) }
func (h *tokenHits) IncrDelHit()      {}
func (h *tokenHits) IncrDelNotFound() {}

func (h *tokenHits) String() string {
	hit, miss, failed := h.hit.Load(), h.miss.Load(), h.failed.Load()
	total := hit + miss + failed
	rate := 0.0
	if total > 0 {
		rate = float64(hit) / float64(total)
	}
	return fmt.Sprintf("local_hit=%d local_miss=%d failed=%d local_rate=%.3f", hit, miss, failed, rate)
}

func newBenchCache(target lru.Target) localcache.Cache[*model.Token] {
	return localcache.New[*model.Token](
		localcache.WithLocalSlotNum(100),
		localcache.WithLocalSlotSize(2000),
		localcache.WithLocalSuccessTTL(5*time.Minute),
		localcache.WithLocalFailedTTL(5*time.Second),
		localcache.WithLinkDisable(),
		localcache.WithTarget(target),
	)
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

type callLatency struct {
	mu         sync.Mutex
	samples    []time.Duration
	errs       int
	contention int
	first      string
}

func (l *callLatency) add(d time.Duration, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil && err.Error() == errorcode.ErrInternal.WithMsg("create session").Error() {
		l.contention++
		return
	}
	if err != nil {
		l.errs++
		if l.first == "" {
			l.first = err.Error()
		}
		return
	}
	l.samples = append(l.samples, d)
}

func (l *callLatency) report(t *testing.T, name string) {
	t.Helper()
	l.mu.Lock()
	samples := append([]time.Duration(nil), l.samples...)
	errs, contention, first := l.errs, l.contention, l.first
	l.mu.Unlock()
	if errs > 0 {
		t.Fatalf("%s errors=%d first=%s", name, errs, first)
	}
	if contention > 0 {
		t.Logf("RESULT name=%s_contention n=%d", name, contention)
	}
	if len(samples) == 0 {
		t.Fatalf("%s recorded no samples", name)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	t.Logf("RESULT name=%s n=%d p50_ms=%.3f p95_ms=%.3f p99_ms=%.3f max_ms=%.3f",
		name, len(samples), ms(percentile(samples, 0.50)), ms(percentile(samples, 0.95)), ms(percentile(samples, 0.99)), ms(samples[len(samples)-1]))
}

func mustValidate(t *testing.T, ctx context.Context, srv *logic.Server, raw, userID string) {
	t.Helper()
	resp, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: raw})
	if err != nil || !resp.GetValid() || resp.GetUser().GetUserId() != userID {
		t.Fatalf("validate %+v err=%v", resp, err)
	}
}

func createUserBenchDB(t *testing.T, env map[string]string) {
	t.Helper()
	create := `CREATE DATABASE IF NOT EXISTS suim_user_bench CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`
	if err := execServer(env["MYSQL_USER"], env["MYSQL_PASSWORD"], create); err == nil {
		return
	}
	rootPass := env["MYSQL_ROOT_PASSWORD"]
	if rootPass == "" {
		t.Fatal("cannot create suim_user_bench")
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
		grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON suim_user_bench.* TO '%s'@'%s'", user, host)
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
