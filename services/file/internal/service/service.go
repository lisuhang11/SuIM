package service

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	pb "SuIM/proto/filepb"
	"fileservice/internal/config"
	"fileservice/internal/repository"
	"fileservice/internal/storage"
	"fileservice/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

const maxAvatarSize int64 = 5 << 20

type Service struct {
	repo  *repository.Repository
	store *storage.Store
	cfg   *config.Config
}

func New(repo *repository.Repository, store *storage.Store, cfg *config.Config) *Service {
	return &Service{repo: repo, store: store, cfg: cfg}
}

func (s *Service) Initiate(ctx context.Context, userID, name, contentType, hash, purpose string, size int64) (*pb.InitiateUploadResp, error) {
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	if purpose == "" {
		purpose = types.PurposeAttachment
	}
	if purpose != types.PurposeAttachment && purpose != types.PurposeAvatar {
		return nil, invalid("unsupported file purpose")
	}
	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || len([]rune(name)) > 255 {
		return nil, invalid("invalid file name")
	}
	if size <= 0 || size > s.cfg.MaxFileSize {
		return nil, invalid(fmt.Sprintf("file size must be between 1 and %d bytes", s.cfg.MaxFileSize))
	}
	hash = strings.ToLower(strings.TrimSpace(hash))
	if hash != "" && !sha256Pattern.MatchString(hash) {
		return nil, invalid("sha256 must be 64 hexadecimal characters")
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(name))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if purpose == types.PurposeAvatar {
		if size > maxAvatarSize {
			return nil, invalid("avatar size must not exceed 5 MiB")
		}
		if !avatarContentType(contentType) {
			return nil, invalid("avatars must be JPEG, PNG, or WebP")
		}
	}
	if forbidden(contentType, filepath.Ext(name)) {
		return nil, invalid("this file type is not allowed")
	}
	now := time.Now()
	if hash != "" {
		if f, err := s.repo.FindDuplicate(ctx, userID, hash, purpose, size, now); err == nil {
			return &pb.InitiateUploadResp{File: toProto(f), AlreadyUploaded: true}, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, internal(err)
		}
	}
	used, err := s.repo.UsedBytes(ctx, userID)
	if err != nil {
		return nil, internal(err)
	}
	if used+size > s.cfg.UserQuota {
		return nil, resource("storage quota exceeded")
	}
	id := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(name))
	key := path.Join("users", userID, purpose, now.Format("2006/01/02"), id+ext)
	uploadExpires := now.Add(s.cfg.UploadExpiry)
	fileCategory := category(contentType)
	if purpose == types.PurposeAvatar {
		fileCategory = "avatar"
	}
	f := &types.File{FileID: id, OwnerID: userID, ObjectKey: key, OriginalName: name, ContentType: contentType, Size: size, SHA256: hash, Category: fileCategory, Purpose: purpose, Status: types.StatusPending, UploadExpiresAt: uploadExpires, ExpiresAt: now.Add(s.cfg.PendingRetention)}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, internal(err)
	}
	u, headers, err := s.store.PresignPut(ctx, key, contentType, s.cfg.UploadExpiry)
	if err != nil {
		return nil, internal(err)
	}
	return &pb.InitiateUploadResp{File: toProto(f), UploadUrl: u, Headers: headers, UploadExpiresAt: uploadExpires.UnixMilli()}, nil
}

func (s *Service) Complete(ctx context.Context, userID, fileID string) (*types.File, error) {
	f, err := s.owned(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	if f.Status == types.StatusAvailable {
		return f, nil
	}
	if time.Now().After(f.UploadExpiresAt) {
		return nil, failed("upload session expired")
	}
	info, err := s.store.Stat(ctx, f.ObjectKey)
	if err != nil {
		return nil, failed("uploaded object not found")
	}
	if info.Size != f.Size {
		return nil, failed("uploaded file size mismatch")
	}
	sum, detectedType, prefix, err := s.store.Inspect(ctx, f.ObjectKey)
	if err != nil {
		return nil, internal(err)
	}
	if forbidden(detectedType, filepath.Ext(f.OriginalName)) || executableMagic(prefix) {
		_ = s.store.Delete(ctx, f.ObjectKey)
		return nil, failed("uploaded file content is not allowed")
	}
	if f.Purpose == types.PurposeAvatar && !avatarContentType(detectedType) {
		_ = s.store.Delete(ctx, f.ObjectKey)
		return nil, failed("uploaded avatar content must be JPEG, PNG, or WebP")
	}
	if f.SHA256 != "" && sum != f.SHA256 {
		_ = s.store.Delete(ctx, f.ObjectKey)
		return nil, failed("uploaded file checksum mismatch")
	}
	if err := s.repo.MarkAvailable(ctx, f.FileID, time.Now().Add(s.cfg.PendingRetention)); err != nil {
		return nil, internal(err)
	}
	return s.repo.Get(ctx, f.FileID)
}

func (s *Service) ActivateAvatar(ctx context.Context, userID, fileID, targetType, targetID string) (*types.File, error) {
	f, err := s.owned(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	if f.Status != types.StatusAvailable || f.Purpose != types.PurposeAvatar {
		return nil, failed("file is not an available avatar")
	}
	if targetType != "user" && targetType != "group" {
		return nil, invalid("target_type must be user or group")
	}
	if targetID == "" || (targetType == "user" && targetID != userID) {
		return nil, permission("avatar target is not allowed")
	}
	if err := s.repo.ActivateAvatar(ctx, fileID, targetType, targetID, time.Now()); err != nil {
		return nil, internal(err)
	}
	return s.repo.Get(ctx, fileID)
}
func (s *Service) Bind(ctx context.Context, userID, fileID, conversationID string) (*types.File, error) {
	f, err := s.owned(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	if f.Status != types.StatusAvailable {
		return nil, failed("file is not available")
	}
	ok, err := s.repo.ConversationExists(ctx, userID, conversationID)
	if err != nil {
		return nil, internal(err)
	}
	if !ok {
		return nil, permission("sender is not a member of the conversation")
	}
	if err := s.repo.Bind(ctx, fileID, conversationID, time.Now().Add(s.cfg.FileRetention)); err != nil {
		return nil, internal(err)
	}
	return s.repo.Get(ctx, fileID)
}
func (s *Service) Get(ctx context.Context, userID, fileID string) (*types.File, error) {
	f, err := s.repo.Get(ctx, fileID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, internal(err)
	}
	ok, err := s.repo.CanAccess(ctx, f, userID, time.Now())
	if err != nil {
		return nil, internal(err)
	}
	if !ok {
		return nil, permission("file access denied")
	}
	return f, nil
}
func (s *Service) Download(ctx context.Context, userID, fileID string) (*types.File, string, time.Time, error) {
	f, err := s.Get(ctx, userID, fileID)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if f.Status != types.StatusAvailable {
		return nil, "", time.Time{}, failed("file is not available")
	}
	expiry := time.Now().Add(s.cfg.DownloadExpiry)
	u, err := s.store.PresignGet(ctx, f.ObjectKey, f.OriginalName, f.ContentType, s.cfg.DownloadExpiry)
	if err != nil {
		return nil, "", time.Time{}, internal(err)
	}
	return f, u, expiry, nil
}
func (s *Service) Delete(ctx context.Context, userID, fileID string) error {
	f, err := s.owned(ctx, userID, fileID)
	if err != nil {
		return err
	}
	n, err := s.repo.BindingCount(ctx, fileID)
	if err != nil {
		return internal(err)
	}
	if n > 0 {
		return failed("a file referenced by a conversation cannot be deleted directly")
	}
	if err := s.store.Delete(ctx, f.ObjectKey); err != nil {
		return internal(err)
	}
	return s.repo.MarkDeleted(ctx, fileID, time.Now())
}
func (s *Service) Cleanup(ctx context.Context) (int, error) {
	fs, err := s.repo.Expired(ctx, time.Now(), 100)
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range fs {
		if err := s.store.Delete(ctx, fs[i].ObjectKey); err != nil {
			continue
		}
		if err := s.repo.MarkDeleted(ctx, fs[i].FileID, time.Now()); err == nil {
			count++
		}
	}
	return count, nil
}
func (s *Service) owned(ctx context.Context, userID, fileID string) (*types.File, error) {
	f, err := s.repo.Get(ctx, fileID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, internal(err)
	}
	if f.OwnerID != userID {
		return nil, permission("only the file owner can perform this operation")
	}
	return f, nil
}

func toProto(f *types.File) *pb.FileInfo {
	if f == nil {
		return nil
	}
	return &pb.FileInfo{FileId: f.FileID, OwnerId: f.OwnerID, Name: f.OriginalName, ContentType: f.ContentType, Size: f.Size, Sha256: f.SHA256, Category: f.Category, Status: f.Status, CreatedAt: f.CreatedAt.UnixMilli(), ExpiresAt: f.ExpiresAt.UnixMilli(), Purpose: f.Purpose}
}
func avatarContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/webp"
}
func category(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	case contentType == "application/pdf" || strings.HasPrefix(contentType, "text/"):
		return "document"
	default:
		return "other"
	}
}
func forbidden(contentType, ext string) bool {
	switch strings.ToLower(ext) {
	case ".exe", ".dll", ".bat", ".cmd", ".com", ".msi", ".ps1", ".sh":
		return true
	}
	return contentType == "text/html" || contentType == "image/svg+xml"
}

func executableMagic(data []byte) bool {
	if len(data) >= 2 && string(data[:2]) == "MZ" {
		return true
	}
	if len(data) >= 4 {
		signature := string(data[:4])
		if signature == "\x7fELF" || signature == "\xfe\xed\xfa\xce" || signature == "\xfe\xed\xfa\xcf" || signature == "\xcf\xfa\xed\xfe" || signature == "\xce\xfa\xed\xfe" {
			return true
		}
	}
	return len(data) >= 2 && string(data[:2]) == "#!"
}

type appError struct {
	kind, msg string
	cause     error
}

func (e *appError) Error() string { return e.msg }
func invalid(s string) error      { return &appError{kind: "invalid", msg: s} }
func resource(s string) error     { return &appError{kind: "resource", msg: s} }
func failed(s string) error       { return &appError{kind: "failed", msg: s} }
func permission(s string) error   { return &appError{kind: "permission", msg: s} }
func notFound() error             { return &appError{kind: "not_found", msg: "file not found"} }
func internal(e error) error {
	return &appError{kind: "internal", msg: "internal file service error", cause: e}
}
func ErrorKind(err error) string {
	var e *appError
	if errors.As(err, &e) {
		return e.kind
	}
	return "internal"
}
func ToProto(f *types.File) *pb.FileInfo { return toProto(f) }
