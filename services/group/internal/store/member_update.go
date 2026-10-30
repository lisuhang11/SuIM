package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

type MemberUpdate struct {
	UserID string
	Patch  model.MemberPatch
}

// UpdateGroupMembers applies partial member updates. Groups are locked in id order.
// A role change also writes the sort sentinel into the member version log.
func (s *Store) UpdateGroupMembers(ctx context.Context, groups map[string][]MemberUpdate, now time.Time) error {
	if len(groups) == 0 {
		return nil
	}
	groupIDs := make([]string, 0, len(groups))
	for groupID := range groups {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Strings(groupIDs)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, groupID := range groupIDs {
		if err := updateGroupMembers(ctx, tx, groupID, groups[groupID], now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		for _, groupID := range groupIDs {
			ids := make([]string, 0, len(groups[groupID]))
			for _, update := range groups[groupID] {
				ids = append(ids, update.UserID)
			}
			s.cache.dropMembers(ctx, groupID, ids...)
		}
	}
	return nil
}

func updateGroupMembers(ctx context.Context, tx *sql.Tx, groupID string, updates []MemberUpdate, now time.Time) error {
	var status int32
	err := tx.QueryRowContext(ctx, "SELECT status FROM `group` WHERE group_id = ? FOR UPDATE", groupID).Scan(&status)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == model.GroupStatusDismissed {
		return ErrDismissed
	}

	ids := make([]string, 0, len(updates))
	seen := map[string]struct{}{}
	for _, update := range updates {
		if _, ok := seen[update.UserID]; ok {
			continue
		}
		seen[update.UserID] = struct{}{}
		ids = append(ids, update.UserID)
	}
	sort.Strings(ids)
	placeholders := placeholders(len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, groupID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx, "SELECT user_id FROM group_member WHERE group_id = ? AND user_id IN ("+placeholders+") FOR UPDATE", args...)
	if err != nil {
		return err
	}
	found := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		found[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(found) != len(ids) {
		return ErrNotMember
	}

	roleChanged := false
	for _, update := range updates {
		setSQL, setArgs, role := memberSet(update.Patch)
		if setSQL == "" {
			continue
		}
		roleChanged = roleChanged || role
		setArgs = append(setArgs, groupID, update.UserID)
		if _, err := tx.ExecContext(ctx, "UPDATE group_member SET "+setSQL+" WHERE group_id = ? AND user_id = ?", setArgs...); err != nil {
			return err
		}
	}
	versionIDs := append([]string{}, ids...)
	if roleChanged {
		versionIDs = append(versionIDs, model.SortElemID)
	}
	return incrMembers(ctx, tx, groupID, versionIDs, model.VersionUpdate, now)
}

func memberSet(patch model.MemberPatch) (string, []any, bool) {
	var sets []string
	var args []any
	role := false
	if patch.Nickname != nil {
		sets = append(sets, "nickname = ?")
		args = append(args, *patch.Nickname)
	}
	if patch.FaceURL != nil {
		sets = append(sets, "face_url = ?")
		args = append(args, *patch.FaceURL)
	}
	if patch.RoleLevel != nil {
		sets = append(sets, "role_level = ?")
		args = append(args, *patch.RoleLevel)
		role = true
	}
	if patch.Ex != nil {
		sets = append(sets, "ex = ?")
		args = append(args, *patch.Ex)
	}
	return strings.Join(sets, ", "), args, role
}
