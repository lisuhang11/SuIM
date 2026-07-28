// Package main 是 API 网关的入口，负责装配依赖、连接 etcd、注册服务、启动 HTTP 服务并响应热重载信号。
// 后端服务地址通过 etcd 服务发现自动解析，不再使用静态 IP 直连。
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"apigateway/internal/config"
	"apigateway/internal/etcd"
	"apigateway/internal/grpc"
	"apigateway/internal/middleware"
	"apigateway/internal/router"

	"SuIM/pkg/discovery"
)

func main() {
	// 初始化结构化日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// 1. 从 YAML 配置文件加载配置。
	configPath := flag.String("config", "etc/apigateway.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	// 2. 注册到 etcd 服务发现。
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("apigateway", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		slog.Error("[gateway] failed to register with etcd", "error", err)
		os.Exit(1)
	}
	defer registry.Deregister()

	// 3. 初始化 Prometheus 指标。
	middleware.MetricsInit()

	// 4. 建立 gRPC 客户端连接池（通过 etcd 服务发现解析后端地址）。
	clients, err := grpc.NewClients(cfg)
	if err != nil {
		slog.Error("[gateway] failed to create gRPC clients", "error", err)
		os.Exit(1)
	}
	defer clients.Close()

	// 5. 创建可热更新的中间件实例。
	authMW := middleware.NewAuthMiddleware(middleware.AuthConfig{
		JWTSecret:   cfg.JWTSecret,
		CacheTTL:    5 * time.Minute,
		PublicPaths: []string{
			"/api/v1/users/register",
			"/api/v1/users/login",
			"/api/v1/users/validate-token",
			"/api/v1/users/refresh-token",
			"/health",
			"/metrics",
		},
	})

	rateLimiter := middleware.NewRateLimiter(
		cfg.RateLimit.Rate,
		cfg.RateLimit.Burst,
		cfg.RateLimit.Enabled,
	)

	// 6. 构建 gin 引擎。
	engine := router.NewEngine(cfg, clients, authMW, rateLimiter)

	// 7. 启动 metrics HTTP 服务（独立端口）。
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", middleware.MetricsHTTPHandler())
	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metricsMux,
	}
	go func() {
		slog.Info("[gateway] metrics server starting", "addr", cfg.MetricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("[gateway] metrics server error", "error", err)
		}
	}()

	// 8. 启动主 HTTP 服务。
	httpServer := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      engine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("[gateway] HTTP server starting", "addr", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("[gateway] HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// 9. 连接 etcd 并监听远程配置变更（JWT 密钥、限流参数等）。
	//    注意：后端服务地址通过 discovery 包自动解析，不在此处管理。
	if len(cfg.EtcdEndpoints) == 0 {
		slog.Info("[gateway] etcd endpoints not configured, skipping etcd config watcher (static config)")
	} else {
		etcdCfg := cfg.ToEtcdConfig()
		// 使用静态默认配置作为 fallback，不依赖 Backends 字段。
		fallbackCfg := config.Default()
		etcdWatcher, err := etcd.NewConfigWatcher(etcdCfg, fallbackCfg, func(newCfg *config.GatewayConfig, err error) {
			if err != nil {
				slog.Error("[gateway] config reload failed", "error", err)
				return
			}
			slog.Info("[gateway] applying new config from etcd")

			// 热更新 JWT 密钥。
			authMW.UpdateSecret(newCfg.JWTSecret)

			// 热更新限流参数。
			rateLimiter.UpdateLimits(
				newCfg.RateLimit.Rate,
				newCfg.RateLimit.Burst,
				newCfg.RateLimit.Enabled,
			)

			// gRPC 后端连接由 etcd resolver 自动管理，无需热切换。
		})
		if err != nil {
			slog.Warn("[gateway] etcd watcher init failed, using static config", "error", err)
		} else {
			defer etcdWatcher.Shutdown()
		}
	}

	// 10. 等待退出信号。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("[gateway] received signal, shutting down gracefully", "signal", sig.String())

	// 11. 优雅关闭。
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 停止 HTTP 服务。
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("[gateway] HTTP server shutdown error", "error", err)
	}
	// 停止 metrics 服务。
	metricsServer.Shutdown(shutdownCtx)

	slog.Info("[gateway] shutdown complete")
}
