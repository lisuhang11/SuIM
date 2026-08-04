// Package main 提供 relation gRPC 服务的入口，负责初始化好友关系、好友请求和拉黑等模块并启动服务。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"relation/internal/client"
	"relation/internal/config"
	"relation/internal/database"
	"relation/internal/handler"
	"relation/internal/middleware"
	"relation/internal/notification"
	"relation/internal/repository"
	"relation/internal/service"

	"SuIM/pkg/discovery"
	pb "SuIM/proto/relationpb"

	messagepb "SuIM/proto/messagepb"
	"SuIM/proto/msggatewaypb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

	// 创建 msggateway gRPC 客户端，通过 etcd 服务发现连接。
	msgGatewayConn, err := grpc.Dial(
		discovery.TargetURL("msggateway"),
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect msggateway: %v", err))
	}
	defer msgGatewayConn.Close()

	msgGatewayClient := msggatewaypb.NewMsgGatewayClient(msgGatewayConn)

	// pushMsg 封装一次调用，后续所有通知复用同一个 client。
	pushMsg := func(ctx context.Context, recvID string, msg *messagepb.MsgData) error {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = metadata.NewOutgoingContext(ctx, md.Copy())
		_, err := msgGatewayClient.OnlinePushMsg(ctx, &msggatewaypb.OnlinePushMsgReq{
			MsgData:      msg,
			PushToUserId: recvID,
		})
		return err
	}

	// 装配 FriendNotificationSender（嵌入内核 + 好友领域方法）。
	friendNotifier := notification.NewFriendNotificationSender(pushMsg)

	// 组合根：将仓库和通知发送器注入到服务层。
	relationRepo := repository.NewRelationRepository(db)
	relationSvc := service.NewRelationService(relationRepo, friendNotifier)

	userConn, err := grpc.NewClient(
		discovery.TargetURL("user"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to user service: %v", err))
	}
	defer userConn.Close()
	authenticator := client.NewUserAuthenticator(userConn)
	profiles := client.NewUserProfiles(userConn)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor(authenticator)),
	)
	pb.RegisterRelationServiceServer(grpcSvr, handler.NewRelationHandler(relationSvc, profiles))
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
