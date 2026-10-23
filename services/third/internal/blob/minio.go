// Package blob stores multipart uploads in MinIO.
package blob

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

const maxPresign = 7 * 24 * time.Hour

// Config is the MinIO bucket that holds upload bytes.
type Config struct {
	Endpoint     string
	SignEndpoint string
	Bucket       string
	AccessKey    string
	SecretKey    string
}

// Minio is a multipart client. Sign URLs use SignEndpoint when it differs from Endpoint.
type Minio struct {
	core       *minio.Core
	sign       *minio.Core
	bucket     string
	publicBase string
}

func Open(ctx context.Context, cfg Config) (*Minio, error) {
	core, err := newCore(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	signEndpoint := cfg.SignEndpoint
	if signEndpoint == "" {
		signEndpoint = cfg.Endpoint
	}
	sign := core
	if signEndpoint != cfg.Endpoint {
		sign, err = newCore(signEndpoint, cfg.AccessKey, cfg.SecretKey)
		if err != nil {
			return nil, err
		}
	}
	exists, err := core.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := core.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("make bucket: %w", err)
		}
	}
	return &Minio{
		core:       core,
		sign:       sign,
		bucket:     cfg.Bucket,
		publicBase: strings.TrimRight(signEndpoint, "/"),
	}, nil
}

func newCore(rawURL, accessKey, secretKey string) (*minio.Core, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: u.Scheme == "https",
	})
	if err != nil {
		return nil, err
	}
	return &minio.Core{Client: client}, nil
}

// Create starts a multipart upload at key.
func (m *Minio) Create(ctx context.Context, key, contentType string) (string, error) {
	opts := minio.PutObjectOptions{ContentType: contentType}
	return m.core.NewMultipartUpload(ctx, m.bucket, key, opts)
}

// Sign returns a presigned PUT URL for one part.
func (m *Minio) Sign(ctx context.Context, key, uploadID string, part int, expire time.Duration) (string, error) {
	if expire <= 0 {
		return "", fmt.Errorf("presign expiry must be positive")
	}
	if expire > maxPresign {
		expire = maxPresign
	}
	params := url.Values{}
	params.Set("uploadId", uploadID)
	params.Set("partNumber", strconv.Itoa(part))
	u, err := m.sign.Presign(ctx, http.MethodPut, m.bucket, key, expire, params)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// List returns parts already stored for this upload.
func (m *Minio) List(ctx context.Context, key, uploadID string) ([]model.StoredPart, error) {
	var out []model.StoredPart
	marker := 0
	for {
		res, err := m.core.ListObjectParts(ctx, m.bucket, key, uploadID, marker, 1000)
		if err != nil {
			if isMissing(err) {
				return nil, model.ErrNotFound
			}
			return nil, err
		}
		for _, part := range res.ObjectParts {
			out = append(out, model.StoredPart{
				Number: part.PartNumber,
				Size:   part.Size,
				ETag:   part.ETag,
			})
			if part.PartNumber > marker {
				marker = part.PartNumber
			}
		}
		if !res.IsTruncated || res.NextPartNumberMarker <= marker {
			break
		}
		marker = res.NextPartNumberMarker
	}
	return out, nil
}

// Finish merges parts into the object at key. parts must use the ETags MinIO returned.
func (m *Minio) Finish(ctx context.Context, key, uploadID string, parts []model.StoredPart) error {
	sorted := append([]model.StoredPart(nil), parts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Number < sorted[j].Number })
	complete := make([]minio.CompletePart, len(sorted))
	for i, part := range sorted {
		complete[i] = minio.CompletePart{PartNumber: part.Number, ETag: part.ETag}
	}
	_, err := m.core.CompleteMultipartUpload(ctx, m.bucket, key, uploadID, complete, minio.PutObjectOptions{})
	return err
}

// Abort drops an unfinished multipart upload.
func (m *Minio) Abort(ctx context.Context, key, uploadID string) error {
	err := m.core.AbortMultipartUpload(ctx, m.bucket, key, uploadID)
	if isMissing(err) {
		return nil
	}
	return err
}

// Form builds a POST policy the client sends straight to MinIO.
func (m *Minio) Form(ctx context.Context, key string, size int64, contentType string, expires time.Time) (*model.FormCredential, error) {
	policy := minio.NewPostPolicy()
	if err := policy.SetBucket(m.bucket); err != nil {
		return nil, err
	}
	if err := policy.SetKey(key); err != nil {
		return nil, err
	}
	if err := policy.SetExpires(expires); err != nil {
		return nil, err
	}
	if size > 0 {
		if err := policy.SetContentLengthRange(0, size); err != nil {
			return nil, err
		}
	}
	if err := policy.SetSuccessStatusAction("200"); err != nil {
		return nil, err
	}
	if contentType != "" {
		if err := policy.SetContentType(contentType); err != nil {
			return nil, err
		}
	}
	_, form, err := m.sign.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return nil, err
	}
	return &model.FormCredential{
		URL:          m.publicBase + "/" + m.bucket,
		File:         "file",
		FormData:     form,
		Expires:      expires,
		SuccessCodes: []int{http.StatusOK},
	}, nil
}

// Head returns the size and ETag of a finished object.
func (m *Minio) Head(ctx context.Context, key string) (int64, string, error) {
	info, err := m.core.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isMissing(err) {
			return 0, "", model.ErrNotFound
		}
		return 0, "", err
	}
	return info.Size, info.ETag, nil
}

// Stat returns the size of a finished object.
func (m *Minio) Stat(ctx context.Context, key string) (int64, error) {
	info, err := m.core.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isMissing(err) {
			return 0, model.ErrNotFound
		}
		return 0, err
	}
	return info.Size, nil
}

func isMissing(err error) bool {
	resp := minio.ToErrorResponse(err)
	switch resp.Code {
	case "NoSuchKey", "NoSuchUpload", "NoSuchBucket":
		return true
	default:
		return resp.StatusCode == http.StatusNotFound
	}
}

// NormalizeETag strips quotes so a part ETag can be compared with an MD5 hex.
func NormalizeETag(etag string) string {
	return strings.ToLower(strings.Trim(etag, `"`))
}
