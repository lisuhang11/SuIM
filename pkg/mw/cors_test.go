package mw

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorsPreflight(t *testing.T) {
	called := false
	h := CorsHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodOptions, "/user/login", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if called {
		t.Fatal("preflight reached the handler")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if rr.Body.String() != "\"Options Request!\"\n" {
		t.Fatalf("body %q", rr.Body.String())
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("origin %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "*" {
		t.Fatalf("methods %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got != "*" {
		t.Fatalf("headers %q", got)
	}
	if got := rr.Header().Get("Access-Control-Max-Age"); got != "172800" {
		t.Fatalf("max age %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "false" {
		t.Fatalf("credentials %q", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q", got)
	}
}

func TestCorsPassesRequest(t *testing.T) {
	h := CorsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/user/login", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("origin %q", got)
	}
}

func TestCorsLeavesWebsocketContentType(t *testing.T) {
	h := CorsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := w.Header().Get("Content-Type"); got != "" {
			t.Fatalf("content type %q", got)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("origin %q", got)
		}
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Connection", "keep-alive, Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusSwitchingProtocols {
		t.Fatalf("status %d", rr.Code)
	}
}
