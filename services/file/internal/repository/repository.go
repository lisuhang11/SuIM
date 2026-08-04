package repository

import (
	"context"
	"fileservice/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Create(ctx context.Context, f *types.File) error {
	return r.db.WithContext(ctx).Create(f).Error
}
func (r *Repository) Get(ctx context.Context, id string) (*types.File, error) {
	var f types.File
	err := r.db.WithContext(ctx).Where("file_id = ? AND status <> ?", id, types.StatusDeleted).First(&f).Error
	return &f, err
}
func (r *Repository) FindDuplicate(ctx context.Context, owner, hash, purpose string, size int64, now time.Time) (*types.File, error) {
	var f types.File
	err := r.db.WithContext(ctx).Where("owner_id = ? AND sha256 = ? AND purpose = ? AND size = ? AND status = ? AND expires_at > ?", owner, hash, purpose, size, types.StatusAvailable, now).First(&f).Error
	return &f, err
}

func (r *Repository) ActivateAvatar(ctx context.Context, fileID, targetType, targetID string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old types.AvatarBinding
		err := tx.Where("target_type = ? AND target_id = ?", targetType, targetID).First(&old).Error
		if err == nil && old.FileID != fileID {
			if err := tx.Model(&types.File{}).Where("file_id = ?", old.FileID).Update("expires_at", now.Add(7*24*time.Hour)).Error; err != nil {
				return err
			}
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		binding := &types.AvatarBinding{TargetType: targetType, TargetID: targetID, FileID: fileID}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "target_type"}, {Name: "target_id"}},
			DoUpdates: clause.Assignments(map[string]any{"file_id": fileID, "updated_at": now}),
		}).Create(binding).Error; err != nil {
			return err
		}
		return tx.Model(&types.File{}).Where("file_id = ?", fileID).Update("expires_at", now.AddDate(100, 0, 0)).Error
	})
}
func (r *Repository) UsedBytes(ctx context.Context, owner string) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&types.File{}).Where("owner_id = ? AND status IN ?", owner, []string{types.StatusPending, types.StatusAvailable}).Select("COALESCE(SUM(size), 0)").Scan(&total).Error
	return total, err
}
func (r *Repository) MarkAvailable(ctx context.Context, id string, expiry time.Time) error {
	return r.db.WithContext(ctx).Model(&types.File{}).Where("file_id = ? AND status = ?", id, types.StatusPending).Updates(map[string]any{"status": types.StatusAvailable, "expires_at": expiry}).Error
}
func (r *Repository) Bind(ctx context.Context, fileID, conversationID string, expiry time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		b := &types.Binding{FileID: fileID, ConversationID: conversationID, ExpiresAt: expiry}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "file_id"}, {Name: "conversation_id"}}, DoUpdates: clause.Assignments(map[string]any{"expires_at": expiry})}).Create(b).Error; err != nil {
			return err
		}
		return tx.Model(&types.File{}).Where("file_id = ? AND expires_at < ?", fileID, expiry).Update("expires_at", expiry).Error
	})
}
func (r *Repository) ConversationExists(ctx context.Context, userID, conversationID string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("conversation").Where("owner_user_id = ? AND conversation_id = ?", userID, conversationID).Count(&n).Error
	return n > 0, err
}
func (r *Repository) CanAccess(ctx context.Context, f *types.File, userID string, now time.Time) (bool, error) {
	if f.OwnerID == userID || (f.Purpose == types.PurposeAvatar && f.Status == types.StatusAvailable) {
		return true, nil
	}
	var n int64
	err := r.db.WithContext(ctx).Table("file_binding b").Joins("JOIN conversation c ON c.conversation_id = b.conversation_id AND c.owner_user_id = ?", userID).Where("b.file_id = ? AND b.expires_at > ?", f.FileID, now).Count(&n).Error
	return n > 0, err
}
func (r *Repository) BindingCount(ctx context.Context, fileID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&types.Binding{}).Where("file_id = ?", fileID).Count(&n).Error
	return n, err
}
func (r *Repository) Expired(ctx context.Context, now time.Time, limit int) ([]types.File, error) {
	var fs []types.File
	err := r.db.WithContext(ctx).Where("(status = ? AND expires_at < ?) OR (status = ? AND expires_at < ?)", types.StatusPending, now, types.StatusAvailable, now).Limit(limit).Find(&fs).Error
	return fs, err
}
func (r *Repository) MarkDeleted(ctx context.Context, id string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("file_id = ?", id).Delete(&types.Binding{}).Error; err != nil {
			return err
		}
		return tx.Model(&types.File{}).Where("file_id = ?", id).Updates(map[string]any{"status": types.StatusDeleted, "deleted_at": now}).Error
	})
}
