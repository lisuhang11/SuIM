//go:build perf

package main

import (
	"sort"
	"testing"
	"time"

	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/logic"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

func TestPerfUser(t *testing.T) {
	ctx, db, st, rdb, issuer := openUserBench(t)
	seedUserBench(t, ctx, db)
	hot := userBenchID(userBenchHot)
	raw := insertBenchToken(t, ctx, db, issuer, hot)
	var extra []string
	t.Cleanup(func() {
		deleteBenchTokenKeys(t, ctx, rdb, append(extra, raw)...)
	})

	t.Logf("EXPLAIN get_user %s", explainUser(t, ctx, db, `SELECT user_id, email, password_hash, nickname, is_active FROM user WHERE user_id = ?`, hot))
	t.Logf("EXPLAIN get_email %s", explainUser(t, ctx, db, `SELECT user_id, email, password_hash, nickname, is_active FROM user WHERE email = ?`, hot+"@bench.test"))
	t.Logf("EXPLAIN token %s", explainUser(t, ctx, db, `SELECT id, user_id, platform_id, token, token_hash, token_type, expires_at, is_revoked FROM auth_tokens WHERE token_hash = ?`, token.Hash(raw)))

	plain := logic.New(st, issuer, nil)
	timeCalls(t, "get_user", 30, func() {
		resp, err := plain.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: hot})
		if err != nil || resp.GetUser().GetUserId() != hot {
			t.Fatalf("get user %+v err=%v", resp, err)
		}
	})
	timeCalls(t, "get_users_20", 30, func() {
		ids := make([]string, 20)
		for i := range ids {
			ids[i] = userBenchID(userBenchHot + i)
		}
		resp, err := plain.GetUsersByIDs(ctx, &userpb.GetUsersByIDsReq{UserIds: ids})
		if err != nil || len(resp.GetUsers()) != 20 {
			t.Fatalf("batch %d err=%v", len(resp.GetUsers()), err)
		}
	})
	timeCalls(t, "search_id", 30, func() {
		resp, err := plain.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: hot})
		if err != nil || len(resp.GetUsers()) != 1 {
			t.Fatalf("search %+v err=%v", resp, err)
		}
	})

	hash := token.Hash(raw)
	timeCalls(t, "validate_mysql", 30, func() {
		if err := rdb.Del(ctx, "suim:token:"+hash).Err(); err != nil {
			t.Fatal(err)
		}
		mustValidate(t, ctx, plain, raw, hot)
	})

	hits := &tokenHits{}
	cache := newBenchCache(hits)
	cached := logic.New(st, issuer, cache)
	reportLayer := func(name string, n int, before func()) {
		samples := make([]time.Duration, n)
		for i := 0; i < n; i++ {
			if before != nil {
				before()
			}
			start := time.Now()
			mustValidate(t, ctx, cached, raw, hot)
			samples[i] = time.Since(start)
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
		t.Logf("RESULT name=%s n=%d min_ms=%.3f p50_ms=%.3f max_ms=%.3f %s",
			name, n, ms(samples[0]), ms(percentile(samples, 0.50)), ms(samples[len(samples)-1]), hits.String())
	}
	hits.hit.Store(0)
	hits.miss.Store(0)
	hits.failed.Store(0)
	reportLayer("validate_redis", 30, func() { cache.DelLocal(ctx, hash) })
	hits.hit.Store(0)
	hits.miss.Store(0)
	hits.failed.Store(0)
	reportLayer("validate_local", 30, nil)

	timeCalls(t, "login", 10, func() {
		resp, err := plain.Login(ctx, &userpb.LoginReq{Account: hot + "@bench.test", Password: userBenchPass, PlatformId: 2})
		if err != nil || resp.GetToken() == "" {
			t.Fatalf("login err=%v", err)
		}
		extra = append(extra, resp.GetToken())
	})
}
