package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"

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
		SELECT `+userColumns+`
		FROM user WHERE user_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateProfile writes only the columns set on the patch. Email and password stay unchanged.
func (s *Store) UpdateProfile(ctx context.Context, userID string, patch model.UserPatch) error {
	sets := make([]string, 0, 16)
	args := make([]any, 0, 16)
	if patch.Nickname != nil {
		sets = append(sets, "nickname = ?")
		args = append(args, *patch.Nickname)
	}
	if patch.AvatarURL != nil {
		sets = append(sets, "avatar_url = ?")
		args = append(args, *patch.AvatarURL)
	}
	if patch.AppMangerLevel != nil {
		sets = append(sets, "app_manger_level = ?")
		args = append(args, *patch.AppMangerLevel)
	}
	if patch.GlobalRecvMsgOpt != nil {
		sets = append(sets, "global_recv_msg_opt = ?")
		args = append(args, *patch.GlobalRecvMsgOpt)
	}
	if patch.IsActive != nil {
		sets = append(sets, "is_active = ?")
		args = append(args, *patch.IsActive)
	}
	if patch.AreaCode != nil {
		sets = append(sets, "area_code = ?")
		args = append(args, *patch.AreaCode)
	}
	if patch.PhoneNumber != nil {
		sets = append(sets, "phone_number = ?")
		args = append(args, nullIfEmpty(*patch.PhoneNumber))
	}
	if patch.Account != nil {
		sets = append(sets, "account = ?")
		args = append(args, nullIfEmpty(*patch.Account))
	}
	if patch.Gender != nil {
		sets = append(sets, "gender = ?")
		args = append(args, *patch.Gender)
	}
	if patch.ClearBirth {
		sets = append(sets, "birth_time = ?")
		args = append(args, nil)
	} else if patch.BirthTime != nil {
		sets = append(sets, "birth_time = ?")
		args = append(args, patch.BirthTime.UTC())
	}
	if patch.Level != nil {
		sets = append(sets, "level = ?")
		args = append(args, *patch.Level)
	}
	if patch.AllowVibration != nil {
		sets = append(sets, "allow_vibration = ?")
		args = append(args, *patch.AllowVibration)
	}
	if patch.AllowBeep != nil {
		sets = append(sets, "allow_beep = ?")
		args = append(args, *patch.AllowBeep)
	}
	if patch.AllowAddFriend != nil {
		sets = append(sets, "allow_add_friend = ?")
		args = append(args, *patch.AllowAddFriend)
	}
	if patch.RegisterType != nil {
		sets = append(sets, "register_type = ?")
		args = append(args, *patch.RegisterType)
	}
	if patch.Ex != nil {
		sets = append(sets, "ex = ?")
		args = append(args, *patch.Ex)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP(3)")
	args = append(args, userID)
	res, err := s.db.ExecContext(ctx, `UPDATE user SET `+strings.Join(sets, ", ")+` WHERE user_id = ?`, args...)
	if err != nil {
		var me *mysqldriver.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return ErrAccountTaken
		}
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

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
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
