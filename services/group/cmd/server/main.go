// Package main 提供 group gRPC 服务的入口，负责初始化群组管理、成员和入群请求等模块并启动服务。
// 通过 etcd 服务发现连接 user 服务进行用户存在性校验。
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"group/internal/cache"
	"group/internal/client"
	"group/internal/config"
	"group/internal/database"
	"group/internal/handler"
	"group/internal/middleware"
	"group/internal/repository"
	"group/internal/service"

	"SuIM/pkg/discovery"
	filePB "SuIM/proto/filepb"
	pb "SuIM/proto/grouppb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// main 是服务入口，依次初始化配置、注册 etcd、连接数据库、通过 etcd 发现 user 服务、启动 gRPC 服务器。
func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "etc/group.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	// 注册到 etcd 服务发现。
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("group", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(fmt.Sprintf("failed to register with etcd: %v", err))
	}
	defer registry.Deregister()

	db := database.MustOpen(cfg)

	// 通过 etcd 服务发现连接 user 服务进行用户存在性校验。
	userConn, err := grpc.NewClient(
		discovery.TargetURL("user"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to user service via etcd: %v", err))
	}
	defer userConn.Close()
	userVerifier := client.NewUserVerifier(userConn)
	conversationConn, err := grpc.NewClient(
		discovery.TargetURL("conversation"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to conversation service: %v", err))
	}
	defer conversationConn.Close()
	messageConn, err := grpc.NewClient(
		discovery.TargetURL("message"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to message service: %v", err))
	}
	defer messageConn.Close()
	eventPublisher := client.NewGroupEventPublisher(conversationConn, messageConn)
	fileConn, err := grpc.NewClient(discovery.TargetURL("file"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		panic(fmt.Sprintf("failed to connect to file service: %v", err))
	}
	defer fileConn.Close()
	fileClient := filePB.NewFileServiceClient(fileConn)

	rdb, err := cache.NewRedisClient(cache.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		slog.Warn("redis unavailable, group info cache disabled", "error", err)
	}
	groupCache := cache.NewGroupInfoCache(rdb, cfg.GroupInfoCacheTTL)

	// 组合根：将按功能聚合的仓库和 user 校验器注入到服务层。
	groupRepo := repository.NewGroupRepository(db)
	groupSvc := service.NewGroupService(groupRepo, userVerifier, groupCache, eventPublisher)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor(userVerifier)),
	)
	pb.RegisterGroupServiceServer(grpcSvr, handler.NewGroupHandler(groupSvc, fileClient))
	reflection.Register(grpcSvr) // 启用 grpcurl 等调试工具

	lis, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to listen on %s: %v", cfg.ServerAddr, err))
	}

	slog.Info("gRPC server starting", "addr", cfg.ServerAddr)
	if err := grpcSvr.Serve(lis); err != nil {
		panic(fmt.Sprintf("gRPC server exited: %v", err))
	}
}
