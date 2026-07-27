// Package main 提供 group gRPC 服务的入口，负责初始化群组管理、成员和入群请求等模块并启动服务。
// 同时连接 user 服务用于用户存在性校验。
package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"group/internal/client"
	"group/internal/config"
	"group/internal/database"
	"group/internal/handler"
	"group/internal/middleware"
	"group/internal/repository"
	"group/internal/service"

	pb "SuIM/proto/grouppb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// main 是服务入口，依次初始化配置、数据库、user 服务客户端、仓库、服务、gRPC 服务器并开始监听。
func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	db := database.MustOpen(cfg)

	// 连接 user 服务进行用户存在性校验（非阻塞连接）。
	userConn, err := grpc.NewClient(cfg.UserServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(fmt.Sprintf("failed to connect to user service: %v", err))
	}
	defer userConn.Close()
	userVerifier := client.NewUserVerifier(userConn)

	// 组合根：将按功能聚合的仓库和 user 校验器注入到服务层。
	groupRepo := repository.NewGroupRepository(db)
	groupSvc := service.NewGroupService(groupRepo, userVerifier)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor()),
	)
	pb.RegisterGroupServiceServer(grpcSvr, handler.NewGroupHandler(groupSvc))
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
