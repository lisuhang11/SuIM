package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
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
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE `object` MODIFY `storage_key` VARCHAR(255) NOT NULL COMMENT 'MinIO对象键'"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
