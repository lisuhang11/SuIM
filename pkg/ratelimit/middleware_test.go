package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

type stubLimiter struct {
	allowErr error
	stat     Stat
	allowed  int
	done     int
}

func (s *stubLimiter) Allow() (DoneFunc, error) {
	if s.allowErr != nil {
		return nil, s.allowErr
	}
	s.allowed++
	return func(DoneInfo) { s.done++ }, nil
}

func (s *stubLimiter) Stat() Stat { return s.stat }

func TestHTTPMiddlewareRejects(t *testing.T) {
	limiter := &stubLimiter{allowErr: ErrLimitExceeded, stat: Stat{CPU: 900, MaxInFlight: 7}}
	called := false
	h := HTTPMiddlewareWithLimiter("test", limiter)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/blocked", nil))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if called {
		t.Fatal("next handler must not run when limited")
	}
	if rec.Header().Get("X-RateLimit-Policy") != "BBR" {
		t.Fatalf("policy header = %q", rec.Header().Get("X-RateLimit-Policy"))
	}
	if rec.Header().Get("X-RateLimit-Limit") != "7" {
		t.Fatalf("limit header = %q", rec.Header().Get("X-RateLimit-Limit"))
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is missing")
	}
}

func TestHTTPMiddlewareAllowsAndCloses(t *testing.T) {
	limiter := &stubLimiter{stat: Stat{CPU: 100}}
	called := false
	h := HTTPMiddlewareWithLimiter("test", limiter)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if rec.Code != http.StatusNoContent || !called {
		t.Fatalf("status = %d, called = %v", rec.Code, called)
	}
	if limiter.allowed != 1 || limiter.done != 1 {
		t.Fatalf("allowed = %d, done = %d", limiter.allowed, limiter.done)
	}
}

func TestUnaryInterceptorRejects(t *testing.T) {
	limiter := &stubLimiter{allowErr: ErrLimitExceeded}
	called := false
	interceptor := UnaryServerInterceptorWithLimiter("test", limiter)
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(context.Context, any) (any, error) {
			called = true
			return "ok", nil
		})
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("code = %v, want ResourceExhausted", status.Code(err))
	}
	if code, ok := errorcode.CodeOf(err); !ok || code != errorcode.RateLimited {
		t.Fatalf("business code = %d, ok = %v", code, ok)
	}
	if called {
		t.Fatal("handler must not run when limited")
	}
}

func TestUnaryInterceptorDoneOnError(t *testing.T) {
	limiter := &stubLimiter{}
	interceptor := UnaryServerInterceptorWithLimiter("test", limiter)
	_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(context.Context, any) (any, error) {
			return nil, context.Canceled
		})
	if limiter.done != 1 {
		t.Fatalf("done = %d, want 1", limiter.done)
	}
}
