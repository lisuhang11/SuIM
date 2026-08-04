package discovery

import (
	"context"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type fakeEtcd struct {
	mu         sync.Mutex
	grants     int
	puts       int
	keepAlives int
	closed     bool

	currentCh chan *clientv3.LeaseKeepAliveResponse
	keepErr   error
}

func (f *fakeEtcd) Grant(_ context.Context, _ int64) (*clientv3.LeaseGrantResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants++
	return &clientv3.LeaseGrantResponse{ID: clientv3.LeaseID(f.grants)}, nil
}

func (f *fakeEtcd) Put(_ context.Context, _ string, _ string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts++
	return &clientv3.PutResponse{}, nil
}

func (f *fakeEtcd) KeepAlive(_ context.Context, _ clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.keepErr != nil {
		return nil, f.keepErr
	}
	f.keepAlives++
	ch := make(chan *clientv3.LeaseKeepAliveResponse, 1)
	f.currentCh = ch
	ch <- &clientv3.LeaseKeepAliveResponse{TTL: 15}
	return ch, nil
}

func (f *fakeEtcd) Revoke(_ context.Context, _ clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	return &clientv3.LeaseRevokeResponse{}, nil
}

func (f *fakeEtcd) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeEtcd) closeKeepAlive() {
	f.mu.Lock()
	ch := f.currentCh
	f.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (f *fakeEtcd) stats() (grants, puts, keepAlives int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.grants, f.puts, f.keepAlives
}

func TestKeepAliveReRegistersWhenChannelCloses(t *testing.T) {
	fake := &fakeEtcd{}
	r, err := newRegistryWithClient("message", "message:8084", fake)
	if err != nil {
		t.Fatalf("newRegistryWithClient: %v", err)
	}
	defer r.Deregister()

	deadline := time.Now().Add(2 * time.Second)
	for {
		_, _, ka := fake.stats()
		if ka >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("keepalive never started")
		}
		time.Sleep(10 * time.Millisecond)
	}

	fake.closeKeepAlive()

	deadline = time.Now().Add(3 * time.Second)
	for {
		grants, puts, ka := fake.stats()
		if grants >= 2 && puts >= 2 && ka >= 2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected re-register after keepalive close; grants=%d puts=%d keepAlives=%d", grants, puts, ka)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestKeepAliveStopsOnDeregister(t *testing.T) {
	fake := &fakeEtcd{}
	r, err := newRegistryWithClient("push", "push:8085", fake)
	if err != nil {
		t.Fatalf("newRegistryWithClient: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		_, _, ka := fake.stats()
		if ka >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("keepalive never started")
		}
		time.Sleep(10 * time.Millisecond)
	}

	r.Deregister()
	grantsBefore, _, _ := fake.stats()
	fake.closeKeepAlive()
	time.Sleep(200 * time.Millisecond)
	grantsAfter, _, _ := fake.stats()
	if grantsAfter != grantsBefore {
		t.Fatalf("deregistered registry should not re-register; before=%d after=%d", grantsBefore, grantsAfter)
	}
}
