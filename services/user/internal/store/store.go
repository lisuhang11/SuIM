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
	for _, stmt := range []string{
		"ALTER TABLE `user` ADD COLUMN area_code VARCHAR(16) NOT NULL DEFAULT '' COMMENT '手机区号'",
		"ALTER TABLE `user` ADD COLUMN phone_number VARCHAR(32) NULL COMMENT '手机号'",
		"ALTER TABLE `user` ADD COLUMN account VARCHAR(64) NULL COMMENT '账号'",
		"ALTER TABLE `user` ADD COLUMN gender INT NOT NULL DEFAULT 0 COMMENT '性别：0未知 1男 2女'",
		"ALTER TABLE `user` ADD COLUMN birth_time DATETIME(3) NULL COMMENT '生日'",
		"ALTER TABLE `user` ADD COLUMN level INT NOT NULL DEFAULT 0 COMMENT '等级'",
		"ALTER TABLE `user` ADD COLUMN allow_vibration INT NOT NULL DEFAULT 1 COMMENT '是否允许震动：1允许 2不允许'",
		"ALTER TABLE `user` ADD COLUMN allow_beep INT NOT NULL DEFAULT 1 COMMENT '是否允许提示音：1允许 2不允许'",
		"ALTER TABLE `user` ADD COLUMN allow_add_friend INT NOT NULL DEFAULT 1 COMMENT '是否允许加好友：1允许 2不允许'",
		"ALTER TABLE `user` ADD COLUMN register_type INT NOT NULL DEFAULT 0 COMMENT '注册方式'",
		"ALTER TABLE `user` ADD COLUMN ex VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '扩展字段'",
		"ALTER TABLE `user` ADD UNIQUE INDEX uk_account (account)",
		"ALTER TABLE `user` ADD UNIQUE INDEX uk_phone (area_code, phone_number)",
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil && !isDuplicateColumn(err) && !isDuplicateKey(err) {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1060
}

func isDuplicateKey(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1061
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

const userColumns = `user_id, email, password_hash, nickname, avatar_url,
	app_manger_level, global_recv_msg_opt, is_active, create_time,
	area_code, phone_number, account, gender, birth_time, level,
	allow_vibration, allow_beep, allow_add_friend, register_type, ex`

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.getUser(ctx, `SELECT `+userColumns+` FROM user WHERE email = ?`, email)
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	return s.getUser(ctx, `SELECT `+userColumns+` FROM user WHERE user_id = ?`, userID)
}

func (s *Store) getUser(ctx context.Context, query, arg string) (*model.User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, query, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func scanUser(sc scanner) (*model.User, error) {
	var u model.User
	var active int
	var phone, account sql.NullString
	var birth sql.NullTime
	err := sc.Scan(
		&u.UserID, &u.Email, &u.PasswordHash, &u.Nickname, &u.AvatarURL,
		&u.AppMangerLevel, &u.GlobalRecvMsgOpt, &active, &u.CreateTime,
		&u.AreaCode, &phone, &account, &u.Gender, &birth, &u.Level,
		&u.AllowVibration, &u.AllowBeep, &u.AllowAddFriend, &u.RegisterType, &u.Ex,
	)
	if err != nil {
		return nil, err
	}
	u.IsActive = active == 1
	u.PhoneNumber = phone.String
	u.Account = account.String
	if birth.Valid {
		u.BirthTime = birth.Time
	}
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
