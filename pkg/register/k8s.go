package register

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/resolver/dns"
)

var _ Registry = (*k8sRegistry)(nil)

type k8sRegistry struct {
	namespace string
	port      int
}

func NewK8sRegistry(cfg *Config) (Registry, error) {
	port := cfg.K8s.Port
	if port <= 0 {
		port = 9090
	}
	return &k8sRegistry{namespace: cfg.K8s.Namespace, port: port}, nil
}

func (r *k8sRegistry) Register(ctx context.Context, inst *ServiceInstance) error {
	// K8s 由 Headless Service + readinessProbe 自动管理
	return nil
}

func (r *k8sRegistry) Deregister(ctx context.Context) error {
	return nil
}

func (r *k8sRegistry) Dial(ctx context.Context, service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	target := fmt.Sprintf("dns:///%s.%s.svc.cluster.local:%d", service, r.namespace, r.port)
	defaultOpts := []grpc.DialOption{
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(20 * 1024 * 1024)),
	}
	return grpc.NewClient(target, append(defaultOpts, opts...)...)
}

func (r *k8sRegistry) Close() error { return nil }