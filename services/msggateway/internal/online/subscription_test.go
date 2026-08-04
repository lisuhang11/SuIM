package online

import (
	"testing"

	"msggateway/internal/types"
)

type stubConn struct {
	id     string
	closed bool
}

func (c *stubConn) ID() string                 { return c.id }
func (c *stubConn) Close(int, string)          {}
func (c *stubConn) IsClosed() bool             { return c.closed }

var _ types.WritableConn = (*stubConn)(nil)

func TestSubscriptionSubAndDel(t *testing.T) {
	s := NewSubscription()
	a := &stubConn{id: "c1"}
	b := &stubConn{id: "c2"}

	s.Sub(a, []string{"u1", "u2"}, nil)
	s.Sub(b, []string{"u1"}, nil)

	subs := s.Subscribers("u1")
	if len(subs) != 2 {
		t.Fatalf("u1 subscribers=%d want 2", len(subs))
	}
	if len(s.Subscribers("u2")) != 1 {
		t.Fatalf("u2 subscribers=%d want 1", len(s.Subscribers("u2")))
	}

	s.Sub(a, nil, []string{"u1"})
	if len(s.Subscribers("u1")) != 1 {
		t.Fatalf("after unsub u1 from a: got %d", len(s.Subscribers("u1")))
	}

	s.DelConn(b)
	if len(s.Subscribers("u1")) != 0 {
		t.Fatalf("after DelConn b: u1 still has subscribers")
	}
	if len(s.Subscribers("u2")) != 1 {
		t.Fatalf("u2 should still be subscribed by a")
	}

	s.DelConn(a)
	if len(s.Subscribers("u2")) != 0 {
		t.Fatalf("after DelConn a: expected empty")
	}
}

func TestSamePlatformSet(t *testing.T) {
	if !samePlatformSet([]int32{1, 2}, []int32{2, 1}) {
		t.Fatal("order should not matter")
	}
	if samePlatformSet([]int32{1}, []int32{1, 2}) {
		t.Fatal("different length")
	}
	if statusFromPlatforms(nil) != 0 || statusFromPlatforms([]int32{1}) != 1 {
		t.Fatal("statusFromPlatforms")
	}
}
