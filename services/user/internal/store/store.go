package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already registered")
)

type Store struct {
	db  *sql.DB
	rdb redis.UniversalClient
}

func Open(ctx context.Context, mysqlDSN string, mysqlMaxOpen, mysqlMaxIdle int, rdb redis.UniversalClient) (*Store, error) {
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		return nil, err
	}
	if mysqlMaxOpen > 0 {
		db.SetMaxOpenConns(mysqlMaxOpen)
	}
	if mysqlMaxIdle > 0 {
		db.SetMaxIdleConns(mysqlMaxIdle)
	}
	db.SetConnMaxLifetime(time.Hour)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Store{db: db, rdb: rdb}, nil
}

func (s *Store) Close() error {
	var err error
	if s.rdb != nil {
		err = s.rdb.Close()
	}
	if s.db != nil {
		if derr := s.db.Close(); derr != nil && err == nil {
			err = derr
		}
	}
	return err
}

func (s *Store) Migrate(ctx context.Context, script string) error {
	for _, stmt := range strings.Split(script, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user (
			user_id, email, password_hash, nickname, avatar_url,
			app_manger_level, global_recv_msg_opt, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.UserID, u.Email, u.PasswordHash, u.Nickname, u.AvatarURL,
		u.AppMangerLevel, u.GlobalRecvMsgOpt, u.IsActive,
	)
	if err != nil {
		var me *mysqldriver.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return ErrEmailTaken
		}
		return err
	}
	row, err := s.GetUserByID(ctx, u.UserID)
	if err != nil {
		return err
	}
	*u = *row
	return nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.getUser(ctx, `SELECT user_id, email, password_hash, nickname, avatar_url,
		app_manger_level, global_recv_msg_opt, is_active, create_time
		FROM user WHERE email = ?`, email)
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	return s.getUser(ctx, `SELECT user_id, email, password_hash, nickname, avatar_url,
		app_manger_level, global_recv_msg_opt, is_active, create_time
		FROM user WHERE user_id = ?`, userID)
}

func (s *Store) getUser(ctx context.Context, query, arg string) (*model.User, error) {
	var u model.User
	var active int
	err := s.db.QueryRowContext(ctx, query, arg).Scan(
		&u.UserID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL,
		&u.AppMangerLevel, &u.GlobalRecvMsgOpt, &active, &u.CreateTime,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.IsActive = active == 1
	return &u, nil
}

// StartSession revokes every live token of this user on this platform, then stores the new token.
func (s *Store) StartSession(ctx context.Context, tok *model.Token) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	live, err := listLive(ctx, tx, tok.UserID, tok.PlatformID)
	if err != nil {
		return nil, err
	}
	if err := revokeAllTx(ctx, tx, tok.UserID, tok.PlatformID); err != nil {
		return nil, err
	}
	if err := insertTokenTx(ctx, tx, tok); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	hashes := tokenHashes(live)
	if err := s.dropSnapshots(ctx, live); err != nil {
		return hashes, err
	}
	if err := s.putSnapshot(ctx, tok); err != nil {
		return hashes, err
	}
	return hashes, nil
}

// RevokeSession marks every live token of the user on one platform revoked and drops their Redis snapshots.
func (s *Store) RevokeSession(ctx context.Context, userID string, platformID int32) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	live, err := listLive(ctx, tx, userID, platformID)
	if err != nil {
		return nil, err
	}
	if err := revokeAllTx(ctx, tx, userID, platformID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tokenHashes(live), s.dropSnapshots(ctx, live)
}

func tokenHashes(tokens []model.Token) []string {
	if len(tokens) == 0 {
		return nil
	}
	hashes := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if tok.Hash != "" {
			hashes = append(hashes, tok.Hash)
		}
	}
	return hashes
}

// Lookup returns the token row. Redis is the hot snapshot; a miss reads MySQL and backfills.
func (s *Store) Lookup(ctx context.Context, hash string) (*model.Token, error) {
	if ok, err := s.denied(ctx, hash); err != nil {
		return nil, err
	} else if ok {
		tok, err := s.getTokenMySQL(ctx, hash)
		if err != nil {
			return nil, err
		}
		tok.IsRevoked = true
		return tok, nil
	}
	snap, err := s.getSnapshot(ctx, hash)
	if err != nil {
		return nil, err
	}
	if snap != nil {
		return snap, nil
	}
	tok, err := s.getTokenMySQL(ctx, hash)
	if err != nil {
		return nil, err
	}
	if tok.IsRevoked {
		if err := s.deny(ctx, tok); err != nil {
			return nil, err
		}
		return tok, nil
	}
	if err := s.putSnapshot(ctx, tok); err != nil {
		return nil, err
	}
	return tok, nil
}

func revokeAllTx(ctx context.Context, tx *sql.Tx, userID string, platformID int32) error {
	_, err := tx.ExecContext(ctx, `UPDATE auth_tokens SET is_revoked = 1 WHERE user_id = ? AND platform_id = ? AND is_revoked = 0`, userID, platformID)
	return err
}

func listLive(ctx context.Context, tx *sql.Tx, userID string, platformID int32) ([]model.Token, error) {
	return listLiveQuery(ctx, tx, `
		SELECT id, user_id, platform_id, token, token_hash, token_type, expires_at, is_revoked
		FROM auth_tokens WHERE user_id = ? AND platform_id = ? AND is_revoked = 0 FOR UPDATE`, userID, platformID)
}

func listLiveUser(ctx context.Context, tx *sql.Tx, userID string) ([]model.Token, error) {
	return listLiveQuery(ctx, tx, `
		SELECT id, user_id, platform_id, token, token_hash, token_type, expires_at, is_revoked
		FROM auth_tokens WHERE user_id = ? AND is_revoked = 0 FOR UPDATE`, userID)
}

func listLiveQuery(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]model.Token, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Token
	for rows.Next() {
		tok, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *tok)
	}
	return out, rows.Err()
}

func insertTokenTx(ctx context.Context, tx *sql.Tx, tok *model.Token) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO auth_tokens (id, user_id, platform_id, token, token_hash, token_type, expires_at, is_revoked)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
		tok.ID, tok.UserID, tok.PlatformID, tok.Raw, tok.Hash, tok.Type, tok.ExpiresAt.UTC())
	return err
}

func (s *Store) getTokenMySQL(ctx context.Context, hash string) (*model.Token, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, platform_id, token, token_hash, token_type, expires_at, is_revoked
		FROM auth_tokens WHERE token_hash = ?`, hash)
	tok, err := scanToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return tok, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanToken(sc scanner) (*model.Token, error) {
	var tok model.Token
	var revoked int
	if err := sc.Scan(&tok.ID, &tok.UserID, &tok.PlatformID, &tok.Raw, &tok.Hash, &tok.Type, &tok.ExpiresAt, &revoked); err != nil {
		return nil, err
	}
	tok.IsRevoked = revoked == 1
	return &tok, nil
}
