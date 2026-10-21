package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

const (
	memberVersionHead = "group_member_version"
	memberVersionLog  = "group_member_version_log"
	joinVersionHead   = "group_join_version"
	joinVersionLog    = "group_join_version_log"
)

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
