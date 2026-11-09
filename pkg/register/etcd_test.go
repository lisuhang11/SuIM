package register

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdRegistryReregistersAfterLeaseLoss(t *testing.T) {
	reg, err := NewEtcdRegistry(&Config{
		Etcd: EtcdConfig{Endpoints: []string{"127.0.0.1:2379"}, DialTimeout: 5, TTL: 10},
	})
	if err != nil {
		t.Skipf("etcd is not reachable: %v", err)
	}
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}, DialTimeout: 5 * time.Second})
	if err != nil {
		_ = reg.Close()
		t.Skipf("etcd is not reachable: %v", err)
	}
	name := fmt.Sprintf("suim-regtest-%d", os.Getpid())
	const id = "inst"
	key := fmt.Sprintf("services/%s/%s", name, id)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = cli.Delete(ctx, "services/"+name+"/", clientv3.WithPrefix())
		_ = cli.Close()
		_ = reg.Close()
	})

	regCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = reg.Register(regCtx, &ServiceInstance{ID: id, Name: name, Addr: "127.0.0.1:9"})
	cancel()
	if err != nil {
		t.Skipf("etcd is not reachable: %v", err)
	}

	first := leaseOf(t, cli, key)
	if first == 0 {
		t.Fatal("registration missing")
	}
	revokeCtx, revokeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, err = cli.Revoke(revokeCtx, first)
	revokeCancel()
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	var next clientv3.LeaseID
	for time.Now().Before(deadline) {
		next = leaseOf(t, cli, key)
		if next != 0 && next != first {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if next == 0 || next == first {
		t.Fatalf("registration was not restored, lease %d", next)
	}
	items, err := reg.List(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id || items[0].Addr != "127.0.0.1:9" {
		t.Fatalf("list %+v", items)
	}

	offCtx, offCancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := reg.Deregister(offCtx); err != nil {
		offCancel()
		t.Fatal(err)
	}
	offCancel()
	time.Sleep(2 * time.Second)
	if got := leaseOf(t, cli, key); got != 0 {
		t.Fatalf("registration came back after deregister, lease %d", got)
	}
}

func leaseOf(t *testing.T, cli *clientv3.Client, key string) clientv3.LeaseID {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	if len(resp.Kvs) == 0 {
		return 0
	}
	return clientv3.LeaseID(resp.Kvs[0].Lease)
}
