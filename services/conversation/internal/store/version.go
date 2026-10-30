package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
)

const (
	versionHead = "conversation_version"
	versionLog  = "conversation_version_log"
)

func (s *Store) Version(ctx context.Context, owner string) (model.VersionHead, error) {
	var head model.VersionHead
	err := s.db.QueryRowContext(ctx, `SELECT version_id, version FROM `+versionHead+` WHERE d_id = ?`, owner).Scan(&head.ID, &head.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return model.VersionHead{}, nil
	}
	return head, err
}

func (s *Store) Changes(ctx context.Context, owner string, version uint64, limit int) ([]model.VersionLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e_id, state, version FROM `+versionLog+` WHERE d_id = ? AND version > ? ORDER BY version LIMIT ?`, owner, version, limit)
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

func incrConversation(ctx context.Context, tx *sql.Tx, owner, conversationID string, state int32, now time.Time) error {
	if owner == "" || conversationID == "" {
		return nil
	}
	vid, err := newVersionID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+versionHead+` (version_id, d_id, version, deleted, last_update) VALUES (?, ?, 0, 0, ?) ON DUPLICATE KEY UPDATE d_id = d_id`, vid, owner, now.UTC()); err != nil {
		return err
	}
	var version uint64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM `+versionHead+` WHERE d_id = ? FOR UPDATE`, owner).Scan(&version); err != nil {
		return err
	}
	version++
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO `+versionLog+` (d_id, e_id, state, version, last_update)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE state = VALUES(state), version = VALUES(version), last_update = VALUES(last_update)`,
		owner, conversationID, state, version, now.UTC()); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE `+versionHead+` SET version = ?, last_update = ? WHERE d_id = ?`, version, now.UTC(), owner)
	return err
}

func newVersionID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
