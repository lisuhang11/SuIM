package store

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

func (s *Store) GetUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, email, password_hash, nickname, avatar_url,
			app_manger_level, global_recv_msg_opt, is_active, create_time
		FROM user WHERE user_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		var u model.User
		var active int
		if err := rows.Scan(
			&u.UserID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL,
			&u.AppMangerLevel, &u.GlobalRecvMsgOpt, &active, &u.CreateTime,
		); err != nil {
			return nil, err
		}
		u.IsActive = active == 1
		out = append(out, &u)
	}
	return out, rows.Err()
}

// UpdateProfile overwrites the public profile. Email and password stay unchanged.
func (s *Store) UpdateProfile(ctx context.Context, u *model.User) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE user
		SET nickname = ?, avatar_url = ?, app_manger_level = ?, global_recv_msg_opt = ?, is_active = ?,
			updated_at = CURRENT_TIMESTAMP(3)
		WHERE user_id = ?`,
		u.Nickname, u.AvatarURL, u.AppMangerLevel, u.GlobalRecvMsgOpt, u.IsActive, u.UserID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ChangePassword stores the new hash and revokes every live token of the user.
func (s *Store) ChangePassword(ctx context.Context, userID, passwordHash string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `UPDATE user SET password_hash = ? WHERE user_id = ?`, passwordHash, userID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	live, err := revokeUserTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := s.dropSnapshots(ctx, live); err != nil {
		return tokenHashes(live), err
	}
	return tokenHashes(live), nil
}

// DeleteUser removes the account and its tokens. Returned hashes are the live tokens that were dropped.
func (s *Store) DeleteUser(ctx context.Context, userID string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	live, err := listLiveUser(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_tokens WHERE user_id = ?`, userID); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM user WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := s.dropSnapshots(ctx, live); err != nil {
		return tokenHashes(live), err
	}
	return tokenHashes(live), nil
}

// RevokeUser marks every live token of the user revoked.
func (s *Store) RevokeUser(ctx context.Context, userID string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	live, err := revokeUserTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := s.dropSnapshots(ctx, live); err != nil {
		return tokenHashes(live), err
	}
	return tokenHashes(live), nil
}

func revokeUserTx(ctx context.Context, tx *sql.Tx, userID string) ([]model.Token, error) {
	live, err := listLiveUser(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_tokens SET is_revoked = 1 WHERE user_id = ? AND is_revoked = 0`, userID); err != nil {
		return nil, err
	}
	return live, nil
}
