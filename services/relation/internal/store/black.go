package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO black (owner_user_id, block_user_id, create_time, add_source, operator_user_id)
		VALUES (?, ?, ?, 0, ?)`,
		owner, block, now.UTC(), operator)
	if isDuplicate(err) {
		return ErrAlreadyBlocked
	}
	return err
}

func (s *Store) DeleteBlack(ctx context.Context, owner, block string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM black WHERE owner_user_id = ? AND block_user_id = ?`, owner, block)
	return err
}
