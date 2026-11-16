package ratelimit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"google.golang.org/grpc"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/prommetrics"
	"github.com/lisuhang11/SuIM/pkg/ratelimit/bbr"
)

// New builds a limiter from validated service config. Disabled config returns nil.
func New(cfg *config.RateLimiter) Limiter {
	if cfg == nil || !cfg.Enable {
		return nil
	}
	return bbr.NewBBRLimiter(
		bbr.WithWindow(cfg.Window),
		bbr.WithBucket(cfg.Bucket),
		bbr.WithCPUThreshold(cfg.CPUThreshold),
	)
}

// UnaryServerInterceptor sheds RPCs before they reach the business handler.
func UnaryServerInterceptor(serviceName string, cfg *config.RateLimiter) grpc.UnaryServerInterceptor {
	return UnaryServerInterceptorWithLimiter(serviceName, New(cfg))
}

// UnaryServerInterceptorWithLimiter is the injectable form used by tests.
func UnaryServerInterceptorWithLimiter(serviceName string, limiter Limiter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if limiter == nil {
			return handler(ctx, req)
		}
		done, err := limiter.Allow()
		if err != nil {
			prommetrics.RateLimitRejected(serviceName, "grpc")
			slog.Warn("grpc rate limited", "service", serviceName, "method", info.FullMethod, "err", err)
			return nil, errorcode.ErrRateLimited.WithMsg("rpc request rate limit exceeded, please try again later")
		}
		defer func() { done(DoneInfo{Err: err}) }()
		return handler(ctx, req)
	}
}

// StreamServerInterceptor applies the same gate once when a stream starts.
func StreamServerInterceptor(serviceName string, cfg *config.RateLimiter) grpc.StreamServerInterceptor {
	return StreamServerInterceptorWithLimiter(serviceName, New(cfg))
}

// StreamServerInterceptorWithLimiter is the injectable form used by tests.
func StreamServerInterceptorWithLimiter(serviceName string, limiter Limiter) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		if limiter == nil {
			return handler(srv, ss)
		}
		done, err := limiter.Allow()
		if err != nil {
			prommetrics.RateLimitRejected(serviceName, "stream")
			slog.Warn("stream rate limited", "service", serviceName, "method", info.FullMethod, "err", err)
			return errorcode.ErrRateLimited.WithMsg("rpc stream rate limit exceeded, please try again later")
		}
		defer func() { done(DoneInfo{Err: err}) }()
		return handler(srv, ss)
	}
}

// HTTPMiddleware sheds HTTP requests and exposes BBR state in response headers,
// matching the header names used by OpenIM's API rate limiter.
func HTTPMiddleware(serviceName string, cfg *config.RateLimiter) func(http.Handler) http.Handler {
	return HTTPMiddlewareWithLimiter(serviceName, New(cfg))
}

// HTTPMiddlewareWithLimiter is the injectable form used by tests.
func HTTPMiddlewareWithLimiter(serviceName string, limiter Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if limiter == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stat := limiter.Stat()
			w.Header().Set("X-BBR-CPU", strconv.FormatInt(stat.CPU, 10))
			w.Header().Set("X-BBR-MinRT", strconv.FormatInt(stat.MinRT, 10))
			w.Header().Set("X-BBR-MaxPass", strconv.FormatInt(stat.MaxPass, 10))
			w.Header().Set("X-BBR-MaxInFlight", strconv.FormatInt(stat.MaxInFlight, 10))
			w.Header().Set("X-BBR-InFlight", strconv.FormatInt(stat.InFlight, 10))

			done, err := limiter.Allow()
			if err != nil {
				prommetrics.RateLimitRejected(serviceName, "http")
				slog.Warn("http rate limited", "service", serviceName, "path", r.URL.Path, "err", err)
				w.Header().Set("X-RateLimit-Policy", "BBR")
				w.Header().Set("Retry-After", retryAfter(stat))
				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(stat.MaxInFlight, 10))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(struct {
					Code int32  `json:"code"`
					Msg  string `json:"msg"`
				}{
					Code: int32(errorcode.RateLimited),
					Msg:  "too many requests, please try again later",
				})
				return
			}
			defer done(DoneInfo{})
			next.ServeHTTP(w, r)
		})
	}
}

// retryAfter uses the 0-1000 CPU scale directly. It avoids OpenIM's
// status.CPU/status.CPU self-division, which always evaluated to 1.
func retryAfter(stat Stat) string {
	switch {
	case stat.CPU >= 950:
		return "5"
	case stat.CPU >= 900:
		return "3"
	case stat.CPU >= 850:
		return "2"
	default:
		return "1"
	}
}
