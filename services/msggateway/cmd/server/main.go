// Package main 是 msg gateway 服务入口，启动 WebSocket 服务端 + gRPC 服务端。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	pb "SuIM/proto/msggatewaypb"
	"SuIM/pkg/discovery"
	"msggateway/internal/config"
	"msggateway/internal/connmgr"
	"msggateway/internal/handler"
	"msggateway/internal/ws"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "etc/msggateway.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	// 注册到 etcd 服务发现（msggateway 注册 gRPC 地址供 push 等服务发现）。
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("msggateway", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(fmt.Sprintf("failed to register with etcd: %v", err))
	}
	defer registry.Deregister()

	// -------- 连接管理器 --------
	connManager := connmgr.New(cfg.MaxConnPerUser)

	// -------- WebSocket 服务端 --------
	wsServer := ws.NewServer(ws.ServerConfig{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PingInterval: cfg.PingInterval,
	}, connManager)

	// 可选：设置上线/下线日志回调。
	wsServer.SetOnlineChangeHook(func(userID string, platformID int32, online bool) {
		action := "offline"
		if online {
			action = "online"
		}
		slog.Info("user status changed",
			"user_id", userID,
			"platform_id", platformID,
			"status", action,
			"online_users", connManager.OnlineUsers(),
			"total_conns", connManager.TotalConns(),
		)
	})

	// -------- gRPC 服务 --------
	grpcSvr := grpc.NewServer()
	msgGwHandler := handler.NewMsgGatewayHandler(wsServer)
	pb.RegisterMsgGatewayServer(grpcSvr, msgGwHandler)
	reflection.Register(grpcSvr)

	// -------- 启动 gRPC --------
	grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to listen gRPC on %s: %v", cfg.GRPCAddr, err))
	}
	go func() {
		slog.Info("gRPC server starting", "addr", cfg.GRPCAddr)
		if err := grpcSvr.Serve(grpcLis); err != nil {
			slog.Error("gRPC server error", "error", err)
		}
	}()

	// -------- 启动 WebSocket (HTTP) 服务 --------
	wsMux := http.NewServeMux()
	// /ws 端点：客户端 WebSocket 连接。
	wsMux.HandleFunc("/ws", wsServer.ServeHTTP)
	// /health 健康检查。
	wsMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","online_users":%d,"total_conns":%d}`,
			connManager.OnlineUsers(), connManager.TotalConns())
	})

	wsHTTP := &http.Server{
		Addr:    cfg.WSAddr,
		Handler: wsMux,
	}
	go func() {
		slog.Info("WebSocket server starting", "addr", cfg.WSAddr)
		if err := wsHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("WebSocket server error", "error", err)
		}
	}()

	// -------- 启动 Metrics HTTP 服务 --------
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP msggateway_online_users Current online users\n")
		fmt.Fprintf(w, "# TYPE msggateway_online_users gauge\n")
		fmt.Fprintf(w, "msggateway_online_users %d\n", connManager.OnlineUsers())
		fmt.Fprintf(w, "# HELP msggateway_total_conns Total WebSocket connections\n")
		fmt.Fprintf(w, "# TYPE msggateway_total_conns gauge\n")
		fmt.Fprintf(w, "msggateway_total_conns %d\n", connManager.TotalConns())
	})

	metricsHTTP := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metricsMux,
	}
	go func() {
		slog.Info("metrics server starting", "addr", cfg.MetricsAddr)
		if err := metricsHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server error", "error", err)
		}
	}()

	slog.Info("msg gateway started",
		"ws_addr", cfg.WSAddr,
		"grpc_addr", cfg.GRPCAddr,
		"metrics_addr", cfg.MetricsAddr,
	)

	// -------- 优雅关闭 --------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
	defer cancel()

	// 1. 关闭 WebSocket HTTP。
	if err := wsHTTP.Shutdown(shutdownCtx); err != nil {
		slog.Error("ws server shutdown error", "error", err)
	}
	// 2. 关闭 Metrics HTTP。
	if err := metricsHTTP.Shutdown(shutdownCtx); err != nil {
		slog.Error("metrics server shutdown error", "error", err)
	}
	// 3. 关闭 gRPC。
	grpcSvr.GracefulStop()

	slog.Info("shutdown complete")
}
