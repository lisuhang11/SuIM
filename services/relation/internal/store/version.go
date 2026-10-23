package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

const (
	friendVersionHead = "friend_version"
	friendVersionLog  = "friend_version_log"
)

func (s *Store) FriendVersion(ctx context.Context, owner string) (model.VersionHead, error) {
	return s.maxVersion(ctx, friendVersionHead, owner)
}

func (s *Store) FriendChanges(ctx context.Context, owner string, version uint64, limit int) ([]model.VersionLog, error) {
	return s.changes(ctx, friendVersionLog, owner, version, limit)
}

func (s *Store) FriendSortVersion(ctx context.Context, owner string) (uint64, error) {
	var version uint64
	err := s.db.QueryRowContext(ctx, `SELECT version FROM friend_version_log WHERE owner_user_id = ? AND elem_id = ?`, owner, model.SortElemID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return version, err
}

func (s *Store) PageFriends(ctx context.Context, owner string, offset, limit int) (int, []model.Friend, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM friend WHERE owner_user_id = ?`, owner).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.db.QueryContext(ctx, friendSelect+` WHERE owner_user_id = ? ORDER BY is_pinned DESC, create_time ASC, friend_user_id ASC LIMIT ? OFFSET ?`, owner, limit, offset)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	rowsOut, err := scanFriends(rows)
	return total, rowsOut, err
}

func (s *Store) FriendUserIDs(ctx context.Context, owner string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT friend_user_id FROM friend WHERE owner_user_id = ? ORDER BY is_pinned DESC, create_time ASC, friend_user_id ASC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) PageBlacks(ctx context.Context, owner string, offset, limit int) (int, []model.Black, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM black WHERE owner_user_id = ?`, owner).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.db.QueryContext(ctx, blackSelect+` WHERE owner_user_id = ? ORDER BY create_time ASC, block_user_id ASC LIMIT ? OFFSET ?`, owner, limit, offset)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	rowsOut, err := scanBlacks(rows)
	return total, rowsOut, err
}

func (s *Store) ListFriends(ctx context.Context, owner string) ([]model.Friend, error) {
	rows, err := s.db.QueryContext(ctx, friendSelect+` WHERE owner_user_id = ?`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFriends(rows)
}

func (s *Store) FriendsByIDs(ctx context.Context, owner string, ids []string) ([]model.Friend, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, owner)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, friendSelect+` WHERE owner_user_id = ? AND friend_user_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found, err := scanFriends(rows)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.Friend, len(found))
	for _, row := range found {
		byID[row.FriendUserID] = row
	}
	out := make([]model.Friend, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

const friendSelect = `SELECT owner_user_id, friend_user_id, remark, create_time, add_source, operator_user_id, is_pinned FROM friend`

const blackSelect = `SELECT owner_user_id, block_user_id, create_time, add_source, operator_user_id FROM black`

func scanFriends(rows *sql.Rows) ([]model.Friend, error) {
	var out []model.Friend
	for rows.Next() {
		var row model.Friend
		var pinned int
		var created sql.NullTime
		if err := rows.Scan(&row.OwnerUserID, &row.FriendUserID, &row.Remark, &created, &row.AddSource, &row.OperatorUserID, &pinned); err != nil {
			return nil, err
		}
		row.IsPinned = pinned == 1
		if created.Valid {
			row.CreateTime = created.Time
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func scanBlacks(rows *sql.Rows) ([]model.Black, error) {
	var out []model.Black
	for rows.Next() {
		var row model.Black
		var created sql.NullTime
		if err := rows.Scan(&row.OwnerUserID, &row.BlockUserID, &created, &row.AddSource, &row.OperatorUserID); err != nil {
			return nil, err
		}
		if created.Valid {
			row.CreateTime = created.Time
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) maxVersion(ctx context.Context, table, owner string) (model.VersionHead, error) {
	var head model.VersionHead
	err := s.db.QueryRowContext(ctx, `SELECT id, version FROM `+table+` WHERE owner_user_id = ?`, owner).Scan(&head.ID, &head.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.VersionHead{}, nil
	}
	return head, err
}

func (s *Store) changes(ctx context.Context, table, owner string, version uint64, limit int) ([]model.VersionLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT elem_id, state, version FROM `+table+` WHERE owner_user_id = ? AND version > ? ORDER BY version LIMIT ?`, owner, version, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.VersionLog
	for rows.Next() {
		var row model.VersionLog
		if err := rows.Scan(&row.ElemID, &row.State, &row.Version); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func incrVersion(ctx context.Context, tx *sql.Tx, headTable, logTable, owner string, ids []string, state int32, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	id, err := newVersionID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+headTable+` (id, owner_user_id, version, deleted, last_update) VALUES (?, ?, 0, 0, ?) ON DUPLICATE KEY UPDATE owner_user_id = owner_user_id`, id, owner, now.UTC()); err != nil {
		return err
	}
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM `+headTable+` WHERE owner_user_id = ? FOR UPDATE`, owner).Scan(&version); err != nil {
		return err
	}
	for _, elem := range ids {
		version++
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO `+logTable+` (owner_user_id, elem_id, state, version, last_update)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE state = VALUES(state), version = VALUES(version), last_update = VALUES(last_update)`,
			owner, elem, state, version, now.UTC()); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE `+headTable+` SET version = ?, last_update = ? WHERE owner_user_id = ?`, version, now.UTC(), owner)
	return err
}

func newVersionID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
