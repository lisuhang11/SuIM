// Package main 提供 relation gRPC 服务的入口，负责初始化好友关系、好友请求和拉黑等模块并启动服务。
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"relation/internal/config"
	"relation/internal/database"
	"relation/internal/handler"
	"relation/internal/middleware"
	"relation/internal/repository"
	"relation/internal/service"

	pb "SuIM/proto/relationpb"
	"SuIM/pkg/discovery"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "etc/relation.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	// 注册到 etcd 服务发现。
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("relation", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(fmt.Sprintf("failed to register with etcd: %v", err))
	}
	defer registry.Deregister()

	db := database.MustOpen(cfg)

	// 组合根：将按功能聚合的仓库注入到服务层。
	relationRepo := repository.NewRelationRepository(db)
	relationSvc := service.NewRelationService(relationRepo)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor()),
	)
	pb.RegisterRelationServiceServer(grpcSvr, handler.NewRelationHandler(relationSvc))
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
