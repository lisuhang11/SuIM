package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/proto/userpb"
	svcconfig "github.com/lisuhang11/SuIM/services/user/internal/config"
	"github.com/lisuhang11/SuIM/services/user/internal/logic"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

func TestSession(t *testing.T) {
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
	redisCfg := config.Redis{
		Address:   []string{"127.0.0.1:6379"},
		Password:  env["REDIS_PASSWORD"],
		RedisMode: "standalone",
	}
	rdb, err := store.NewRedis(redisCfg)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn, rdb)
	if err != nil {
		t.Skipf("mysql or redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "user.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}

	const expireSeconds int64 = 2592000
	tokenCache, stopCache := logic.NewTokenCache(ctx, rdb, svcconfig.LocalCache{
		Topic:         "suim:user:token:delete:test",
		SlotNum:       4,
		SlotSize:      64,
		SuccessExpire: 60,
		FailedExpire:  5,
	})
	t.Cleanup(stopCache)
	srv := logic.New(st, token.NewIssuer(config.JWT{
		Secret:        "test-secret",
		ExpireSeconds: expireSeconds,
	}), tokenCache)

	email := fmt.Sprintf("itest-%d@example.com", time.Now().UnixNano())
	if _, err := srv.Register(ctx, &userpb.RegisterReq{Username: "a", Email: email, Password: "short"}); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidPassword) {
		t.Fatalf("short password code=%v err=%v", status.Code(err), err)
	}
	reg, err := srv.Register(ctx, &userpb.RegisterReq{Username: "Ada", Email: " " + email, Password: "secret1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupUser(t, mysqlCfg, reg.GetUser().GetUserId()) })
	if reg.GetUser().GetNickname() != "Ada" || reg.GetUser().GetEmail() != email {
		t.Fatalf("profile = %+v", reg.GetUser())
	}
	if _, err := srv.Register(ctx, &userpb.RegisterReq{Username: "Ada", Email: email, Password: "secret1"}); !isCode(err, codes.AlreadyExists, errorcode.UserEmailTaken) {
		t.Fatalf("duplicate code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1"}); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidLogin) {
		t.Fatalf("missing platform code=%v err=%v", status.Code(err), err)
	}
	if _, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "nope12", PlatformId: 1}); !isCode(err, codes.Unauthenticated, errorcode.UserBadCredential) {
		t.Fatalf("bad password code=%v err=%v", status.Code(err), err)
	}

	first, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}
	if first.GetToken() == "" || first.GetExpireTimeSeconds() != expireSeconds {
		t.Fatalf("login token=%q expire=%d", first.GetToken(), first.GetExpireTimeSeconds())
	}
	valid, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: first.GetToken()})
	if err != nil || !valid.GetValid() || valid.GetUser().GetUserId() != reg.GetUser().GetUserId() || valid.GetPlatformId() != 1 {
		t.Fatalf("validate first token: %+v err=%v", valid, err)
	}

	other, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: first.GetToken()}); err != nil {
		t.Fatalf("another platform login revoked this platform: %v", err)
	}

	second, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: first.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("previous session still valid after login: %v", err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: other.GetToken()}); err != nil {
		t.Fatalf("relogin on platform 1 revoked platform 2: %v", err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: second.GetToken()}); err != nil {
		t.Fatal(err)
	}

	if _, err := srv.Logout(ctx, &userpb.LogoutReq{Token: second.GetToken()}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Logout(ctx, &userpb.LogoutReq{Token: second.GetToken()}); err != nil {
		t.Fatalf("logout should be idempotent: %v", err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: second.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("token valid after logout: %v", err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: other.GetToken()}); err != nil {
		t.Fatalf("logout on platform 1 revoked platform 2: %v", err)
	}

	if err := dialRegisteredUser(ctx, t, st, srv); err != nil {
		t.Fatal(err)
	}
}

func TestProfile(t *testing.T) {
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
	rdb, err := store.NewRedis(config.Redis{
		Address:   []string{"127.0.0.1:6379"},
		Password:  env["REDIS_PASSWORD"],
		RedisMode: "standalone",
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, mysqlDSN(mysqlCfg), mysqlCfg.MaxOpenConn, mysqlCfg.MaxIdleConn, rdb)
	if err != nil {
		t.Skipf("mysql or redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	tokenCache, stopCache := logic.NewTokenCache(ctx, rdb, svcconfig.LocalCache{
		Topic: "suim:user:token:delete:profile", SlotNum: 4, SlotSize: 64, SuccessExpire: 60, FailedExpire: 5,
	})
	t.Cleanup(stopCache)
	srv := logic.New(st, token.NewIssuer(config.JWT{Secret: "test-secret", ExpireSeconds: 2592000}), tokenCache)

	email := fmt.Sprintf("profile-%d@example.com", time.Now().UnixNano())
	reg, err := srv.Register(ctx, &userpb.RegisterReq{Username: "Ada", Email: email, Password: "secret1"})
	if err != nil {
		t.Fatal(err)
	}
	id := reg.GetUser().GetUserId()
	t.Cleanup(func() { cleanupUser(t, mysqlCfg, id) })

	if _, err := srv.GetUserByID(ctx, &userpb.GetUserByIDReq{}); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidUserID) {
		t.Fatalf("empty user id: %v", err)
	}
	got, err := srv.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: id})
	if err != nil || got.GetUser().GetNickname() != "Ada" {
		t.Fatalf("get user: %+v err=%v", got, err)
	}
	if _, err := srv.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: "missing-user"}); !isCode(err, codes.NotFound, errorcode.UserNotFound) {
		t.Fatalf("missing user: %v", err)
	}
	batch, err := srv.GetUsersByIDs(ctx, &userpb.GetUsersByIDsReq{UserIds: []string{id, id, "missing-user"}})
	if err != nil || len(batch.GetUsers()) != 1 || batch.GetUsers()[id].GetEmail() != email {
		t.Fatalf("batch: %+v err=%v", batch, err)
	}
	if _, err := srv.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: " "}); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidProfile) {
		t.Fatalf("empty search: %v", err)
	}
	found, err := srv.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: email})
	if err != nil || len(found.GetUsers()) != 0 {
		t.Fatalf("search email: %+v err=%v", found, err)
	}
	found, err = srv.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: id})
	if err != nil || len(found.GetUsers()) != 1 || found.GetUsers()[0].GetEmail() != email {
		t.Fatalf("search id: %+v err=%v", found, err)
	}
	none, err := srv.SearchUsers(ctx, &userpb.SearchUsersReq{Keyword: "nobody-" + id})
	if err != nil || len(none.GetUsers()) != 0 {
		t.Fatalf("search miss: %+v err=%v", none, err)
	}

	nick := "Ada Lovelace"
	avatar := "https://example.com/a.png"
	if _, err := srv.UpdateUser(ctx, &userpb.UpdateUserReq{
		UserId: id, Nickname: &nick, AvatarUrl: &avatar,
	}); err != nil {
		t.Fatal(err)
	}
	gender := int32(1)
	birth := time.Date(1990, 5, 6, 0, 0, 0, 0, time.UTC).UnixMilli()
	if _, err := srv.UpdateUser(ctx, &userpb.UpdateUserReq{UserId: id, Gender: &gender, BirthTime: &birth}); err != nil {
		t.Fatal(err)
	}
	got, err = srv.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: id})
	if err != nil || got.GetUser().GetNickname() != "Ada Lovelace" || got.GetUser().GetAvatarUrl() != "https://example.com/a.png" ||
		got.GetUser().GetGender() != 1 || got.GetUser().GetBirthTime() != birth || !got.GetUser().GetIsActive() {
		t.Fatalf("updated profile: %+v err=%v", got.GetUser(), err)
	}
	if _, err := srv.UpdateUser(ctx, &userpb.UpdateUserReq{UserId: id}); !isCode(err, codes.InvalidArgument, errorcode.UserNothingToUpdate) {
		t.Fatalf("empty update: %v", err)
	}

	p1, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ChangePassword(ctx, &userpb.ChangePasswordReq{UserId: id, OldPassword: "nope12", NewPassword: "secret2"}); !isCode(err, codes.Unauthenticated, errorcode.UserPasswordMismatch) {
		t.Fatalf("bad old password: %v", err)
	}
	if _, err := srv.ChangePassword(ctx, &userpb.ChangePasswordReq{UserId: id, OldPassword: "secret1", NewPassword: "secret2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: p1.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("platform 1 survived password change: %v", err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: p2.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("platform 2 survived password change: %v", err)
	}
	if _, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret1", PlatformId: 1}); !isCode(err, codes.Unauthenticated, errorcode.UserBadCredential) {
		t.Fatalf("old password still works: %v", err)
	}
	fresh, err := srv.Login(ctx, &userpb.LoginReq{Email: email, Password: "secret2", PlatformId: 1})
	if err != nil {
		t.Fatal(err)
	}

	inactive := false
	if _, err := srv.UpdateUser(ctx, &userpb.UpdateUserReq{UserId: id, IsActive: &inactive}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: fresh.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("token survived disable: %v", err)
	}
	if _, err := srv.DeleteUser(ctx, &userpb.DeleteUserReq{UserId: id}); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.GetUserByID(ctx, &userpb.GetUserByIDReq{UserId: id}); !isCode(err, codes.NotFound, errorcode.UserNotFound) {
		t.Fatalf("deleted user still readable: %v", err)
	}
	if _, err := srv.DeleteUser(ctx, &userpb.DeleteUserReq{UserId: id}); !isCode(err, codes.NotFound, errorcode.UserNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func isCode(err error, grpc codes.Code, code errorcode.Code) bool {
	got, ok := errorcode.CodeOf(err)
	return ok && got == code && status.Code(err) == grpc
}

func dialRegisteredUser(ctx context.Context, t *testing.T, st *store.Store, handler userpb.UserServiceServer) error {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	gs := grpc.NewServer()
	userpb.RegisterUserServiceServer(gs, handler)
	go gs.Serve(lis)
	t.Cleanup(gs.Stop)

	reg, err := register.New(&register.Config{
		Type: "etcd",
		Etcd: register.EtcdConfig{Endpoints: []string{"127.0.0.1:2379"}, DialTimeout: 5, TTL: 10},
	})
	if err != nil {
		return err
	}
	t.Cleanup(func() { _ = reg.Close() })
	id, err := logic.NewID()
	if err != nil {
		return err
	}
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{ID: id, Name: "user", Addr: lis.Addr().String()})
	cancel()
	if err != nil {
		t.Skipf("etcd is not reachable: %v", err)
	}
	t.Cleanup(func() {
		off, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = reg.Deregister(off)
	})

	conn, err := reg.Dial(ctx, "user", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := userpb.NewUserServiceClient(conn)

	email := fmt.Sprintf("itest-dial-%d@example.com", time.Now().UnixNano())
	var resp *userpb.RegisterResp
	deadline := time.Now().Add(5 * time.Second)
	for {
		callCtx, cancel := context.WithTimeout(ctx, time.Second)
		resp, err = client.Register(callCtx, &userpb.RegisterReq{Username: "Dial", Email: email, Password: "secret1"})
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dial user service through etcd: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Cleanup(func() {
		env := readEnvFile(t, filepath.Join("..", "..", ".env"))
		cleanupUser(t, config.MySQL{
			Address:  []string{"127.0.0.1:3306"},
			Username: env["MYSQL_USER"],
			Password: env["MYSQL_PASSWORD"],
			Database: env["MYSQL_DATABASE"],
		}, resp.GetUser().GetUserId())
	})
	_ = st
	if resp.GetUser().GetEmail() != email {
		return fmt.Errorf("dialed register email = %s", resp.GetUser().GetEmail())
	}
	return nil
}

func cleanupUser(t *testing.T, cfg config.MySQL, userID string) {
	t.Helper()
	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		t.Errorf("cleanup connect: %v", err)
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE user_id = ?`, userID); err != nil {
		t.Errorf("cleanup tokens: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM user WHERE user_id = ?`, userID); err != nil {
		t.Errorf("cleanup user: %v", err)
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
