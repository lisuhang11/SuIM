// Package main 是 rtc gRPC 服务的入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"rtc/internal/client"
	"rtc/internal/config"
	"rtc/internal/database"
	"rtc/internal/handler"
	"rtc/internal/middleware"
	"rtc/internal/notification"
	"rtc/internal/repository"
	"rtc/internal/service"

	"SuIM/pkg/discovery"
	messagepb "SuIM/proto/messagepb"
	msggatewaypb "SuIM/proto/msggatewaypb"
	pushpb "SuIM/proto/pushpb"
	pb "SuIM/proto/rtcpb"
	relationpb "SuIM/proto/relationpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
)

func dialService(name string) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		discovery.TargetURL(name),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "etc/rtc.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("rtc", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(fmt.Sprintf("failed to register with etcd: %v", err))
	}
	defer registry.Deregister()

	db := database.MustOpen(context.Background(), cfg)

	msgGatewayConn, err := dialService("msggateway")
	if err != nil {
		panic(fmt.Sprintf("failed to connect msggateway: %v", err))
	}
	defer msgGatewayConn.Close()

	relationConn, err := dialService("relation")
	if err != nil {
		panic(fmt.Sprintf("failed to connect relation: %v", err))
	}
	defer relationConn.Close()

	messageConn, err := dialService("message")
	if err != nil {
		panic(fmt.Sprintf("failed to connect message: %v", err))
	}
	defer messageConn.Close()

	pushConn, err := dialService("push")
	if err != nil {
		panic(fmt.Sprintf("failed to connect push: %v", err))
	}
	defer pushConn.Close()

	msgGatewayClient := msggatewaypb.NewMsgGatewayClient(msgGatewayConn)
	pushMsg := func(ctx context.Context, recvID string, msg *messagepb.MsgData) error {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = metadata.NewOutgoingContext(ctx, md.Copy())
		_, err := msgGatewayClient.OnlinePushMsg(ctx, &msggatewaypb.OnlinePushMsgReq{
			MsgData:      msg,
			PushToUserId: recvID,
		})
		return err
	}

	callRepo := repository.NewCallRepository(db)
	callNotifier := notification.NewCallNotificationSender(pushMsg)
	callSvc := service.NewCallService(
		callRepo,
		cfg,
		callNotifier,
		&client.RelationFriendChecker{Client: relationpb.NewRelationServiceClient(relationConn)},
		&client.MsgGatewayPresence{Client: msgGatewayClient},
		&client.MessageTimelineWriter{Client: messagepb.NewMessageClient(messageConn)},
		&client.PushOfflinePusher{Client: pushpb.NewPushMsgServiceClient(pushConn)},
	)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor(cfg.JWTSecret)),
	)
	pb.RegisterRtcServiceServer(grpcSvr, handler.NewRtcHandler(callSvc, cfg.LiveKitURL))
	reflection.Register(grpcSvr)

	lis, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to listen on %s: %v", cfg.ServerAddr, err))
	}

	slog.Info("rtc gRPC server starting", "addr", cfg.ServerAddr)
	if err := grpcSvr.Serve(lis); err != nil {
		panic(fmt.Sprintf("rtc gRPC server exited: %v", err))
	}
}
