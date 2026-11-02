package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// MemberFriendForbidden reports whether the two users share a live group that
// forbids members from adding each other. A dismissed group does not count.
func (s *Store) MemberFriendForbidden(ctx context.Context, user1, user2 string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1
		FROM group_member AS a
		INNER JOIN group_member AS b ON a.group_id = b.group_id
		INNER JOIN `+"`group`"+` AS g ON g.group_id = a.group_id
		WHERE a.user_id = ? AND b.user_id = ? AND g.status <> ? AND g.apply_member_friend = 0
		LIMIT 1`, user1, user2, model.GroupStatusDismissed).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
