package register

import (
	"context"

	"google.golang.org/grpc"
)

type ServiceInstance struct {
	ID       string
	Name     string
	Addr     string
	Metadata map[string]string
}

type Registry interface {
	Register(ctx context.Context, instance *ServiceInstance) error
	Deregister(ctx context.Context) error
	Dial(ctx context.Context, service string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	Close() error
}

type Config struct {
	Type string
	Etcd EtcdConfig
	K8s  K8sConfig
}

type EtcdConfig struct {
	Endpoints   []string
	RootDir     string
	Username    string
	Password    string
	DialTimeout int
	TTL         int
}

type K8sConfig struct {
	Namespace string
	Port      int
}