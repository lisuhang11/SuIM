package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

func (s *Store) CheckBlack(ctx context.Context, user1, user2 string) (bool, bool, error) {
	in1, err := s.isBlack(ctx, user1, user2)
	if err != nil {
		return false, false, err
	}
	in2, err := s.isBlack(ctx, user2, user1)
	if err != nil {
		return false, false, err
	}
	return in1, in2, nil
}

func (s *Store) isBlack(ctx context.Context, owner, block string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM black WHERE owner_user_id = ? AND block_user_id = ?`, owner, block).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) CreateBlack(ctx context.Context, owner, block, operator string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO black (owner_user_id, block_user_id, create_time, add_source, operator_user_id)
		VALUES (?, ?, ?, 0, ?)`,
		owner, block, now.UTC(), operator); err != nil {
		if isDuplicate(err) {
			return ErrAlreadyBlocked
		}
		return err
	}
	if err := incrVersion(ctx, tx, blackVersionHead, blackVersionLog, owner, []string{block}, model.VersionInsert, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteBlack(ctx context.Context, owner, block string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM black WHERE owner_user_id = ? AND block_user_id = ?`, owner, block)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		if err := incrVersion(ctx, tx, blackVersionHead, blackVersionLog, owner, []string{block}, model.VersionDelete, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
