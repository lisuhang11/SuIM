// Package prommetrics provides Prometheus counters and gauges for SuIM.
// Each service owns an independent registry and exposes its own /metrics
// endpoint, mirroring the per-service model used by OpenIM.
package prommetrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const MetricsPath = "/metrics"

var baseCollectors = []prometheus.Collector{
	collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	collectors.NewGoCollector(),
}

// 通用指标：所有服务都会注册。
var (
	RPCCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rpc_count",
		Help: "Total number of RPC calls.",
	}, []string{"name", "path", "code"})
	HTTPCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_count",
		Help: "Total number of HTTP calls.",
	}, []string{"path", "method", "status"})
	APICounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "api_count",
		Help: "Total number of API calls.",
	}, []string{"path", "method", "code"})
)

// 业务指标：按服务划分，由 ForService 选择性注册。
var (
	// user / auth
	UserRegisterCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "user_register_total", Help: "The number of user registrations."})
	UserLoginCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "user_login_total", Help: "The number of user logins."})

	// msg 服务：单聊/群聊消息处理成败
	SingleChatMsgSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "single_chat_msg_process_success_total", Help: "Successful single chat messages."})
	SingleChatMsgFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "single_chat_msg_process_failed_total", Help: "Failed single chat messages."})
	GroupChatMsgSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "group_chat_msg_process_success_total", Help: "Successful group chat messages."})
	GroupChatMsgFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "group_chat_msg_process_failed_total", Help: "Failed group chat messages."})

	// msgtransfer：消息写 Redis / MySQL 成败、seq 设置失败
	MsgInsertRedisSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "msg_insert_redis_success_total", Help: "Successful message inserts to Redis."})
	MsgInsertRedisFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "msg_insert_redis_failed_total", Help: "Failed message inserts to Redis."})
	MsgInsertMySQLSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "msg_insert_mysql_success_total", Help: "Successful message inserts to MySQL."})
	MsgInsertMySQLFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "msg_insert_mysql_failed_total", Help: "Failed message inserts to MySQL."})
	SeqSetFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "seq_set_failed_total", Help: "The number of failed seq sets."})

	// msggateway：在线用户数
	OnlineUserGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "online_user_num", Help: "The number of online users."})

	// push：离线推送失败、慢推送
	MsgOfflinePushFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "msg_offline_push_failed_total", Help: "Failed offline pushes."})
	MsgLongTimePush = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "msg_long_time_push_total", Help: "Messages whose push took more than 10 seconds."})
)

// serviceCollectors 把业务指标按服务分组。OpenIM 也采用相同的分发方式：
// 每个服务的 /metrics 只暴露自身相关的指标，避免全量指标污染。
var serviceCollectors = map[string][]prometheus.Collector{
	"user":        {UserRegisterCounter},
	"msggateway":  {OnlineUserGauge},
	"msg":         {SingleChatMsgSuccess, SingleChatMsgFailed, GroupChatMsgSuccess, GroupChatMsgFailed},
	"msgtransfer": {MsgInsertRedisSuccess, MsgInsertRedisFailed, MsgInsertMySQLSuccess, MsgInsertMySQLFailed, SeqSetFailed},
	"push":        {MsgOfflinePushFailed, MsgLongTimePush},
	"apigateway":  {UserLoginCounter, APICounter, HTTPCounter},
}

// ForService 返回一个只含该服务指标的 registry。通用指标（rpc/process/go）
// 已包含在内；未列出的服务只暴露通用指标。
func ForService(name string) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	for _, c := range baseCollectors {
		reg.MustRegister(c)
	}
	reg.MustRegister(RPCCounter)
	for _, c := range serviceCollectors[name] {
		reg.MustRegister(c)
	}
	return reg
}

func codeString(code int) string { return strconv.Itoa(code) }

func RPCCall(name, path string, code int) {
	RPCCounter.WithLabelValues(name, path, codeString(code)).Inc()
}

func HTTPCall(path, method string, status int) {
	HTTPCounter.WithLabelValues(path, method, codeString(status)).Inc()
}

func APICall(path, method string, code int) {
	APICounter.WithLabelValues(path, method, codeString(code)).Inc()
}

// Server 是一个自管理的 /metrics HTTP 服务。ctx 取消时自动优雅关闭，
// 监听错误只记日志，不写入业务 errCh，故不需改动各服务 errCh 容量。
type Server struct {
	httpServer *http.Server
	listener   net.Listener
}

// Start 在 listenIP:port 启动 metrics 端点，返回 *Server 供显式关闭（可选）。
// port<=0 时不启动，返回 nil。
func Start(ctx context.Context, serviceName, listenIP string, port int) *Server {
	if port <= 0 {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", listenIP, port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("listen metrics", "addr", addr, "err", err)
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle(MetricsPath, promhttp.HandlerFor(ForService(serviceName), promhttp.HandlerOpts{}))
	srv := &Server{
		listener: lis,
		httpServer: &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.httpServer.Shutdown(shutdownCtx)
	}()
	go func() {
		slog.Info("metrics listening", "addr", addr, "path", MetricsPath)
		if err := srv.httpServer.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics server stopped", "addr", addr, "err", err)
		}
	}()
	return srv
}

// Shutdown 显式关闭 metrics 服务，通常由 Start 的 ctx 监听自动完成。
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
