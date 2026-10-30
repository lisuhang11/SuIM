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

type Store struct {
	db *sql.DB
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
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE conversation ADD COLUMN ex VARCHAR(1024) NOT NULL DEFAULT ''"); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1060
}
