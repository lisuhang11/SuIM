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
	Type string     `yaml:"type"`
	Etcd EtcdConfig `yaml:"etcd"`
	K8s  K8sConfig  `yaml:"k8s"`
}

type EtcdConfig struct {
	Endpoints   []string `yaml:"endpoints"`
	RootDir     string   `yaml:"rootDir"`
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password"`
	DialTimeout int      `yaml:"dialTimeout"`
	TTL         int      `yaml:"ttl"`
}

type K8sConfig struct {
	Namespace string `yaml:"namespace"`
	Port      int    `yaml:"port"`
}
