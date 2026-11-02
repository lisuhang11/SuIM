package logic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/webhook"
)

func TestFriendAgreeWebhookFields(t *testing.T) {
	bodies := make(chan map[string]any, 2)
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode webhook: %v", err)
		}
		bodies <- body
		_ = json.NewEncoder(w).Encode(webhook.CommonCallbackResp{})
	}))
	t.Cleanup(httpSrv.Close)
	hooks := webhook.NewWebhookClient(httpSrv.URL)
	t.Cleanup(hooks.Close)
	srv := New(nil, nil, Hooks{
		Client:               hooks,
		BeforeAddFriendAgree: webhook.BeforeConfig{Enable: true, Timeout: 2},
		AfterAddFriendAgree:  webhook.AfterConfig{Enable: true, Timeout: 2},
	})

	if err := srv.beforeAddFriendAgree(context.Background(), "from", "to", "ok", 1); err != nil {
		t.Fatal(err)
	}
	srv.afterAddFriendAgree(context.Background(), "from", "to", "ok", 1)
	for i := 0; i < 2; i++ {
		select {
		case body := <-bodies:
			if body["fromUserID"] != "from" || body["toUserID"] != "to" || body["handleMsg"] != "ok" || body["handleResult"] != float64(1) {
				t.Fatalf("agree webhook %+v", body)
			}
			if _, ok := body["blackUserID"]; ok {
				t.Fatalf("old to field %+v", body)
			}
			if _, ok := body["HandleResult"]; ok {
				t.Fatalf("old result field %+v", body)
			}
			if _, ok := body["HandleMsg"]; ok {
				t.Fatalf("old message field %+v", body)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("agree webhook did not arrive")
		}
	}
}
