package store

import (
	"context"
	"strconv"
	"time"

	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

const uploadPrefix = "suim:third:upload:"

func (s *Store) CreateSession(ctx context.Context, sess *model.Session) error {
	ttl := time.Until(sess.ExpireAt) + time.Minute
	if ttl < time.Minute {
		ttl = time.Minute
	}
	fields := map[string]any{
		"user_id":         sess.UserID,
		"hash":            sess.Hash,
		"size":            sess.Size,
		"part_size":       sess.PartSize,
		"part_count":      sess.PartCount,
		"content_type":    sess.ContentType,
		"cause":           sess.Cause,
		"name":            sess.Name,
		"expire_at":       sess.ExpireAt.UnixMilli(),
		"store_upload_id": sess.StoreUploadID,
		"object_key":      sess.ObjectKey,
	}
	key := uploadPrefix + sess.ID
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) GetSession(ctx context.Context, id string) (*model.Session, error) {
	vals, err := s.rdb.HGetAll(ctx, uploadPrefix+id).Result()
	if err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		return nil, model.ErrUploadNotFound
	}
	size, err := strconv.ParseInt(vals["size"], 10, 64)
	if err != nil {
		return nil, err
	}
	partSize, err := strconv.ParseInt(vals["part_size"], 10, 64)
	if err != nil {
		return nil, err
	}
	partCount, err := strconv.Atoi(vals["part_count"])
	if err != nil {
		return nil, err
	}
	expireAt, err := strconv.ParseInt(vals["expire_at"], 10, 64)
	if err != nil {
		return nil, err
	}
	return &model.Session{
		ID:            id,
		UserID:        vals["user_id"],
		Hash:          vals["hash"],
		Size:          size,
		PartSize:      partSize,
		PartCount:     partCount,
		ContentType:   vals["content_type"],
		Cause:         vals["cause"],
		Name:          vals["name"],
		ExpireAt:      time.UnixMilli(expireAt),
		StoreUploadID: vals["store_upload_id"],
		ObjectKey:     vals["object_key"],
	}, nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	return s.rdb.Del(ctx, uploadPrefix+id).Err()
}
