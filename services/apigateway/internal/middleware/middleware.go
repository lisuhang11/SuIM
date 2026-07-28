// Package middleware 提供 API 网关中间件。
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ---------- Request ID ----------

const headerRequestID = "X-Request-ID"

// RequestID 为每个请求注入唯一标识，并写入响应头。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(headerRequestID)
		if id == "" {
			id = uuid.New().String()
		}
		c.Set("request_id", id)
		c.Header(headerRequestID, id)
		c.Next()
	}
}

// GetRequestID 从 gin.Context 中提取请求 ID。
func GetRequestID(c *gin.Context) string {
	if id, ok := c.Get("request_id"); ok {
		return id.(string)
	}
	return ""
}

// ---------- Recovery ----------

// Recovery 捕获 panic 并返回 500，同时打印堆栈。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var brokenPipe bool
				if ne, ok := r.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") {
							brokenPipe = true
						}
					}
				}
				if brokenPipe {
					c.Abort()
					return
				}
				reqID := GetRequestID(c)
				slog.Error("[gateway] panic recovered",
					"request_id", reqID,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "internal server error",
				})
			}
		}()
		c.Next()
	}
}

// ---------- Structured Logging ----------

// Logging 结构化记录每个请求的耗时、状态码与路径。
func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start).Milliseconds()
		status := c.Writer.Status()
		reqID := GetRequestID(c)

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		slog.LogAttrs(c.Request.Context(), level, "[gateway] request",
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("latency_ms", latency),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}

// ---------- Prometheus Metrics ----------

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total HTTP requests processed by the gateway.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_http_request_duration_seconds",
			Help:    "HTTP request latency histogram.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	httpRequestInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gateway_http_requests_in_flight",
			Help: "Number of in-flight HTTP requests.",
		},
	)
	grpcRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_grpc_request_duration_seconds",
			Help:    "gRPC backend call latency histogram.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)
	grpcRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_grpc_requests_total",
			Help: "Total gRPC requests forwarded by the gateway.",
		},
		[]string{"service", "method", "status"},
	)
)

var metricsOnce sync.Once

// MetricsInit 注册 Prometheus 指标（幂等）。
func MetricsInit() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(
			httpRequestsTotal,
			httpRequestDuration,
			httpRequestInFlight,
			grpcRequestDuration,
			grpcRequestTotal,
		)
	})
}

// Metrics 记录 HTTP 层的请求数、耗时与并发数。
func Metrics() gin.HandlerFunc {
	MetricsInit()
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), http.StatusText(status)).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(time.Since(start).Seconds())
	}
}

// MetricsInFlight 记录正在处理的请求数。
func MetricsInFlight() gin.HandlerFunc {
	MetricsInit()
	return func(c *gin.Context) {
		httpRequestInFlight.Inc()
		defer httpRequestInFlight.Dec()
		c.Next()
	}
}

// MetricsHandler 返回 Prometheus metrics HTTP handler（gin 兼容）。
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// MetricsHTTPHandler 返回纯 net/http 兼容的 Prometheus handler（供独立 metrics 服务使用）。
func MetricsHTTPHandler() http.Handler {
	return promhttp.Handler()
}

// RecordGRPCCall 记录 gRPC 调用指标（在 handler 层调用）。
func RecordGRPCCall(service, method string, err error, start time.Time) {
	dur := time.Since(start).Seconds()
	status := "ok"
	if err != nil {
		status = "error"
	}
	MetricsInit()
	grpcRequestDuration.WithLabelValues(service, method).Observe(dur)
	grpcRequestTotal.WithLabelValues(service, method, status).Inc()
}

// ---------- Rate Limiter ----------

// RateLimiter 基于令牌桶的 IP 级别限流中间件。
type RateLimiter struct {
	clients map[string]*tokenBucket
	mu      sync.Mutex
	rate    float64
	burst   int
	enabled bool
}

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter 创建一个速率限制器。
func NewRateLimiter(rate float64, burst int, enabled bool) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
		enabled: enabled,
	}
}

// UpdateLimits 动态更新限流参数供热重载使用。
func (rl *RateLimiter) UpdateLimits(rate float64, burst int, enabled bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rate = rate
	rl.burst = burst
	rl.enabled = enabled
	rl.clients = make(map[string]*tokenBucket) // 重置令牌桶
}

// Handler 返回 gin 限流中间件。
func (rl *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.enabled {
			c.Next()
			return
		}

		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many requests",
			})
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	tb, ok := rl.clients[key]
	now := time.Now()
	if !ok {
		rl.clients[key] = &tokenBucket{
			tokens:   float64(rl.burst) - 1,
			lastTime: now,
		}
		// 定期清理老旧客户端（概率 1/1000）。
		if len(rl.clients)%1000 == 0 {
			rl.reapStale(now)
		}
		return true
	}

	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens += elapsed * rl.rate
	if tb.tokens > float64(rl.burst) {
		tb.tokens = float64(rl.burst)
	}
	tb.lastTime = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) reapStale(now time.Time) {
	for k, tb := range rl.clients {
		if now.Sub(tb.lastTime) > 5*time.Minute {
			delete(rl.clients, k)
		}
	}
}

// ---------- CORS ----------

// CORS 返回跨域中间件。
func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowOrigin := ""
		for _, o := range origins {
			if o == "*" || o == origin {
				allowOrigin = origin
				if o == "*" {
					allowOrigin = "*"
				}
				break
			}
		}
		if origin == "" {
			allowOrigin = "*"
		}

		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ---------- Compression ----------

// CompressBody 对 JSON > 1KB 的响应体启用 gzip（由 gin-contrib/gzip 驱动）。
// 这里提供一个轻量的 body reader 工具，用于 request body 的重复读取。
func CompressBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 缓存请求体以便重复读取（限流/日志等场景）
		if c.Request.Body != nil {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
				c.Set("raw_body", body)
			}
		}
		c.Next()
	}
}

// ---------- Body Logger (Debug) ----------

// BodyLogger 调试用：打印请求体和响应体（仅在 debug 模式启用）。
func BodyLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if slog.Default().Enabled(c.Request.Context(), slog.LevelDebug) {
			if raw, ok := c.Get("raw_body"); ok {
				var pretty bytes.Buffer
				if err := json.Indent(&pretty, raw.([]byte), "", "  "); err == nil {
					slog.Debug("[gateway] request body", "request_id", GetRequestID(c), "body", pretty.String())
				}
			}
		}
		c.Next()
	}
}
