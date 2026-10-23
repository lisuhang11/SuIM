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

func (s *Server) PartLimit(context.Context, *thirdpb.PartLimitReq) (*thirdpb.PartLimitResp, error) {
	return &thirdpb.PartLimitResp{
		MinPartSize: s.limits.MinPartSize,
		MaxPartSize: s.limits.MaxPartSize,
		MaxNumSize:  int32(s.limits.MaxNumSize),
	}, nil
}

func (s *Server) InitiateMultipartUpload(ctx context.Context, req *thirdpb.InitiateMultipartUploadReq) (*thirdpb.InitiateMultipartUploadResp, error) {
	hash, err := normalizeHash(req.GetHash())
	if err != nil {
		return nil, err
	}
	if err := checkName(req.GetUserId(), req.GetName()); err != nil {
		return nil, err
	}
	if err := checkMeta(req.GetContentType(), req.GetCause(), req.GetUrlPrefix()); err != nil {
		return nil, err
	}
	partSize, count, err := Plan(s.limits, req.GetSize())
	if err != nil {
		return nil, err
	}
	if req.GetPartSize() != 0 && req.GetPartSize() != partSize {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part_size does not match the server plan")
	}

	key := objectKey(hash)
	if resp, done, err := s.reuse(ctx, req, hash); done || err != nil {
		return resp, err
	}

	storeID, err := s.parts.Create(ctx, key, req.GetContentType())
	if err != nil {
		return nil, err
	}
	id, err := newUploadID()
	if err != nil {
		_ = s.parts.Abort(ctx, key, storeID)
		return nil, err
	}
	expireAt := s.now().Add(s.expire)
	sess := &model.Session{
		ID:            id,
		UserID:        req.GetUserId(),
		Hash:          hash,
		Size:          req.GetSize(),
		PartSize:      partSize,
		PartCount:     count,
		ContentType:   req.GetContentType(),
		Cause:         req.GetCause(),
		Name:          req.GetName(),
		ExpireAt:      expireAt,
		StoreUploadID: storeID,
		ObjectKey:     key,
	}
	if err := s.sessions.CreateSession(ctx, sess); err != nil {
		_ = s.parts.Abort(ctx, key, storeID)
		return nil, err
	}
	sign, err := s.signParts(ctx, sess, 1, signCount(req.GetMaxParts(), count))
	if err != nil {
		_ = s.sessions.DeleteSession(ctx, id)
		_ = s.parts.Abort(ctx, key, storeID)
		return nil, err
	}
	return &thirdpb.InitiateMultipartUploadResp{
		Upload: &thirdpb.UploadInfo{
			UploadId:   id,
			PartSize:   partSize,
			ExpireTime: expireAt.UnixMilli(),
			Sign:       sign,
		},
	}, nil
}

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

func (s *Server) AuthSign(ctx context.Context, req *thirdpb.AuthSignReq) (*thirdpb.AuthSignResp, error) {
	if req.GetUserId() == "" || !validID(req.GetUploadId()) {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("upload_id is invalid")
	}
	if len(req.GetPartNumbers()) == 0 || len(req.GetPartNumbers()) > maxSignBatch {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part_numbers is invalid")
	}
	sess, err := s.sessions.GetSession(ctx, req.GetUploadId())
	if errors.Is(err, model.ErrUploadNotFound) {
		return nil, errorcode.ErrThirdNotFound
	}
	if err != nil {
		return nil, err
	}
	if sess.UserID != req.GetUserId() {
		return nil, errorcode.ErrNoPermission
	}
	if !s.now().Before(sess.ExpireAt) {
		return nil, errorcode.ErrThirdNotFound.WithMsg("upload expired")
	}
	seen := make(map[int32]struct{}, len(req.GetPartNumbers()))
	numbers := make([]int, 0, len(req.GetPartNumbers()))
	for _, n := range req.GetPartNumbers() {
		if _, ok := seen[n]; ok {
			return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part number is repeated")
		}
		seen[n] = struct{}{}
		if n < 1 || int(n) > sess.PartCount {
			return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part number is out of range")
		}
		numbers = append(numbers, int(n))
	}
	signed, err := s.signNumbers(ctx, sess, numbers)
	if err != nil {
		return nil, err
	}
	return &thirdpb.AuthSignResp{Parts: signed}, nil
}

func (s *Server) CompleteMultipartUpload(ctx context.Context, req *thirdpb.CompleteMultipartUploadReq) (*thirdpb.CompleteMultipartUploadResp, error) {
	if err := checkName(req.GetUserId(), req.GetName()); err != nil {
		return nil, err
	}
	if err := checkMeta(req.GetContentType(), req.GetCause(), req.GetUrlPrefix()); err != nil {
		return nil, err
	}
	sums, err := normalizeSums(req.GetParts())
	if err != nil {
		return nil, err
	}
	hash := joinHash(sums)
	sess, err := s.sessions.GetSession(ctx, req.GetUploadId())
	if errors.Is(err, model.ErrUploadNotFound) {
		return s.completeExisting(ctx, req, hash)
	}
	if err != nil {
		return nil, err
	}
	if sess.UserID != req.GetUserId() {
		return nil, errorcode.ErrNoPermission
	}
	if sess.Name != req.GetName() {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("name does not match the upload")
	}
	if len(sums) != sess.PartCount || hash != sess.Hash {
		return nil, errorcode.ErrThirdHashMismatch
	}
	if err := s.finishParts(ctx, sess, sums); err != nil {
		return nil, err
	}
	contentType := req.GetContentType()
	if contentType == "" {
		contentType = sess.ContentType
	}
	cause := req.GetCause()
	if cause == "" {
		cause = sess.Cause
	}
	if err := s.bind(ctx, &model.Object{
		Name:        req.GetName(),
		UserID:      req.GetUserId(),
		Hash:        sess.Hash,
		StorageKey:  sess.ObjectKey,
		Size:        sess.Size,
		ContentType: contentType,
		Cause:       cause,
	}); err != nil {
		return nil, err
	}
	_ = s.sessions.DeleteSession(ctx, sess.ID)
	return &thirdpb.CompleteMultipartUploadResp{Url: req.GetUrlPrefix() + req.GetName()}, nil
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
