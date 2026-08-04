package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fileservice/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Store struct {
	internal, public *minio.Client
	bucket           string
}

func New(ctx context.Context, cfg *config.Config) (*Store, error) {
	creds := credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, "")
	in, err := minio.New(cfg.MinioEndpoint, &minio.Options{Creds: creds, Secure: cfg.MinioUseSSL})
	if err != nil {
		return nil, err
	}
	pub, err := minio.New(cfg.MinioPublicEndpoint, &minio.Options{Creds: creds, Secure: cfg.MinioUseSSL})
	if err != nil {
		return nil, err
	}
	ok, err := in.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		return nil, err
	}
	if !ok {
		if err := in.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return &Store{internal: in, public: pub, bucket: cfg.MinioBucket}, nil
}
func (s *Store) PresignPut(ctx context.Context, key, contentType string, expiry time.Duration) (string, map[string]string, error) {
	u, err := s.public.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", nil, err
	}
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return u.String(), headers, nil
}
func (s *Store) PresignGet(ctx context.Context, key, name, contentType string, expiry time.Duration) (string, error) {
	q := url.Values{}
	disposition := "attachment"
	if strings.HasPrefix(contentType, "image/") {
		disposition = "inline"
	}
	q.Set("response-content-disposition", disposition+"; filename*=UTF-8''"+url.PathEscape(name))
	if contentType != "" {
		q.Set("response-content-type", contentType)
	}
	u, err := s.public.PresignedGetObject(ctx, s.bucket, key, expiry, q)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
func (s *Store) Stat(ctx context.Context, key string) (minio.ObjectInfo, error) {
	return s.internal.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
}
func (s *Store) Inspect(ctx context.Context, key string) (string, string, []byte, error) {
	o, err := s.internal.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "", "", nil, err
	}
	defer o.Close()
	h := sha256.New()
	buf := make([]byte, 512)
	n, readErr := io.ReadFull(o, buf)
	if readErr != nil && readErr != io.ErrUnexpectedEOF {
		return "", "", nil, readErr
	}
	if _, err := h.Write(buf[:n]); err != nil {
		return "", "", nil, err
	}
	if _, err := io.Copy(h, o); err != nil {
		return "", "", nil, err
	}
	return hex.EncodeToString(h.Sum(nil)), http.DetectContentType(buf[:n]), buf[:n], nil
}
func (s *Store) Delete(ctx context.Context, key string) error {
	return s.internal.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
