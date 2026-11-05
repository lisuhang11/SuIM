package mw

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGzipCompressesJSON(t *testing.T) {
	const body = `{"status":"ok"}`
	h := GzipHandler(gzipDefault, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("encoding %q", got)
	}
	if got := rr.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("vary %q", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type %q", got)
	}
	got := gunzip(t, rr.Body)
	if got != body {
		t.Fatalf("body %q", got)
	}
}

func TestGzipLevels(t *testing.T) {
	for _, level := range []int{gzipDefault, gzipBest, gzipFast} {
		h := GzipHandler(level, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		req := httptest.NewRequest(http.MethodPost, "/user/get", nil)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if gunzip(t, rr.Body) != "hello" {
			t.Fatalf("level %d body %q", level, rr.Body.String())
		}
	}
}

func TestGzipSkipsWithoutAcceptEncoding(t *testing.T) {
	h := GzipHandler(gzipDefault, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "plain")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != "plain" {
		t.Fatalf("headers %v body %q", rr.Header(), rr.Body.String())
	}
}

func TestGzipDisabled(t *testing.T) {
	for _, level := range []int{gzipOff, 9} {
		h := GzipHandler(level, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "plain")
		}))
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != "plain" {
			t.Fatalf("level %d headers %v body %q", level, rr.Header(), rr.Body.String())
		}
	}
}

func TestGzipSkipsImagesUpgradeAndEvents(t *testing.T) {
	h := GzipHandler(gzipDefault, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "raw")
	}))
	cases := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/a.png", nil),
		httptest.NewRequest(http.MethodGet, "/a.gif", nil),
		httptest.NewRequest(http.MethodGet, "/a.jpeg", nil),
		httptest.NewRequest(http.MethodGet, "/a.jpg", nil),
	}
	upgrade := httptest.NewRequest(http.MethodGet, "/", nil)
	upgrade.Header.Set("Connection", "Upgrade")
	cases = append(cases, upgrade)
	events := httptest.NewRequest(http.MethodGet, "/events", nil)
	events.Header.Set("Accept", "text/event-stream")
	cases = append(cases, events)
	for _, req := range cases {
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != "raw" {
			t.Fatalf("%s %s encoding %q body %q", req.Method, req.URL.Path, rr.Header().Get("Content-Encoding"), rr.Body.String())
		}
	}
}

func gunzip(t *testing.T, r io.Reader) string {
	t.Helper()
	zr, err := gzip.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
