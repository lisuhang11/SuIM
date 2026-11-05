package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	svcconfig "github.com/lisuhang11/SuIM/services/user/internal/config"
	"github.com/lisuhang11/SuIM/services/user/internal/logic"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"

	"google.golang.org/grpc/codes"
)

func TestDisabledLogin(t *testing.T) {
	ctx, srv, cfg, cleanup := openUser(t, "suim:user:token:delete:disabled")
	defer cleanup()
	email := fmt.Sprintf("disabled-%d@example.com", time.Now().UnixNano())
	id := registerUser(t, ctx, srv, cfg, email, "Ada")
	session, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}
	inactive := false
	if _, err := srv.UpdateUser(ctx, &userpb.UpdateUserReq{Token: session.GetToken(), IsActive: &inactive}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Login(ctx, &userpb.LoginReq{Account: email, Password: "secret1", PlatformId: 1}); !isCode(err, codes.PermissionDenied, errorcode.UserDisabled) {
		t.Fatalf("disabled login: %v", err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: session.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("disabled session: %v", err)
	}
	_ = id
}

func TestSearchMatchesUserIDOnly(t *testing.T) {
	ctx, srv, cfg, cleanup := openUser(t, "suim:user:token:delete:search")
	defer cleanup()
	email := fmt.Sprintf("search-%d@example.com", time.Now().UnixNano())
	nick := fmt.Sprintf("Nick%d", time.Now().UnixNano())
	id := registerUser(t, ctx, srv, cfg, email, nick)
	found, err := srv.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: nick})
	if err != nil || len(found.GetUsers()) != 0 {
		t.Fatalf("nickname search: %+v err=%v", found, err)
	}
	found, err = srv.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: email})
	if err != nil || len(found.GetUsers()) != 0 {
		t.Fatalf("email search: %+v err=%v", found, err)
	}
	found, err = srv.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: id})
	if err != nil || len(found.GetUsers()) != 1 || found.GetUsers()[0].GetUserId() != id || found.GetUsers()[0].GetEmail() != "" {
		t.Fatalf("id search: %+v err=%v", found, err)
	}
}

func TestExpiredSessionCanLogout(t *testing.T) {
	ctx, srv, cfg, rdb, cleanup := openUserRedis(t, "suim:user:token:delete:expire")
	defer cleanup()
	email := fmt.Sprintf("expire-%d@example.com", time.Now().UnixNano())
	registerUser(t, ctx, srv, cfg, email, "Ada")
	session, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}
	hash := token.Hash(session.GetToken())
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `UPDATE auth_tokens SET expires_at = ? WHERE token_hash = ?`, time.Now().Add(-time.Minute).UTC(), hash); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Del(ctx, "suim:token:"+hash).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: session.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("expired token still valid: %v", err)
	}
	if _, err := srv.Logout(ctx, &userpb.LogoutReq{Token: session.GetToken()}); err != nil {
		t.Fatalf("logout expired: %v", err)
	}
}

func TestTokenCacheFollowsLogout(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, "../../.env")
	cfg := config.MySQL{
		Address: []string{"127.0.0.1:3306"}, Username: env["MYSQL_USER"], Password: env["MYSQL_PASSWORD"],
		Database: env["MYSQL_DATABASE"], MaxOpenConn: 8, MaxIdleConn: 4,
	}
	rdb, err := store.NewRedis(config.Redis{Address: []string{"127.0.0.1:6379"}, Password: env["REDIS_PASSWORD"], RedisMode: "standalone"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn, rdb)
	if err != nil {
		t.Skipf("mysql or redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	topic := fmt.Sprintf("suim:user:token:delete:pair:%d", time.Now().UnixNano())
	cacheCfg := svcconfig.LocalCache{Topic: topic, SlotNum: 4, SlotSize: 64, SuccessExpire: 60, FailedExpire: 5}
	cacheA, stopA := logic.NewTokenCache(ctx, rdb, cacheCfg)
	cacheB, stopB := logic.NewTokenCache(ctx, rdb, cacheCfg)
	t.Cleanup(stopA)
	t.Cleanup(stopB)
	waitSubscribers(t, ctx, rdb, topic, 2)
	issuer := token.NewIssuer(config.JWT{Secret: "test-secret", ExpireSeconds: 3600})
	a := logic.New(st, issuer, cacheA)
	b := logic.New(st, issuer, cacheB)
	email := fmt.Sprintf("cache-%d@example.com", time.Now().UnixNano())
	registerUser(t, ctx, a, cfg, email, "Ada")
	session, err := a.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: session.GetToken()}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: session.GetToken()}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Logout(ctx, &userpb.LogoutReq{Token: session.GetToken()}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err = b.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: session.GetToken()})
		if isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("other cache kept the token: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestConcurrentSamePlatformLogin(t *testing.T) {
	ctx, srv, cfg, cleanup := openUser(t, "suim:user:token:delete:race")
	defer cleanup()
	email := fmt.Sprintf("race-%d@example.com", time.Now().UnixNano())
	registerUser(t, ctx, srv, cfg, email, "Ada")
	var mu sync.Mutex
	var tokens []string
	var fails []string
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fails = append(fails, err.Error())
				return
			}
			tokens = append(tokens, resp.GetToken())
		}()
	}
	wg.Wait()
	if len(tokens) == 0 {
		t.Fatalf("no login succeeded: %v", fails)
	}
	for _, msg := range fails {
		if msg != errorcode.ErrInternal.WithMsg("create session").Error() {
			t.Fatalf("unexpected login error %q", msg)
		}
	}
	valid := 0
	for _, raw := range tokens {
		if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: raw}); err == nil {
			valid++
		}
	}
	if valid != 1 {
		t.Fatalf("live tokens=%d ok=%d fail=%d", valid, len(tokens), len(fails))
	}
	t.Logf("logins ok=%d fail=%d live=%d", len(tokens), len(fails), valid)
}

func TestProfileCacheFollowsUpdate(t *testing.T) {
	ctx := context.Background()
	env := readEnvFile(t, "../../.env")
	cfg := config.MySQL{
		Address: []string{"127.0.0.1:3306"}, Username: env["MYSQL_USER"], Password: env["MYSQL_PASSWORD"],
		Database: env["MYSQL_DATABASE"], MaxOpenConn: 16, MaxIdleConn: 8,
	}
	rdb, err := store.NewRedis(config.Redis{Address: []string{"127.0.0.1:6379"}, Password: env["REDIS_PASSWORD"], RedisMode: "standalone"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn, rdb)
	if err != nil {
		_ = rdb.Close()
		t.Skipf("mysql or redis is not reachable: %v", err)
	}
	defer st.Close()
	topic := fmt.Sprintf("suim:user:profile:test:%d", time.Now().UnixNano())
	local := svcconfig.LocalCache{Topic: topic, SlotNum: 4, SlotSize: 64, SuccessExpire: 60, FailedExpire: 5}
	a, stopA := logic.NewProfileCache(ctx, rdb, st, local)
	b, stopB := logic.NewProfileCache(ctx, rdb, st, local)
	defer stopA()
	defer stopB()
	waitSubscribers(t, ctx, rdb, topic, 2)
	issuer := token.NewIssuer(config.JWT{Secret: "test-secret", ExpireSeconds: 3600})
	left := logic.New(st, issuer, nil)
	right := logic.New(st, issuer, nil)
	left.UseProfiles(a)
	right.UseProfiles(b)

	email := fmt.Sprintf("profile-%d@example.com", time.Now().UnixNano())
	reg, err := left.Register(ctx, &userpb.RegisterReq{Username: "Ada", Email: email, Password: "secret1"})
	if err != nil {
		t.Fatal(err)
	}
	id := reg.GetUser().GetUserId()
	t.Cleanup(func() {
		cleanupUser(t, cfg, id)
		_ = rdb.Del(context.Background(), "suim:user:profile:"+id).Err()
	})
	session, err := left.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := left.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: id, Token: session.GetToken()}); err != nil {
		t.Fatal(err)
	}
	if _, err := right.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: id}); err != nil {
		t.Fatal(err)
	}
	raw, err := rdb.Get(ctx, "suim:user:profile:"+id).Result()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "password_hash") || strings.Contains(raw, "$2") {
		t.Fatal("profile cache stored a password hash")
	}
	if _, err := left.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 2}); err != nil {
		t.Fatal(err)
	}
	nick := "Bea"
	if _, err := left.UpdateUser(ctx, &userpb.UpdateUserReq{Token: session.GetToken(), Nickname: &nick}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := right.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: id})
		if err == nil && got.GetUser().GetNickname() == "Bea" {
			return
		}
		if time.Now().After(deadline) {
			name := ""
			if got != nil {
				name = got.GetUser().GetNickname()
			}
			t.Fatalf("other instance nickname=%q err=%v", name, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func openUser(t *testing.T, topic string) (context.Context, *logic.Server, config.MySQL, func()) {
	t.Helper()
	ctx, srv, cfg, _, cleanup := openUserRedis(t, topic)
	return ctx, srv, cfg, cleanup
}

func openUserRedis(t *testing.T, topic string) (context.Context, *logic.Server, config.MySQL, redis.UniversalClient, func()) {
	t.Helper()
	ctx := context.Background()
	env := readEnvFile(t, "../../.env")
	cfg := config.MySQL{
		Address: []string{"127.0.0.1:3306"}, Username: env["MYSQL_USER"], Password: env["MYSQL_PASSWORD"],
		Database: env["MYSQL_DATABASE"], MaxOpenConn: 16, MaxIdleConn: 8,
	}
	rdb, err := store.NewRedis(config.Redis{Address: []string{"127.0.0.1:6379"}, Password: env["REDIS_PASSWORD"], RedisMode: "standalone"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, mysqlDSN(cfg), cfg.MaxOpenConn, cfg.MaxIdleConn, rdb)
	if err != nil {
		_ = rdb.Close()
		t.Skipf("mysql or redis is not reachable: %v", err)
	}
	cache, stop := logic.NewTokenCache(ctx, rdb, svcconfig.LocalCache{
		Topic: topic, SlotNum: 4, SlotSize: 64, SuccessExpire: 60, FailedExpire: 5,
	})
	srv := logic.New(st, token.NewIssuer(config.JWT{Secret: "test-secret", ExpireSeconds: 3600}), cache)
	return ctx, srv, cfg, rdb, func() {
		stop()
		_ = st.Close()
	}
}

func registerUser(t *testing.T, ctx context.Context, srv *logic.Server, cfg config.MySQL, email, nick string) string {
	t.Helper()
	reg, err := srv.Register(ctx, &userpb.RegisterReq{Username: nick, Email: email, Password: "secret1"})
	if err != nil {
		t.Fatal(err)
	}
	id := reg.GetUser().GetUserId()
	t.Cleanup(func() { cleanupUser(t, cfg, id) })
	return id
}

func waitSubscribers(t *testing.T, ctx context.Context, rdb redis.UniversalClient, topic string, want int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		n, err := rdb.PubSubNumSub(ctx, topic).Result()
		if err == nil && n[topic] >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscribers %v err=%v", n, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
