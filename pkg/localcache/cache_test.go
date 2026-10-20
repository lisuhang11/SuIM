package localcache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetUsesCacheUntilDeleted(t *testing.T) {
	ctx := context.Background()
	c := New[int](WithLocalSlotNum(4), WithLocalSlotSize(8), WithLocalSuccessTTL(time.Minute))
	defer c.Stop()

	var fetches atomic.Int32
	fetch := func(context.Context) (int, error) {
		fetches.Add(1)
		return 7, nil
	}
	got, err := c.Get(ctx, "user:1", fetch)
	if err != nil || got != 7 || fetches.Load() != 1 {
		t.Fatalf("first get=%d fetches=%d err=%v", got, fetches.Load(), err)
	}
	got, err = c.Get(ctx, "user:1", fetch)
	if err != nil || got != 7 || fetches.Load() != 1 {
		t.Fatalf("second get=%d fetches=%d err=%v", got, fetches.Load(), err)
	}

	c.Del(ctx, "user:1")
	got, err = c.Get(ctx, "user:1", fetch)
	if err != nil || got != 7 || fetches.Load() != 2 {
		t.Fatalf("after delete get=%d fetches=%d err=%v", got, fetches.Load(), err)
	}
}

func TestGetLinkDeletesRelatedKeys(t *testing.T) {
	ctx := context.Background()
	c := New[string](WithLocalSlotNum(2), WithLocalSlotSize(8), WithLinkSlotNum(2), WithLocalSuccessTTL(time.Minute))
	defer c.Stop()

	if _, err := c.GetLink(ctx, "member:1", func(context.Context) (string, error) { return "m", nil }, "group:1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(ctx, "group:1", func(context.Context) (string, error) { return "g", nil }); err != nil {
		t.Fatal(err)
	}

	var fetches atomic.Int32
	c.Del(ctx, "group:1")
	deadline := time.Now().Add(time.Second)
	for fetches.Load() == 0 {
		if _, err := c.Get(ctx, "member:1", func(context.Context) (string, error) {
			fetches.Add(1)
			return "m2", nil
		}); err != nil {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("linked key was not deleted")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestFailedFetchIsCached(t *testing.T) {
	ctx := context.Background()
	c := New[int](WithLocalSlotNum(1), WithLocalSlotSize(4), WithLocalFailedTTL(time.Minute))
	defer c.Stop()

	want := errors.New("missing")
	var fetches atomic.Int32
	fetch := func(context.Context) (int, error) {
		fetches.Add(1)
		return 0, want
	}
	_, err := c.Get(ctx, "k", fetch)
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	_, err = c.Get(ctx, "k", fetch)
	if !errors.Is(err, want) || fetches.Load() != 1 {
		t.Fatalf("fetches=%d err=%v", fetches.Load(), err)
	}
}

func TestGetPublishKeysByTopic(t *testing.T) {
	InitLocalCache([]Subscription{
		{Topic: "user", Keys: []string{"USER_INFO:", "USER_RECV:"}},
		{Topic: "group", Keys: []string{"GROUP_INFO:"}},
	})
	got := GetPublishKeysByTopic([]string{"user", "group"}, []string{"USER_INFO:1", "GROUP_INFO:2", "OTHER"})
	if len(got["user"]) != 1 || got["user"][0] != "USER_INFO:1" {
		t.Fatalf("user keys=%v", got["user"])
	}
	if len(got["group"]) != 1 || got["group"][0] != "GROUP_INFO:2" {
		t.Fatalf("group keys=%v", got["group"])
	}
}
