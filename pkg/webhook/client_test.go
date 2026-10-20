package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

type testReq struct {
	CommonCallbackReq
	UserID string `json:"userID"`
}

func TestSyncPost(t *testing.T) {
	var gotHeader, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/beforeAddFriend" {
			t.Errorf("path = %s", r.URL.Path)
		}
		gotHeader = r.Header.Get(HeaderOperationID)
		var body testReq
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		gotUser = body.UserID
		if body.GetCallbackCommand() != "beforeAddFriend" {
			t.Errorf("command = %s", body.GetCallbackCommand())
		}
		_ = json.NewEncoder(w).Encode(CommonCallbackResp{})
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL + "/")
	defer c.Close()

	ctx := WithOperationID(context.Background(), "op-1")
	resp := &CommonCallbackResp{}
	err := c.SyncPost(ctx, "beforeAddFriend", &testReq{
		CommonCallbackReq: CommonCallbackReq{CallbackCommand: "beforeAddFriend", OperationID: "op-1"},
		UserID:            "u1",
	}, resp, &BeforeConfig{Enable: true, Timeout: 2})
	if err != nil {
		t.Fatal(err)
	}
	if gotHeader != "op-1" || gotUser != "u1" {
		t.Fatalf("header=%s user=%s", gotHeader, gotUser)
	}
}

func TestSyncPostResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/reject":
			_ = json.NewEncoder(w).Encode(CommonCallbackResp{ActionCode: ActionForbidden, ErrMsg: "no", ErrDlt: "detail"})
		case "/next":
			_ = json.NewEncoder(w).Encode(CommonCallbackResp{NextCode: NextCodeContinue})
		default:
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, "down")
		}
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL)
	defer c.Close()
	before := &BeforeConfig{Timeout: 2}

	err := c.SyncPost(context.Background(), "reject", &testReq{}, &CommonCallbackResp{}, before)
	if !errors.Is(err, errorcode.ErrWebhookRejected) || err.Error() != "no: detail" {
		t.Fatalf("reject err=%v", err)
	}
	err = c.SyncPost(context.Background(), "next", &testReq{}, &CommonCallbackResp{}, before)
	if !errors.Is(err, errorcode.ErrWebhookContinue) {
		t.Fatalf("continue err=%v", err)
	}
	err = c.SyncPost(context.Background(), "bad", &testReq{}, &CommonCallbackResp{}, before)
	if !errors.Is(err, errorcode.ErrWebhookNetwork) {
		t.Fatalf("status err=%v", err)
	}
	err = c.SyncPost(context.Background(), "bad", &testReq{}, nil, before)
	if !errors.Is(err, errorcode.ErrWebhookData) {
		t.Fatalf("nil resp err=%v", err)
	}
}

func TestAsyncPostWithQuery(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.URL.Query().Get(Key)
		_ = json.NewEncoder(w).Encode(CommonCallbackResp{})
	}))
	defer srv.Close()

	c := NewWebhookClient(srv.URL)
	defer c.Close()

	c.AsyncPost(context.Background(), "after", &testReq{}, &CommonCallbackResp{}, &AfterConfig{Enable: false, Timeout: 2})
	select {
	case key := <-got:
		t.Fatalf("disabled webhook posted key=%s", key)
	case <-time.After(50 * time.Millisecond):
	}

	c.AsyncPostWithQuery(context.Background(), "after", &testReq{}, &CommonCallbackResp{}, &AfterConfig{Enable: true, Timeout: 2}, map[string]string{Key: "secret"})
	select {
	case key := <-got:
		if key != "secret" {
			t.Fatalf("key = %s", key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("async post did not arrive")
	}
}

func TestWithCondition(t *testing.T) {
	called := false
	err := WithCondition(context.Background(), &BeforeConfig{Enable: false}, func(context.Context) error {
		called = true
		return errors.New("should not run")
	})
	if err != nil || called {
		t.Fatalf("disabled called=%v err=%v", called, err)
	}
	err = WithCondition(context.Background(), &BeforeConfig{Enable: true}, func(context.Context) error {
		return errorcode.ErrWebhookContinue
	})
	if !errors.Is(err, errorcode.ErrWebhookContinue) {
		t.Fatalf("enabled err=%v", err)
	}
}

func TestQueueFull(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	q := NewMemoryQueue(1, 1)
	defer q.Stop()

	block := func() {
		once.Do(func() { close(started) })
		<-release
	}
	if err := q.Push(block); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := q.Push(func() {}); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(func() {}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("full err=%v", err)
	}
	close(release)
}
