package logic

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/services/third/internal/blob"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

func (s *Server) reuse(ctx context.Context, req *thirdpb.InitiateMultipartUploadReq, hash string) (*thirdpb.InitiateMultipartUploadResp, bool, error) {
	existing, err := s.objects.FindByHash(ctx, hash)
	if errors.Is(err, model.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if existing.Size != req.GetSize() {
		return nil, false, errorcode.ErrThirdHashMismatch.WithMsg("hash exists with a different size")
	}
	size, err := s.parts.Stat(ctx, existing.StorageKey)
	if errors.Is(err, model.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if size != existing.Size {
		return nil, false, errorcode.ErrThirdHashMismatch.WithMsg("stored object size mismatch")
	}
	if err := s.bind(ctx, &model.Object{
		Name:        req.GetName(),
		UserID:      req.GetUserId(),
		Hash:        hash,
		StorageKey:  existing.StorageKey,
		Size:        existing.Size,
		ContentType: req.GetContentType(),
		Cause:       req.GetCause(),
	}); err != nil {
		return nil, false, err
	}
	return &thirdpb.InitiateMultipartUploadResp{Url: req.GetUrlPrefix() + req.GetName()}, true, nil
}

func (s *Server) finishParts(ctx context.Context, sess *model.Session, sums []string) error {
	size, err := s.parts.Stat(ctx, sess.ObjectKey)
	if err == nil {
		if size != sess.Size {
			return errorcode.ErrThirdHashMismatch.WithMsg("stored object size mismatch")
		}
		_ = s.parts.Abort(ctx, sess.ObjectKey, sess.StoreUploadID)
		return nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return err
	}
	listed, err := s.parts.List(ctx, sess.ObjectKey, sess.StoreUploadID)
	if errors.Is(err, model.ErrNotFound) {
		return errorcode.ErrThirdInvalidArgs.WithMsg("part is missing")
	}
	if err != nil {
		return err
	}
	byNumber := make(map[int]model.StoredPart, len(listed))
	for _, part := range listed {
		byNumber[part.Number] = part
	}
	ordered := make([]model.StoredPart, sess.PartCount)
	for i, sum := range sums {
		part, ok := byNumber[i+1]
		if !ok {
			return errorcode.ErrThirdInvalidArgs.WithMsg("part is missing")
		}
		want := partLen(sess.Size, sess.PartSize, sess.PartCount, i+1)
		if part.Size != want {
			return errorcode.ErrThirdPartSize
		}
		if blob.NormalizeETag(part.ETag) != sum {
			return errorcode.ErrThirdHashMismatch
		}
		ordered[i] = part
	}
	return s.parts.Finish(ctx, sess.ObjectKey, sess.StoreUploadID, ordered)
}

func (s *Server) completeExisting(ctx context.Context, req *thirdpb.CompleteMultipartUploadReq, hash string) (*thirdpb.CompleteMultipartUploadResp, error) {
	obj, err := s.objects.FindByName(ctx, req.GetName())
	if errors.Is(err, model.ErrNotFound) || (err == nil && (obj.UserID != req.GetUserId() || obj.Hash != hash)) {
		return nil, errorcode.ErrThirdNotFound
	}
	if err != nil {
		return nil, err
	}
	return &thirdpb.CompleteMultipartUploadResp{Url: req.GetUrlPrefix() + req.GetName()}, nil
}

func (s *Server) bind(ctx context.Context, obj *model.Object) error {
	err := s.objects.InsertObject(ctx, obj)
	if err == nil {
		return nil
	}
	if !errors.Is(err, model.ErrNameTaken) {
		return err
	}
	existing, gerr := s.objects.FindByName(ctx, obj.Name)
	if gerr != nil {
		return gerr
	}
	if existing.Hash == obj.Hash && existing.Size == obj.Size {
		return nil
	}
	return errorcode.ErrThirdNameConflict
}

func (s *Server) signParts(ctx context.Context, sess *model.Session, from, n int) (*thirdpb.AuthSignParts, error) {
	numbers := make([]int, n)
	for i := 0; i < n; i++ {
		numbers[i] = from + i
	}
	parts, err := s.signNumbers(ctx, sess, numbers)
	if err != nil {
		return nil, err
	}
	return &thirdpb.AuthSignParts{Parts: parts}, nil
}

func (s *Server) signNumbers(ctx context.Context, sess *model.Session, numbers []int) ([]*thirdpb.SignPart, error) {
	ttl := sess.ExpireAt.Sub(s.now())
	if ttl <= 0 {
		return nil, errorcode.ErrThirdNotFound.WithMsg("upload expired")
	}
	parts := make([]*thirdpb.SignPart, 0, len(numbers))
	for _, number := range numbers {
		raw, err := s.parts.Sign(ctx, sess.ObjectKey, sess.StoreUploadID, number, ttl)
		if err != nil {
			return nil, err
		}
		parts = append(parts, &thirdpb.SignPart{PartNumber: int32(number), Url: raw})
	}
	return parts, nil
}

func objectKey(hash string) string {
	return "objects/" + hash
}

func newUploadID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func normalizeHash(h string) (string, error) {
	h = strings.ToLower(strings.TrimSpace(h))
	if !validID(h) {
		return "", errorcode.ErrThirdInvalidArgs.WithMsg("hash must be 32 hex characters")
	}
	return h, nil
}

func normalizeSums(parts []string) ([]string, error) {
	if len(parts) == 0 {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("parts is empty")
	}
	out := make([]string, len(parts))
	for i, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if !validID(p) {
			return nil, errorcode.ErrThirdHashMismatch
		}
		out[i] = p
	}
	return out, nil
}

func joinHash(parts []string) string {
	sum := md5.Sum([]byte(strings.Join(parts, ",")))
	return hex.EncodeToString(sum[:])
}

func checkName(userID, name string) error {
	if userID == "" {
		return errorcode.ErrThirdInvalidArgs.WithMsg("user_id is required")
	}
	if name == "" || strings.TrimSpace(name) == "" || len(name) > 512 || !utf8.ValidString(name) {
		return errorcode.ErrThirdInvalidArgs.WithMsg("name is invalid")
	}
	if name[0] == '/' || strings.Contains(name, `\`) || strings.ContainsRune(name, 0) {
		return errorcode.ErrThirdInvalidArgs.WithMsg("name is invalid")
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errorcode.ErrThirdInvalidArgs.WithMsg("name is invalid")
		}
	}
	if !strings.HasPrefix(name, userID+"/") {
		return errorcode.ErrNoPermission.WithMsg("name must start with the user id")
	}
	return nil
}

func checkMeta(contentType, cause, prefix string) error {
	if len(contentType) > 255 || strings.ContainsRune(contentType, 0) {
		return errorcode.ErrThirdInvalidArgs.WithMsg("content_type is invalid")
	}
	if len(cause) > 64 || strings.ContainsRune(cause, 0) {
		return errorcode.ErrThirdInvalidArgs.WithMsg("cause is invalid")
	}
	if len(prefix) > 1024 || strings.ContainsAny(prefix, "\r\n\x00") {
		return errorcode.ErrThirdInvalidArgs.WithMsg("url_prefix is invalid")
	}
	return nil
}

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
