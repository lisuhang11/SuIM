package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

const (
	memberVersionHead = "group_member_version"
	memberVersionLog  = "group_member_version_log"
	joinVersionHead   = "group_join_version"
	joinVersionLog    = "group_join_version_log"
)

func (s *Store) JoinVersion(ctx context.Context, userID string) (model.VersionHead, error) {
	return s.versionHead(ctx, joinVersionHead, userID)
}

func (s *Store) JoinChanges(ctx context.Context, userID string, version uint64, limit int) ([]model.VersionLog, error) {
	return s.versionChanges(ctx, joinVersionLog, userID, version, limit)
}

func (s *Store) MemberVersion(ctx context.Context, groupID string) (model.VersionHead, error) {
	return s.versionHead(ctx, memberVersionHead, groupID)
}

func (s *Store) MemberChanges(ctx context.Context, groupID string, version uint64, limit int) ([]model.VersionLog, error) {
	return s.versionChanges(ctx, memberVersionLog, groupID, version, limit)
}

func (s *Store) versionHead(ctx context.Context, table, did string) (model.VersionHead, error) {
	var head model.VersionHead
	err := s.db.QueryRowContext(ctx, `SELECT version_id, version FROM `+table+` WHERE d_id = ?`, did).Scan(&head.ID, &head.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.VersionHead{}, nil
	}
	return head, err
}

func (s *Store) versionChanges(ctx context.Context, table, did string, version uint64, limit int) ([]model.VersionLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e_id, state, version FROM `+table+` WHERE d_id = ? AND version > ? ORDER BY version LIMIT ?`, did, version, limit)
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

func incrMembers(ctx context.Context, tx *sql.Tx, groupID string, userIDs []string, state int32, now time.Time) error {
	return incrVersion(ctx, tx, memberVersionHead, memberVersionLog, groupID, userIDs, state, now)
}

func incrJoin(ctx context.Context, tx *sql.Tx, userID, groupID string, state int32, now time.Time) error {
	return incrVersion(ctx, tx, joinVersionHead, joinVersionLog, userID, []string{groupID}, state, now)
}

func incrVersion(ctx context.Context, tx *sql.Tx, head, log, did string, eids []string, state int32, now time.Time) error {
	if did == "" || len(eids) == 0 {
		return nil
	}
	vid, err := newVersionID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+head+` (version_id, d_id, version, deleted, last_update) VALUES (?, ?, 0, 0, ?) ON DUPLICATE KEY UPDATE d_id = d_id`, vid, did, now.UTC()); err != nil {
		return err
	}
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM `+head+` WHERE d_id = ? FOR UPDATE`, did).Scan(&version); err != nil {
		return err
	}
	for _, eid := range eids {
		version++
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO `+log+` (d_id, e_id, state, version, last_update)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE state = VALUES(state), version = VALUES(version), last_update = VALUES(last_update)`,
			did, eid, state, version, now.UTC()); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE `+head+` SET version = ?, last_update = ? WHERE d_id = ?`, version, now.UTC(), did)
	return err
}

func newVersionID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
