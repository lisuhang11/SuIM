package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// SetMemberMute writes one member's mute end. A nil end clears it.
// The member version log records that user, not the role-sort sentinel.
func (s *Store) SetMemberMute(ctx context.Context, groupID, userID string, until *time.Time, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := lockActiveGroup(ctx, tx, groupID); err != nil {
		return err
	}
	var id string
	err = tx.QueryRowContext(ctx, "SELECT user_id FROM group_member WHERE group_id = ? AND user_id = ? FOR UPDATE", groupID, userID).Scan(&id)
	if err == sql.ErrNoRows {
		return ErrNotMember
	}
	if err != nil {
		return err
	}
	var mute any
	if until != nil {
		mute = until.UTC()
	}
	if _, err := tx.ExecContext(ctx, "UPDATE group_member SET mute_end_time = ? WHERE group_id = ? AND user_id = ?", mute, groupID, userID); err != nil {
		return err
	}
	if err := incrMembers(ctx, tx, groupID, []string{userID}, model.VersionUpdate, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropMembers(ctx, groupID, userID)
	}
	return nil
}

// SetGroupMuted sets status to muted or back to normal. A dismissed group is left unchanged.
// The empty member-version element tells clients the group profile changed.
func (s *Store) SetGroupMuted(ctx context.Context, groupID string, muted bool, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := lockActiveGroup(ctx, tx, groupID); err != nil {
		return err
	}
	status := int32(model.GroupStatusOK)
	if muted {
		status = model.GroupStatusMuted
	}
	if _, err := tx.ExecContext(ctx, "UPDATE `group` SET status = ? WHERE group_id = ?", status, groupID); err != nil {
		return err
	}
	if err := incrMembers(ctx, tx, groupID, []string{""}, model.VersionUpdate, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.cache != nil {
		s.cache.dropGroupInfo(ctx, groupID)
	}
	return nil
}

func lockActiveGroup(ctx context.Context, tx *sql.Tx, groupID string) error {
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
	return nil
}
