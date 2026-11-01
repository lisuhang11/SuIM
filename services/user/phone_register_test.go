package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/user"
	svcconfig "github.com/lisuhang11/SuIM/services/user/internal/config"
	"github.com/lisuhang11/SuIM/services/user/internal/logic"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

var testPhoneSeq atomic.Int64

func nextTestPhone() string {
	n := time.Now().UnixNano()%1_000_000_000 + testPhoneSeq.Add(1)
	return fmt.Sprintf("13%09d", n%1_000_000_000)
}

type captureSMS struct {
	mu    sync.Mutex
	calls int
	last  string
	fail  bool
}

func (c *captureSMS) Send(_ context.Context, _, code, _ string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.last = code
	if c.fail {
		return "", errors.New("provider down")
	}
	return "ok", nil
}

func (c *captureSMS) snapshot() (int, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.last
}

type scriptCaptcha struct{ reject bool }

func (c scriptCaptcha) Verify(context.Context, string, string, string, string) error {
	if c.reject {
		return errors.New("rejected")
	}
	return nil
}

func TestPhoneRegister(t *testing.T) {
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
	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", "user.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	tokenCache, stopCache := logic.NewTokenCache(ctx, rdb, svcconfig.LocalCache{
		Topic: "suim:user:token:delete:phone", SlotNum: 4, SlotSize: 64, SuccessExpire: 60, FailedExpire: 5,
	})
	t.Cleanup(stopCache)
	sender := &captureSMS{}
	const pepper = "test-secret"
	srv := logic.New(st, token.NewIssuer(config.JWT{Secret: pepper, ExpireSeconds: 2592000}), tokenCache)
	policy := testSMSPolicy()
	srv.UseSMS(sender, nil, policy, pepper)

	phone := nextTestPhone()
	sent, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: "+86 " + phone[:3] + "-" + phone[3:7] + "-" + phone[7:], Scene: logic.SceneRegister, ClientIp: "10.1.1.1"})
	if err != nil || sent.GetExpireSeconds() != 300 {
		t.Fatalf("send %+v err=%v", sent, err)
	}
	calls, code := sender.snapshot()
	if calls != 1 || len(code) != 6 {
		t.Fatalf("sender calls=%d code=%q", calls, code)
	}
	if _, err := srv.RegisterPhone(ctx, phoneReq(phone, code, false)); !isCode(err, codes.InvalidArgument, errorcode.UserProtocolRequired) {
		t.Fatalf("protocol code=%v err=%v", codes.OK, err)
	}
	short := phoneReq(phone, code, true)
	short.Password = "123"
	if _, err := srv.RegisterPhone(ctx, short); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidPassword) {
		t.Fatalf("short password err=%v", err)
	}
	reg, err := srv.RegisterPhone(ctx, phoneReq(phone, code, true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupUser(t, mysqlCfg, reg.GetUser().GetUserId()) })
	if reg.GetToken() == "" || reg.GetUser().GetRegisterType() != model.RegisterTypePhone || reg.GetUser().GetNickname() != "用户"+phone[len(phone)-4:] {
		t.Fatalf("register %+v", reg.GetUser())
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: reg.GetToken()}); err != nil {
		t.Fatal(err)
	}
	again, err := srv.RegisterPhone(ctx, phoneReq(phone, code, true))
	if err != nil || again.GetUser().GetUserId() != reg.GetUser().GetUserId() {
		t.Fatalf("retry user=%s err=%v", again.GetUser().GetUserId(), err)
	}
	if _, err := srv.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: reg.GetToken()}); !isCode(err, codes.Unauthenticated, errorcode.UserInvalidToken) {
		t.Fatalf("retry did not replace the session: %v", err)
	}

	quietCalls, _ := sender.snapshot()
	quiet, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: phone, Scene: logic.SceneRegister})
	if err != nil || quiet.GetExpireSeconds() != sent.GetExpireSeconds() {
		t.Fatalf("quiet send %+v err=%v", quiet, err)
	}
	if callsNow, _ := sender.snapshot(); callsNow != quietCalls {
		t.Fatalf("registered phone sent sms calls %d -> %d", quietCalls, callsNow)
	}
	unknown := nextTestPhone()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: unknown, Scene: logic.SceneLogin}); err != nil {
		t.Fatal(err)
	}
	if callsNow, _ := sender.snapshot(); callsNow != quietCalls {
		t.Fatalf("login scene for a new phone sent sms")
	}

	loginSent, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: phone, Scene: logic.SceneLogin, DeviceId: "device-a"})
	if err != nil || loginSent.GetExpireSeconds() != 300 {
		t.Fatalf("login send %+v err=%v", loginSent, err)
	}
	_, loginCode := sender.snapshot()
	asRegister := phoneReq(phone, loginCode, true)
	if _, err := srv.RegisterPhone(ctx, asRegister); !isCode(err, codes.Unauthenticated, errorcode.UserSmsCodeInvalid) {
		t.Fatalf("login code registered err=%v", err)
	}
	asRegister.Scene = logic.SceneLogin
	if _, err := srv.RegisterPhone(ctx, asRegister); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidScene) {
		t.Fatalf("login scene register err=%v", err)
	}
	logged, err := srv.Login(ctx, &userpb.LoginReq{Phone: phone, Password: "secret1", PlatformId: 1})
	if err != nil || logged.GetUser().GetUserId() != reg.GetUser().GetUserId() {
		t.Fatalf("phone login %+v err=%v", logged.GetUser(), err)
	}
	if _, err := srv.Login(ctx, &userpb.LoginReq{Phone: phone, Email: "a@b.c", Password: "secret1", PlatformId: 1}); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidLogin) {
		t.Fatalf("both identifiers err=%v", err)
	}
	if _, err := srv.Login(ctx, &userpb.LoginReq{Phone: "12345", Password: "secret1", PlatformId: 1}); !isCode(err, codes.Unauthenticated, errorcode.UserBadCredential) {
		t.Fatalf("bad phone login err=%v", err)
	}

	noPassPhone := nextTestPhone()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: noPassPhone, Scene: logic.SceneRegister}); err != nil {
		t.Fatal(err)
	}
	_, noPassCode := sender.snapshot()
	noPassReq := phoneReq(noPassPhone, noPassCode, true)
	noPassReq.Password = ""
	noPass, err := srv.RegisterPhone(ctx, noPassReq)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupUser(t, mysqlCfg, noPass.GetUser().GetUserId()) })
	if _, err := srv.Login(ctx, &userpb.LoginReq{Phone: noPassPhone, Password: "", PlatformId: 1}); !isCode(err, codes.InvalidArgument, errorcode.UserInvalidLogin) {
		t.Fatalf("empty password err=%v", err)
	}
	if _, err := srv.Login(ctx, &userpb.LoginReq{Phone: noPassPhone, Password: "secret1", PlatformId: 1}); !isCode(err, codes.Unauthenticated, errorcode.UserBadCredential) {
		t.Fatalf("unset password accepted err=%v", err)
	}

	resendPhone := nextTestPhone()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: resendPhone, Scene: logic.SceneRegister}); err != nil {
		t.Fatal(err)
	}
	_, firstCode := sender.snapshot()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: resendPhone, Scene: logic.SceneRegister}); err != nil {
		t.Fatal(err)
	}
	_, secondCode := sender.snapshot()
	if _, err := srv.RegisterPhone(ctx, phoneReq(resendPhone, firstCode, true)); !isCode(err, codes.Unauthenticated, errorcode.UserSmsCodeInvalid) {
		t.Fatalf("old code still valid err=%v", err)
	}
	resend, err := srv.RegisterPhone(ctx, phoneReq(resendPhone, secondCode, true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupUser(t, mysqlCfg, resend.GetUser().GetUserId()) })

	lockPhone := nextTestPhone()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: lockPhone, Scene: logic.SceneRegister}); err != nil {
		t.Fatal(err)
	}
	_, lockCode := sender.snapshot()
	for i := 0; i < 5; i++ {
		if _, err := srv.RegisterPhone(ctx, phoneReq(lockPhone, "000000", true)); !isCode(err, codes.Unauthenticated, errorcode.UserSmsCodeInvalid) {
			t.Fatalf("wrong code %d err=%v", i, err)
		}
	}
	if _, err := srv.RegisterPhone(ctx, phoneReq(lockPhone, lockCode, true)); !isCode(err, codes.Unauthenticated, errorcode.UserSmsCodeInvalid) {
		t.Fatalf("locked code err=%v", err)
	}

	limitPhone := nextTestPhone()
	for i := 0; i < 5; i++ {
		if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: limitPhone, Scene: logic.SceneRegister}); err != nil {
			t.Fatalf("send %d err=%v", i, err)
		}
	}
	beforeLimit, _ := sender.snapshot()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: limitPhone, Scene: logic.SceneRegister}); !isCode(err, codes.ResourceExhausted, errorcode.UserSmsLimited) {
		t.Fatalf("rate limit err=%v", err)
	}
	if after, _ := sender.snapshot(); after != beforeLimit {
		t.Fatalf("limited send still called the provider")
	}

	sender.fail = true
	failPhone := nextTestPhone()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: failPhone, Scene: logic.SceneRegister}); !isCode(err, codes.Unavailable, errorcode.UserSmsFailed) {
		t.Fatalf("provider failure err=%v", err)
	}
	if _, err := srv.RegisterPhone(ctx, phoneReq(failPhone, "000000", true)); !isCode(err, codes.Unauthenticated, errorcode.UserSmsCodeInvalid) {
		t.Fatalf("code stored after provider failure err=%v", err)
	}
	sender.fail = false
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: failPhone, Scene: logic.SceneRegister}); err != nil {
		t.Fatal(err)
	}
	_, failCode := sender.snapshot()
	failed, err := srv.RegisterPhone(ctx, phoneReq(failPhone, failCode, true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupUser(t, mysqlCfg, failed.GetUser().GetUserId()) })

	takenPhone := nextTestPhone()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: takenPhone, Scene: logic.SceneRegister}); err != nil {
		t.Fatal(err)
	}
	_, takenCode := sender.snapshot()
	area, national, err := logic.NormalizePhone(takenPhone)
	if err != nil {
		t.Fatal(err)
	}
	takenID, err := logic.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreatePhoneUser(ctx, &model.User{
		UserID: takenID, PasswordHash: "x", Nickname: "taken", AreaCode: area, PhoneNumber: national, RegisterReceipt: "other-receipt",
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupUser(t, mysqlCfg, takenID) })
	if _, err := srv.RegisterPhone(ctx, phoneReq(takenPhone, takenCode, true)); !isCode(err, codes.AlreadyExists, errorcode.UserPhoneTaken) {
		t.Fatalf("phone taken err=%v", err)
	}

	blockedPhone := nextTestPhone()
	blockedArea, blockedNational, err := logic.NormalizePhone(blockedPhone)
	if err != nil {
		t.Fatal(err)
	}
	blockKey := "sms:block:phone:" + blockedArea + blockedNational
	if err := rdb.Set(ctx, blockKey, "1", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.Del(context.Background(), blockKey).Err() })
	beforeBlock, _ := sender.snapshot()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: blockedPhone, Scene: logic.SceneRegister}); !isCode(err, codes.PermissionDenied, errorcode.UserSmsRejected) {
		t.Fatalf("block err=%v", err)
	}
	if after, _ := sender.snapshot(); after != beforeBlock {
		t.Fatalf("blocked send called the provider")
	}

	srv.UseSMS(sender, scriptCaptcha{reject: true}, withCaptcha(policy, true), pepper)
	captchaPhone := nextTestPhone()
	beforeCaptcha, _ := sender.snapshot()
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: captchaPhone, Scene: logic.SceneRegister, CaptchaTicket: "bad"}); !isCode(err, codes.InvalidArgument, errorcode.UserCaptchaInvalid) {
		t.Fatalf("captcha err=%v", err)
	}
	if after, _ := sender.snapshot(); after != beforeCaptcha {
		t.Fatalf("bad captcha called the provider")
	}
	srv.UseSMS(sender, scriptCaptcha{}, withCaptcha(policy, true), pepper)
	if _, err := srv.SendSmsCode(ctx, &userpb.SendSmsCodeReq{Phone: captchaPhone, Scene: logic.SceneRegister, CaptchaTicket: "ok"}); err != nil {
		t.Fatalf("captcha passed err=%v", err)
	}
}

func phoneReq(phone, code string, agree bool) *userpb.RegisterPhoneReq {
	return &userpb.RegisterPhoneReq{
		Phone:           phone,
		Code:            code,
		Scene:           logic.SceneRegister,
		Password:        "secret1",
		AgreeProtocol:   agree,
		ProtocolVersion: "v1",
		PlatformId:      1,
		ClientIp:        "10.1.1.1",
	}
}

func testSMSPolicy() logic.SMSPolicy {
	return logic.SMSPolicy{
		CodeTTL:         300 * time.Second,
		RetryTTL:        120 * time.Second,
		PhonePerMinute:  5,
		PhonePerHour:    20,
		PhonePerDay:     20,
		IPPerMinute:     100,
		DevicePerMinute: 100,
		MaxAttempts:     5,
	}
}

func withCaptcha(p logic.SMSPolicy, required bool) logic.SMSPolicy {
	p.CaptchaRequired = required
	return p
}
