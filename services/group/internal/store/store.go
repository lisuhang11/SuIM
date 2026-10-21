package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

var (
	ErrNotFound        = errors.New("group not found")
	ErrNotMember       = errors.New("not a group member")
	ErrAlreadyMember   = errors.New("already in group")
	ErrRequestRepeated = errors.New("group request already pending")
	ErrDismissed       = errors.New("group dismissed")
)

type Store struct {
	db    *sql.DB
	cache *readCache
}

// UseCache puts a process-local cache in front of group and member reads.
func (s *Store) UseCache(cache *readCache) {
	s.cache = cache
}

func Open(ctx context.Context, mysqlDSN string, mysqlMaxOpen, mysqlMaxIdle int) (*Store, error) {
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
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
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
		"ALTER TABLE `group` ADD COLUMN ex VARCHAR(1024) NOT NULL DEFAULT ''",
		"ALTER TABLE group_member ADD COLUMN ex VARCHAR(1024) NOT NULL DEFAULT ''",
		"ALTER TABLE group_request ADD COLUMN ex VARCHAR(1024) NOT NULL DEFAULT ''",
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil && !isDuplicateColumn(err) {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func isDuplicate(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func isDuplicateColumn(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1060
}
