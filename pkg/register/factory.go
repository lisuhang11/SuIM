package register

import "fmt"

func New(cfg *Config) (Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	switch cfg.Type {
	case "etcd":
		return NewEtcdRegistry(cfg)
	case "kubernetes":
		return NewK8sRegistry(cfg)
	default:
		return nil, fmt.Errorf("unsupported registry type: %s", cfg.Type)
	}
}