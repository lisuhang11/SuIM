//go:build perf || load

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/relationcache"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/logic"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

const (
	cacheFriends = 5000
	cachePeers   = 200
	cacheRepeat  = 30
	cacheReaders = 16
	cacheLoadFor = 8 * time.Second
)

func TestPerfRelationCache(t *testing.T) {
	ctx, srv, lists, st, friend := openCacheBench(t)
	plain := logic.New(st, knownUsers{cacheWhale(): {}}, logic.Hooks{})

	report(t, "mysql_full_ids", cacheRepeat, func() {
		resp, err := plain.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: cacheWhale()})
		if err != nil || len(resp.GetUserIds()) != cacheFriends {
			t.Fatalf("mysql full %d err=%v", len(resp.GetUserIds()), err)
		}
	})
	report(t, "mysql_is_friend", cacheRepeat, func() {
		resp, err := plain.IsFriend(ctx, &relationpb.IsFriendReq{User1: cacheWhale(), User2: friend})
		if err != nil || !resp.GetInUser1Friends() || !resp.GetInUser2Friends() {
			t.Fatalf("mysql friend %+v err=%v", resp, err)
		}
	})

	lists.ResetStats()
	start := time.Now()
	resp, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: cacheWhale()})
	if err != nil || len(resp.GetUserIds()) != cacheFriends {
		t.Fatalf("cold full %d err=%v", len(resp.GetUserIds()), err)
	}
	t.Logf("RESULT name=cold_full_ids ms=%.3f %s", ms(time.Since(start)), formatStats(lists.Stats()))

	redisLists, stopRedis := relationcache.NewLists(ctx, cacheRedis(t), relationcache.DefaultConfig())
	t.Cleanup(stopRedis)
	redisSrv := logic.New(st, knownUsers{cacheWhale(): {}}, logic.Hooks{})
	redisSrv.UseCache(redisLists)
	start = time.Now()
	resp, err = redisSrv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: cacheWhale()})
	if err != nil || len(resp.GetUserIds()) != cacheFriends {
		t.Fatalf("redis full %d err=%v", len(resp.GetUserIds()), err)
	}
	t.Logf("RESULT name=redis_full_ids ms=%.3f %s", ms(time.Since(start)), formatStats(redisLists.Stats()))

	measure(t, lists, "local_full_ids", cacheRepeat, func() {
		resp, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: cacheWhale()})
		if err != nil || len(resp.GetUserIds()) != cacheFriends {
			t.Fatalf("local full %d err=%v", len(resp.GetUserIds()), err)
		}
	})
	if _, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: cacheWhale(), User2: friend}); err != nil {
		t.Fatal(err)
	}
	measure(t, lists, "local_is_friend", cacheRepeat, func() {
		resp, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: cacheWhale(), User2: friend})
		if err != nil || !resp.GetInUser1Friends() || !resp.GetInUser2Friends() {
			t.Fatalf("local friend %+v err=%v", resp, err)
		}
	})

	redisLists.ResetStats()
	start = time.Now()
	resp2, err := redisSrv.IsFriend(ctx, &relationpb.IsFriendReq{User1: cacheWhale(), User2: friend})
	if err != nil || !resp2.GetInUser1Friends() || !resp2.GetInUser2Friends() {
		t.Fatalf("redis friend %+v err=%v", resp2, err)
	}
	t.Logf("RESULT name=redis_is_friend ms=%.3f %s", ms(time.Since(start)), formatStats(redisLists.Stats()))
}

func TestLoadRelationCache(t *testing.T) {
	ctx, srv, lists, _, _ := openCacheBench(t)
	whale := cacheWhale()
	peers := make([]string, cachePeers)
	for i := range peers {
		peers[i] = benchID(10001 + i)
	}
	if _, err := srv.GetFullFriendUserIDs(ctx, &relationpb.GetFullFriendUserIDsReq{UserId: whale}); err != nil {
		t.Fatal(err)
	}
	for _, peer := range peers {
		resp, err := srv.IsFriend(ctx, &relationpb.IsFriendReq{User1: whale, User2: peer})
		if err != nil || !resp.GetInUser1Friends() || !resp.GetInUser2Friends() {
			t.Fatalf("warm %s %+v err=%v", peer, resp, err)
		}
	}
	lists.ResetStats()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var full, friend cacheLatency
	var wg sync.WaitGroup
	for worker := 0; worker < cacheReaders; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; runCtx.Err() == nil; i++ {
				peer := peers[(worker+i)%len(peers)]
				if i%5 == 0 {
					start := time.Now()
					resp, err := srv.GetFullFriendUserIDs(runCtx, &relationpb.GetFullFriendUserIDsReq{UserId: whale})
					if runCtx.Err() != nil {
						return
					}
					if err != nil || len(resp.GetUserIds()) != cacheFriends {
						t.Errorf("full %d err=%v", len(resp.GetUserIds()), err)
						cancel()
						return
					}
					full.add(time.Since(start))
					continue
				}
				start := time.Now()
				resp, err := srv.IsFriend(runCtx, &relationpb.IsFriendReq{User1: whale, User2: peer})
				if runCtx.Err() != nil {
					return
				}
				if err != nil || !resp.GetInUser1Friends() || !resp.GetInUser2Friends() {
					t.Errorf("friend %+v err=%v", resp, err)
					cancel()
					return
				}
				friend.add(time.Since(start))
			}
		}(worker)
	}
	time.Sleep(cacheLoadFor)
	cancel()
	wg.Wait()
	full.report(t, "load_local_full_ids")
	friend.report(t, "load_local_is_friend")
	stats := lists.Stats()
	t.Logf("RESULT name=load_hit_rate %s", formatStats(stats))
	if stats.LocalHit == 0 || stats.MySQLLoad != 0 {
		t.Fatalf("warm cache should hit locally and not load mysql: %+v", stats)
	}
}

func openCacheBench(t *testing.T) (context.Context, *logic.Server, *relationcache.Lists, *store.Store, string) {
	t.Helper()
	ctx := context.Background()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	createBenchDB(t, env)
	cfg := config.MySQL{
		Address:     []string{"127.0.0.1:3306"},
		Username:    env["MYSQL_USER"],
		Password:    env["MYSQL_PASSWORD"],
		Database:    benchDB,
		MaxOpenConn: 32,
		MaxIdleConn: 16,
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
	seedCacheBench(t, ctx, db)
	t.Cleanup(func() { clearBench(t, db) })

	rdb := cacheRedis(t)
	t.Cleanup(func() { deleteCacheKeys(t, rdb) })
	lists, stop := relationcache.NewLists(ctx, rdb, relationcache.DefaultConfig())
	t.Cleanup(stop)
	srv := logic.New(st, knownUsers{cacheWhale(): {}}, logic.Hooks{})
	srv.UseCache(lists)
	return ctx, srv, lists, st, benchID(10001)
}

func cacheWhale() string { return benchID(1) }

func seedCacheBench(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	clearBench(t, db)
	whale := cacheWhale()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	friends := make([]row, 0, cacheFriends*2)
	logs := make([]row, 0, cacheFriends+1)
	for i := 0; i < cacheFriends; i++ {
		id := benchID(10001 + i)
		pinned := 0
		if i < 20 {
			pinned = 1
		}
		created := base.Add(time.Duration(i) * time.Second)
		friends = append(friends, row{a: whale, b: id, at: created, n: pinned})
		friends = append(friends, row{a: id, b: whale, at: created, n: 0})
		logs = append(logs, row{a: whale, b: id, at: created, n: int(model.VersionInsert), v: uint64(i + 1)})
	}
	logs = append(logs, row{a: whale, b: model.SortElemID, at: base, n: int(model.VersionUpdate), v: uint64(cacheFriends + 1)})
	if err := insertRows(ctx, db, `INSERT INTO friend (owner_user_id, friend_user_id, remark, create_time, add_source, operator_user_id, is_pinned) VALUES %s`, "(?,?, '', ?, 2, ?, ?)", friends, func(r row) []any {
		return []any{r.a, r.b, r.at, r.a, r.n}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO friend_version (id, owner_user_id, version, deleted, last_update) VALUES (?, ?, ?, 0, ?)`, "benchc000000000000000001", whale, cacheFriends+1, base); err != nil {
		t.Fatal(err)
	}
	if err := insertRows(ctx, db, `INSERT INTO friend_version_log (owner_user_id, elem_id, state, version, last_update) VALUES %s`, "(?,?,?,?,?)", logs, func(r row) []any {
		return []any{r.a, r.b, r.n, r.v, r.at}
	}); err != nil {
		t.Fatal(err)
	}
}

func cacheRedis(t *testing.T) *redis.Client {
	t.Helper()
	env := readEnvFile(t, filepath.Join("..", "..", ".env"))
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: env["REDIS_PASSWORD"]})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis is not reachable: %v", err)
	}
	return rdb
}

func deleteCacheKeys(t *testing.T, rdb *redis.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	keys := make([]string, 0, cacheFriends*2+2)
	keys = append(keys, relationcache.FriendIDsKey(cacheWhale()), relationcache.VersionKey(cacheWhale()))
	for i := 0; i < cacheFriends; i++ {
		id := benchID(10001 + i)
		keys = append(keys, relationcache.FriendIDsKey(id), relationcache.VersionKey(id))
	}
	for start := 0; start < len(keys); start += 500 {
		end := start + 500
		if end > len(keys) {
			end = len(keys)
		}
		if err := rdb.Del(ctx, keys[start:end]...).Err(); err != nil {
			t.Errorf("delete cache keys: %v", err)
		}
	}
}

func measure(t *testing.T, lists *relationcache.Lists, name string, n int, fn func()) {
	t.Helper()
	lists.ResetStats()
	samples := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		samples[i] = time.Since(start)
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	t.Logf("RESULT name=%s n=%d min_ms=%.3f med_ms=%.3f max_ms=%.3f %s", name, n, ms(sorted[0]), ms(sorted[len(sorted)/2]), ms(sorted[len(sorted)-1]), formatStats(lists.Stats()))
}

func formatStats(s relationcache.Stats) string {
	return fmt.Sprintf("local_hit=%d local_miss=%d redis_hit=%d mysql=%d local_rate=%.3f avoid_mysql=%.3f",
		s.LocalHit, s.LocalMiss, s.RedisHit, s.MySQLLoad,
		rate(s.LocalHit, s.LocalHit+s.LocalMiss),
		rate(s.LocalHit+s.RedisHit, s.LocalHit+s.RedisHit+s.MySQLLoad))
}

func rate(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

type cacheLatency struct {
	mu      sync.Mutex
	samples []time.Duration
}

func (l *cacheLatency) add(d time.Duration) {
	l.mu.Lock()
	l.samples = append(l.samples, d)
	l.mu.Unlock()
}

func (l *cacheLatency) report(t *testing.T, name string) {
	t.Helper()
	l.mu.Lock()
	samples := append([]time.Duration(nil), l.samples...)
	l.mu.Unlock()
	if len(samples) == 0 {
		t.Fatalf("%s recorded no samples", name)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p := func(q float64) float64 {
		i := int(float64(len(samples)-1) * q)
		return ms(samples[i])
	}
	t.Logf("RESULT name=%s n=%d p50_ms=%.3f p95_ms=%.3f p99_ms=%.3f max_ms=%.3f", name, len(samples), p(0.50), p(0.95), p(0.99), ms(samples[len(samples)-1]))
}
