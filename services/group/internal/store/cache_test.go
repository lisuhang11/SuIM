package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

func testCache(t *testing.T) (*readCache, func()) {
	t.Helper()
	cache, stop := NewReadCache(context.Background(), nil, CacheConfig{
		SlotNum:       4,
		SlotSize:      16,
		SuccessExpire: time.Minute,
		FailedExpire:  time.Minute,
	})
	return cache, stop
}

func TestTakeGroupReadsCacheUntilMembersChange(t *testing.T) {
	ctx := context.Background()
	cache, stop := testCache(t)
	defer stop()
	s := &Store{cache: cache}

	_, err := cache.groups.Get(ctx, groupKey("g1"), func(context.Context) (model.Group, error) {
		return model.Group{GroupID: "g1", GroupName: "room", MemberCount: 2, OwnerUserID: "a"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.TakeGroup(ctx, "g1")
	if err != nil || got.GroupName != "room" || got.MemberCount != 2 || got.OwnerUserID != "a" {
		t.Fatalf("%+v err=%v", got, err)
	}
	got.GroupName = "changed"
	again, err := s.TakeGroup(ctx, "g1")
	if err != nil || again.GroupName != "room" {
		t.Fatalf("cache mutated %+v err=%v", again, err)
	}

	cache.dropMembers(ctx, "g1", "a")
	var fetches int
	reloaded, err := cache.groups.Get(ctx, groupKey("g1"), func(context.Context) (model.Group, error) {
		fetches++
		return model.Group{GroupID: "g1", GroupName: "reloaded"}, nil
	})
	if err != nil || fetches != 1 || reloaded.GroupName != "reloaded" {
		t.Fatalf("fetches=%d group=%+v err=%v", fetches, reloaded, err)
	}
}

func TestFindGroupsUsesCachedHits(t *testing.T) {
	ctx := context.Background()
	cache, stop := testCache(t)
	defer stop()
	s := &Store{cache: cache}
	for _, group := range []model.Group{
		{GroupID: "g1", GroupName: "one"},
		{GroupID: "g2", GroupName: "two"},
	} {
		group := group
		if _, err := cache.groups.Get(ctx, groupKey(group.GroupID), func(context.Context) (model.Group, error) {
			return group, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cache.groups.Get(ctx, groupKey("missing"), func(context.Context) (model.Group, error) {
		return model.Group{}, ErrNotFound
	}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	got, err := s.FindGroups(ctx, []string{"g2", "missing", "g1", "g2"})
	if err != nil || len(got) != 2 || got[0].GroupID != "g2" || got[1].GroupID != "g1" {
		t.Fatalf("%+v err=%v", got, err)
	}
}

func TestFindMembersSkipsCachedAbsence(t *testing.T) {
	ctx := context.Background()
	cache, stop := testCache(t)
	defer stop()
	s := &Store{cache: cache}
	if _, err := cache.members.Get(ctx, memberKey("g1", "a"), func(context.Context) (model.Member, error) {
		return model.Member{GroupID: "g1", UserID: "a", RoleLevel: model.RoleOwner}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.members.Get(ctx, memberKey("g1", "b"), func(context.Context) (model.Member, error) {
		return model.Member{}, ErrNotMember
	}); !errors.Is(err, ErrNotMember) {
		t.Fatal(err)
	}
	got, err := s.FindMembers(ctx, "g1", []string{"b", "a", "b"})
	if err != nil || len(got) != 1 || got[0].UserID != "a" || got[0].RoleLevel != model.RoleOwner {
		t.Fatalf("%+v err=%v", got, err)
	}
}

func TestMemberUserIDsCopy(t *testing.T) {
	ctx := context.Background()
	cache, stop := testCache(t)
	defer stop()
	s := &Store{cache: cache}
	if _, err := cache.memberIDs.Get(ctx, memberIDsKey("g1"), func(context.Context) ([]string, error) {
		return []string{"a", "b"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.MemberUserIDs(ctx, "g1")
	if err != nil || len(got) != 2 {
		t.Fatalf("%v err=%v", got, err)
	}
	got[0] = "changed"
	again, err := s.MemberUserIDs(ctx, "g1")
	if err != nil || again[0] != "a" {
		t.Fatalf("cache mutated %v err=%v", again, err)
	}
}

func TestSplitKeys(t *testing.T) {
	got := splitKeys(",group:1,,member:1:a,")
	if len(got) != 2 || got[0] != "group:1" || got[1] != "member:1:a" {
		t.Fatalf("%v", got)
	}
}
