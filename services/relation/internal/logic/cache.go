package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/relationcache"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

// UseCache attaches the Redis and process-local friend cache.
// A nil cache keeps every read on MySQL, which is how the in-process tests run.
func (s *Server) UseCache(lists *relationcache.Lists) {
	if s != nil {
		s.lists = lists
	}
}

func (s *Server) friendIDs(ctx context.Context, owner string) ([]string, error) {
	if s.lists == nil {
		return s.store.FriendUserIDs(ctx, owner)
	}
	return s.lists.FriendIDs(ctx, owner, func(ctx context.Context) ([]string, error) {
		return s.store.FriendUserIDs(ctx, owner)
	})
}

func (s *Server) blackIDs(ctx context.Context, owner string) ([]string, error) {
	if s.lists == nil {
		return s.store.BlackUserIDs(ctx, owner)
	}
	return s.lists.BlackIDs(ctx, owner, func(ctx context.Context) ([]string, error) {
		return s.store.BlackUserIDs(ctx, owner)
	})
}

func (s *Server) friendHead(ctx context.Context, owner string) (model.VersionHead, error) {
	if s.lists == nil {
		return s.store.FriendVersion(ctx, owner)
	}
	head, err := s.lists.Version(ctx, owner, func(ctx context.Context) (relationcache.Version, error) {
		row, err := s.store.FriendVersion(ctx, owner)
		if err != nil {
			return relationcache.Version{}, err
		}
		return relationcache.Version{ID: row.ID, Version: row.Version}, nil
	})
	if err != nil {
		return model.VersionHead{}, err
	}
	return model.VersionHead{ID: head.ID, Version: head.Version}, nil
}

func (s *Server) friendship(ctx context.Context, user1, user2 string) (bool, bool, error) {
	left, err := s.friendIDs(ctx, user1)
	if err != nil {
		return false, false, err
	}
	right, err := s.friendIDs(ctx, user2)
	if err != nil {
		return false, false, err
	}
	return containsID(left, user2), containsID(right, user1), nil
}

func (s *Server) blocked(ctx context.Context, user1, user2 string) (bool, bool, error) {
	left, err := s.blackIDs(ctx, user1)
	if err != nil {
		return false, false, err
	}
	right, err := s.blackIDs(ctx, user2)
	if err != nil {
		return false, false, err
	}
	return containsID(left, user2), containsID(right, user1), nil
}

func (s *Server) dropFriends(ctx context.Context, owners ...string) {
	if s.lists != nil {
		s.lists.DropFriends(ctx, owners...)
	}
}

func (s *Server) dropBlacks(ctx context.Context, owners ...string) {
	if s.lists != nil {
		s.lists.DropBlacks(ctx, owners...)
	}
}

func containsID(ids []string, id string) bool {
	for _, item := range ids {
		if item == id {
			return true
		}
	}
	return false
}
