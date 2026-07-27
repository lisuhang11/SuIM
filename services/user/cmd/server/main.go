// Package main 提供 user gRPC 服务的入口，负责初始化并启动服务。
package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"user/internal/config"
	"user/internal/database"
	"user/internal/handler"
	"user/internal/repository"
	"user/internal/service"

	pb "SuIM/proto/userpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	db := database.MustOpen(cfg)

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewAuthTokenRepository(db)
	userSvc := service.NewUserService(userRepo, tokenRepo, cfg)

	grpcSvr := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcSvr, handler.NewUserHandler(userSvc))
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
