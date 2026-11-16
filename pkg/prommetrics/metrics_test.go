package prommetrics

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestForServiceRegistersExpectedCollectors(t *testing.T) {
	reg := ForService("push")
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range families {
		found[f.GetName()] = true
	}
	for _, want := range []string{"go_goroutines", "msg_offline_push_failed_total", "msg_long_time_push_total"} {
		if !found[want] {
			t.Errorf("missing metric %q in push registry", want)
		}
	}
	if found["user_register_total"] {
		t.Error("push registry should not contain user metrics")
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	RPCCounter.Reset()
	interceptor := UnaryServerInterceptor("user")
	info := &grpc.UnaryServerInfo{FullMethod: "/user.UserService/GetUserByID"}
	_, err := interceptor(context.Background(), nil, info,
		func(context.Context, any) (any, error) { return "ok", status.Error(codes.NotFound, "x") })
	if err == nil {
		t.Fatal("expected error")
	}
	families, _ := ForService("user").Gather()
	for _, f := range families {
		if f.GetName() == "rpc_count" {
			for _, m := range f.GetMetric() {
				if m.GetCounter().GetValue() < 1 {
					t.Fatal("rpc_count not incremented")
				}
				return
			}
		}
	}
	t.Fatal("rpc_count not found")
}

func TestHTTPMiddleware(t *testing.T) {
	HTTPCounter.Reset()
	handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/user/register", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	families, _ := ForService("apigateway").Gather()
	for _, f := range families {
		if f.GetName() == "http_count" {
			for _, m := range f.GetMetric() {
				if m.GetCounter().GetValue() >= 1 {
					return
				}
			}
		}
	}
	t.Fatal("http_count not incremented")
}

func TestStartAndServeMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := Start(ctx, "user", "127.0.0.1", 0)
	if srv != nil {
		t.Fatal("port 0 should not start")
	}

	// 用一个临时 listener 端口做端到端探测。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	srv = Start(ctx, "user", "127.0.0.1", port)
	if srv == nil {
		t.Fatal("failed to start metrics server")
	}
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + MetricsPath)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "rpc_count") {
		t.Fatal("/metrics body missing rpc_count")
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
