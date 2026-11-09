package register

import (
	"context"
	"fmt"
	"net"

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

func (r *k8sRegistry) List(ctx context.Context, service string) ([]ServiceInstance, error) {
	if service == "" || r.namespace == "" {
		return nil, fmt.Errorf("service name and namespace are required")
	}
	host := fmt.Sprintf("%s.%s.svc.cluster.local", service, r.namespace)
	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("lookup %s: %w", host, err)
	}
	out := make([]ServiceInstance, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ServiceInstance{
			ID:   ip,
			Name: service,
			Addr: net.JoinHostPort(ip, fmt.Sprintf("%d", r.port)),
		})
	}
	return out, nil
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
