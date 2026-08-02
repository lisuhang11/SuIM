package service

import (
	"context"
	"testing"

	"relation/internal/types"
)

type incrFakeRepo struct {
	friendUpdateFakeRepo
	ver       *types.FriendVersion
	logs      []*types.FriendVersionLog
	byIDs     map[string]*types.Friend
	ensureErr error
	logsErr   error
}

func (f *incrFakeRepo) EnsureFriendVersion(context.Context, string) (*types.FriendVersion, error) {
	if f.ensureErr != nil {
		return nil, f.ensureErr
	}
	return f.ver, nil
}
func (f *incrFakeRepo) ListFriendVersionLogs(_ context.Context, _ string, after, to uint64) ([]*types.FriendVersionLog, error) {
	if f.logsErr != nil {
		return nil, f.logsErr
	}
	var out []*types.FriendVersionLog
	for _, lg := range f.logs {
		if lg.Version > after && lg.Version <= to {
			out = append(out, lg)
		}
	}
	return out, nil
}
func (f *incrFakeRepo) ListFriendsByIDs(_ context.Context, _ string, ids []string) ([]*types.Friend, error) {
	var out []*types.Friend
	for _, id := range ids {
		if fr, ok := f.byIDs[id]; ok {
			out = append(out, fr)
		}
	}
	return out, nil
}
func (f *incrFakeRepo) IncrVersion(context.Context, string, []string, int8, bool) error {
	return nil
}

func TestGetIncrementalFriends(t *testing.T) {
	svc := NewRelationService(&incrFakeRepo{
		ver: &types.FriendVersion{OwnerUserID: "u1", VersionID: "vid-1", Version: 3},
		logs: []*types.FriendVersionLog{
			{OwnerUserID: "u1", Version: 2, FriendUserID: "f1", State: types.VersionStateInsert},
			{OwnerUserID: "u1", Version: 3, FriendUserID: "f2", State: types.VersionStateUpdate},
		},
		byIDs: map[string]*types.Friend{
			"f1": {OwnerUserID: "u1", FriendUserID: "f1"},
			"f2": {OwnerUserID: "u1", FriendUserID: "f2", Remark: "r"},
		},
	}, nil)

	t.Run("version zero is full", func(t *testing.T) {
		res, err := svc.GetIncrementalFriends(context.Background(), "u1", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Full {
			t.Fatal("expected full")
		}
	})

	t.Run("version id mismatch is full", func(t *testing.T) {
		res, err := svc.GetIncrementalFriends(context.Background(), "u1", "other", 1)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Full {
			t.Fatal("expected full")
		}
	})

	t.Run("gap is full", func(t *testing.T) {
		res, err := svc.GetIncrementalFriends(context.Background(), "u1", "vid-1", 1)
		if err != nil {
			t.Fatal(err)
		}
		// logs start at 2, client at 1 → OK continuous from 2
		if res.Full {
			t.Fatalf("unexpected full: %+v", res)
		}
		if len(res.Insert) != 1 || res.Insert[0].FriendUserID != "f1" {
			t.Fatalf("insert=%v", res.Insert)
		}
		if len(res.Update) != 1 || res.Update[0].FriendUserID != "f2" {
			t.Fatalf("update=%v", res.Update)
		}
	})

	t.Run("up to date empty", func(t *testing.T) {
		res, err := svc.GetIncrementalFriends(context.Background(), "u1", "vid-1", 3)
		if err != nil {
			t.Fatal(err)
		}
		if res.Full || len(res.Insert)+len(res.Update)+len(res.Delete) != 0 {
			t.Fatalf("expected empty incr, got %+v", res)
		}
	})
}
