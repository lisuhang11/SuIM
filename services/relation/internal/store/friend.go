package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

func (s *Store) CheckIn(ctx context.Context, user1, user2 string) (bool, bool, error) {
	in1, err := s.isFriend(ctx, user1, user2)
	if err != nil {
		return false, false, err
	}
	in2, err := s.isFriend(ctx, user2, user1)
	if err != nil {
		return false, false, err
	}
	return in1, in2, nil
}

func (s *Store) isFriend(ctx context.Context, owner, friend string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM friend WHERE owner_user_id = ? AND friend_user_id = ?`, owner, friend).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// AddFriendRequest inserts a pending request. A pending duplicate is rejected.
// A refused or previously agreed request is reopened, so the same pair can apply again.
func (s *Store) AddFriendRequest(ctx context.Context, from, to, msg string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var result int32
	err = tx.QueryRowContext(ctx, `SELECT handle_result FROM friend_request WHERE from_user_id = ? AND to_user_id = ? FOR UPDATE`, from, to).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO friend_request (
				from_user_id, to_user_id, handle_result, req_msg, create_time, handler_user_id, handle_msg, handle_time
			) VALUES (?, ?, ?, ?, ?, '', '', NULL)`,
			from, to, model.HandlePending, msg, now.UTC())
		if err != nil {
			if isDuplicate(err) {
				return ErrRequestRepeated
			}
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if result == model.HandlePending {
		return ErrRequestRepeated
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE friend_request
		SET handle_result = ?, req_msg = ?, create_time = ?, handler_user_id = '', handle_msg = '', handle_time = NULL
		WHERE from_user_id = ? AND to_user_id = ?`,
		model.HandlePending, msg, now.UTC(), from, to)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AgreeFriendRequest(ctx context.Context, from, to, handleMsg string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockPending(ctx, tx, from, to); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE friend_request
		SET handle_result = ?, handle_msg = ?, handler_user_id = ?, handle_time = ?
		WHERE from_user_id = ? AND to_user_id = ?`,
		model.HandleAgree, handleMsg, to, now.UTC(), from, to); err != nil {
		return err
	}
	if err := insertFriend(ctx, tx, from, to, now, from); err != nil {
		return err
	}
	if err := insertFriend(ctx, tx, to, from, now, from); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RefuseFriendRequest(ctx context.Context, from, to, handleMsg string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockPending(ctx, tx, from, to); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE friend_request
		SET handle_result = ?, handle_msg = ?, handler_user_id = ?, handle_time = ?
		WHERE from_user_id = ? AND to_user_id = ?`,
		model.HandleRefuse, handleMsg, to, now.UTC(), from, to); err != nil {
		return err
	}
	return tx.Commit()
}

func lockPending(ctx context.Context, tx *sql.Tx, from, to string) error {
	var result int32
	err := tx.QueryRowContext(ctx, `SELECT handle_result FROM friend_request WHERE from_user_id = ? AND to_user_id = ? FOR UPDATE`, from, to).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRequestNotFound
	}
	if err != nil {
		return err
	}
	if result != model.HandlePending {
		return ErrRequestHandled
	}
	return nil
}

func insertFriend(ctx context.Context, tx *sql.Tx, owner, friend string, now time.Time, operator string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO friend (owner_user_id, friend_user_id, remark, create_time, add_source, operator_user_id, is_pinned)
		VALUES (?, ?, '', ?, ?, ?, 0)
		ON DUPLICATE KEY UPDATE owner_user_id = owner_user_id`,
		owner, friend, now.UTC(), model.AddSourceApply, operator)
	return err
}

func (s *Store) DeleteFriend(ctx context.Context, owner, friend string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM friend WHERE owner_user_id = ? AND friend_user_id = ?`, owner, friend)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFriend
	}
	return nil
}

func (s *Store) UpdateFriend(ctx context.Context, owner, friend string, remark *string, pinned *bool) error {
	ok, err := s.isFriend(ctx, owner, friend)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFriend
	}
	sets := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if remark != nil {
		sets = append(sets, "remark = ?")
		args = append(args, *remark)
	}
	if pinned != nil {
		value := 0
		if *pinned {
			value = 1
		}
		sets = append(sets, "is_pinned = ?")
		args = append(args, value)
	}
	args = append(args, owner, friend)
	_, err = s.db.ExecContext(ctx, `UPDATE friend SET `+strings.Join(sets, ", ")+` WHERE owner_user_id = ? AND friend_user_id = ?`, args...)
	return err
}

func isDuplicate(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
