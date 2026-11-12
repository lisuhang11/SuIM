package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

// FindExpired returns object rows older than expireAt, optionally filtered by cause.
func (s *Store) FindExpired(ctx context.Context, expireAt time.Time, causes []string, limit int) ([]*model.Object, error) {
	if limit <= 0 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if len(causes) == 0 {
		rows, err = s.db.QueryContext(ctx, `
SELECT name, user_id, hash, storage_key, size, content_type, cause, create_time
FROM object
WHERE create_time < ?
ORDER BY create_time ASC
LIMIT ?`, expireAt.UTC(), limit)
	} else {
		ph := placeholders(len(causes))
		args := make([]any, 0, len(causes)+2)
		args = append(args, expireAt.UTC())
		for _, c := range causes {
			args = append(args, c)
		}
		args = append(args, limit)
		rows, err = s.db.QueryContext(ctx, `
SELECT name, user_id, hash, storage_key, size, content_type, cause, create_time
FROM object
WHERE create_time < ? AND cause IN (`+ph+`)
ORDER BY create_time ASC
LIMIT ?`, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Object
	for rows.Next() {
		obj, err := scanObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

// DeleteByName removes one object row by name.
func (s *Store) DeleteByName(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM object WHERE name = ?`, name)
	return err
}

// CountByStorageKey counts rows that still point at the same storage key.
func (s *Store) CountByStorageKey(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return 0, nil
	}
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM object WHERE storage_key = ?`, key).Scan(&n)
	return n, err
}

func placeholders(n int) string {
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}
