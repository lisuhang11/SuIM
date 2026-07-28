// Package grpc 管理到后端微服务的 gRPC 客户端连接池，通过 etcd 服务发现动态解析地址。
package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"apigateway/internal/config"

	pbUser "SuIM/proto/userpb"
	pbRel "SuIM/proto/relationpb"
	pbGroup "SuIM/proto/grouppb"
	pbConv "SuIM/proto/conversationpb"
	pbMsg "SuIM/proto/messagepb"
	"SuIM/pkg/discovery"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Clients 持有所有后端 gRPC 客户端，对外只暴露接口。
// 所有后端地址通过 etcd 服务发现自动解析，不再使用静态 IP 直连。
type Clients struct {
	User         pbUser.UserServiceClient
	Relation     pbRel.RelationServiceClient
	Group        pbGroup.GroupServiceClient
	Conversation pbConv.ConversationClient
	Message      pbMsg.MessageClient

	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn
}

// NewClients 根据配置创建所有 gRPC 客户端连接，使用 etcd 服务发现。
func NewClients(cfg *config.GatewayConfig) (*Clients, error) {
	c := &Clients{
		conns: make(map[string]*grpc.ClientConn),
	}

	// 后端服务列表及对应的 proto 客户端创建函数。
	backends := []struct {
		name   string
		setter func(*Clients, *grpc.ClientConn)
	}{
		{"user", func(c *Clients, conn *grpc.ClientConn) { c.User = pbUser.NewUserServiceClient(conn) }},
		{"relation", func(c *Clients, conn *grpc.ClientConn) { c.Relation = pbRel.NewRelationServiceClient(conn) }},
		{"group", func(c *Clients, conn *grpc.ClientConn) { c.Group = pbGroup.NewGroupServiceClient(conn) }},
		{"conversation", func(c *Clients, conn *grpc.ClientConn) { c.Conversation = pbConv.NewConversationClient(conn) }},
		{"message", func(c *Clients, conn *grpc.ClientConn) { c.Message = pbMsg.NewMessageClient(conn) }},
	}

	for _, b := range backends {
		conn, err := c.createConn(b.name)
		if err != nil {
			return nil, fmt.Errorf("create grpc client for %s: %w", b.name, err)
		}
		c.conns[b.name] = conn
		b.setter(c, conn)
		slog.Info("[gateway] gRPC client created via etcd discovery", "service", b.name)
	}

	return c, nil
}

// createConn 创建到指定服务的 gRPC 连接，使用 etcd 服务发现。
func (c *Clients) createConn(serviceName string) (*grpc.ClientConn, error) {
	target := discovery.TargetURL(serviceName)
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", target, err)
	}
	return conn, nil
}

// Reload 根据新配置热切换后端连接（先建新连接，再断开旧连接）。
func (c *Clients) Reload(ctx context.Context, newCfg *config.GatewayConfig) error {
	newClients := &Clients{
		conns: make(map[string]*grpc.ClientConn),
	}

	backends := []struct {
		name   string
		setter func(*Clients, *grpc.ClientConn)
	}{
		{"user", func(c *Clients, conn *grpc.ClientConn) { c.User = pbUser.NewUserServiceClient(conn) }},
		{"relation", func(c *Clients, conn *grpc.ClientConn) { c.Relation = pbRel.NewRelationServiceClient(conn) }},
		{"group", func(c *Clients, conn *grpc.ClientConn) { c.Group = pbGroup.NewGroupServiceClient(conn) }},
		{"conversation", func(c *Clients, conn *grpc.ClientConn) { c.Conversation = pbConv.NewConversationClient(conn) }},
		{"message", func(c *Clients, conn *grpc.ClientConn) { c.Message = pbMsg.NewMessageClient(conn) }},
	}

	for _, b := range backends {
		conn, err := newClients.createConn(b.name)
		if err != nil {
			return fmt.Errorf("reload grpc client for %s: %w", b.name, err)
		}
		newClients.conns[b.name] = conn
		b.setter(newClients, conn)
	}

	// 原子替换。
	c.mu.Lock()
	oldConns := c.conns
	c.User = newClients.User
	c.Relation = newClients.Relation
	c.Group = newClients.Group
	c.Conversation = newClients.Conversation
	c.Message = newClients.Message
	c.conns = newClients.conns
	c.mu.Unlock()

	// 异步关闭旧连接，给 in-flight 请求缓冲时间。
	go func() {
		for name, conn := range oldConns {
			slog.Info("[gateway] closing old gRPC connection", "service", name)
			conn.Close()
		}
	}()

	slog.Info("[gateway] gRPC clients reloaded")
	return nil
}

// Close 关闭所有 gRPC 连接。
func (c *Clients) Close() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name, conn := range c.conns {
		slog.Info("[gateway] closing gRPC connection", "service", name)
		conn.Close()
	}
}
