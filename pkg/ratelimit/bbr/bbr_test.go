package bbr

import (
	"errors"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/ratelimit/types"
)

func TestAllowShedsOverCapacityWhenCPUHot(t *testing.T) {
	limiter := NewBBRLimiter(
		WithCPUGetter(func() int64 { return 1000 }),
		WithCPUThreshold(800),
		WithWindow(20*time.Second),
		WithBucket(500),
	)
	done1, err := limiter.Allow()
	if err != nil {
		t.Fatalf("first allow: %v", err)
	}
	done2, err := limiter.Allow()
	if err != nil {
		t.Fatalf("second allow: %v", err)
	}
	if _, err := limiter.Allow(); !errors.Is(err, types.ErrLimitExceeded) {
		t.Fatalf("third allow err = %v, want ErrLimitExceeded", err)
	}
	done1(types.DoneInfo{})
	done2(types.DoneInfo{})
	if _, err := limiter.Allow(); err != nil {
		t.Fatalf("allow after done: %v", err)
	}
}

func TestAllowUnderCPUThreshold(t *testing.T) {
	limiter := NewBBRLimiter(
		WithCPUGetter(func() int64 { return 100 }),
		WithCPUThreshold(800),
	)
	for i := 0; i < 10; i++ {
		done, err := limiter.Allow()
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		done(types.DoneInfo{})
	}
}

func TestDoneIsIdempotentAndRecordsStats(t *testing.T) {
	limiter := NewBBRLimiter(WithCPUGetter(func() int64 { return 100 }))
	done, err := limiter.Allow()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	done(types.DoneInfo{})
	done(types.DoneInfo{})

	stat := limiter.Stat()
	if stat.InFlight != 0 {
		t.Fatalf("inFlight = %d, want 0", stat.InFlight)
	}
	if stat.MaxPass < 1 {
		t.Fatalf("maxPass = %d, want >= 1", stat.MaxPass)
	}
	if stat.MinRT < 1 {
		t.Fatalf("minRT = %d, want >= 1", stat.MinRT)
	}
}

func TestCPUQuotaScalesThreshold(t *testing.T) {
	limiter := NewBBRLimiter(
		WithCPUGetter(func() int64 { return 900 }),
		WithCPUQuota(0.9),
		WithCPUThreshold(800),
	)
	if got := limiter.Stat().CPU; got < 800 {
		t.Fatalf("scaled CPU = %d, want >= 800", got)
	}
}
