package store

import (
	"context"
	"database/sql"
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

func (s *Store) FindByHash(ctx context.Context, hash string) (*model.Object, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT name, user_id, hash, storage_key, size, content_type, cause, create_time
		FROM object WHERE hash = ? LIMIT 1`, hash)
	return scanObject(row)
}

func (s *Store) FindByName(ctx context.Context, name string) (*model.Object, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT name, user_id, hash, storage_key, size, content_type, cause, create_time
		FROM object WHERE name = ?`, name)
	return scanObject(row)
}

func (s *Store) InsertObject(ctx context.Context, obj *model.Object) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO object (name, user_id, hash, storage_key, size, content_type, cause)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		obj.Name, obj.UserID, obj.Hash, obj.StorageKey, obj.Size, obj.ContentType, obj.Cause)
	if isDuplicate(err) {
		return model.ErrNameTaken
	}
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanObject(row rowScanner) (*model.Object, error) {
	var obj model.Object
	err := row.Scan(&obj.Name, &obj.UserID, &obj.Hash, &obj.StorageKey, &obj.Size, &obj.ContentType, &obj.Cause, &obj.CreateTime)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func isDuplicate(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
