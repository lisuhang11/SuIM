package repository

import (
	"context"
	"errors"

	"group/internal/types"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IncrJoinVersion bumps a user's joined-group list version and appends changelog rows.
// Uses the repository's current DB handle (works inside WithinTransaction).
func (r *groupRepository) IncrJoinVersion(ctx context.Context, userID string, groupIDs []string, state int8) error {
	if userID == "" || len(groupIDs) == 0 {
		return nil
	}
	return incrJoinVersionTx(r.db.WithContext(ctx), userID, groupIDs, state)
}

func incrJoinVersionTx(tx *gorm.DB, userID string, groupIDs []string, state int8) error {
	var ver types.JoinGroupVersion
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&ver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ver = types.JoinGroupVersion{
			UserID:    userID,
			VersionID: uuid.NewString(),
			Version:   0,
		}
		if err := tx.Create(&ver).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&ver).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	ver.Version++
	if err := tx.Model(&types.JoinGroupVersion{}).
		Where("user_id = ?", userID).
		Update("version", ver.Version).Error; err != nil {
		return err
	}

	logs := make([]*types.JoinGroupVersionLog, 0, len(groupIDs))
	for _, gid := range groupIDs {
		if gid == "" {
			continue
		}
		logs = append(logs, &types.JoinGroupVersionLog{
			UserID:  userID,
			Version: ver.Version,
			GroupID: gid,
			State:   state,
		})
	}
	if len(logs) == 0 {
		return nil
	}
	return tx.Create(&logs).Error
}

// EnsureJoinGroupVersion creates a zero watermark row if missing.
func (r *groupRepository) EnsureJoinGroupVersion(ctx context.Context, userID string) (*types.JoinGroupVersion, error) {
	var ver types.JoinGroupVersion
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&ver).Error
	if err == nil {
		return &ver, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	ver = types.JoinGroupVersion{
		UserID:    userID,
		VersionID: uuid.NewString(),
		Version:   0,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&ver).Error; err != nil {
		return nil, err
	}
	return r.GetJoinGroupVersion(ctx, userID)
}

// GetJoinGroupVersion returns the watermark; missing row returns zero values.
func (r *groupRepository) GetJoinGroupVersion(ctx context.Context, userID string) (*types.JoinGroupVersion, error) {
	var ver types.JoinGroupVersion
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&ver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &types.JoinGroupVersion{UserID: userID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &ver, nil
}

// ListJoinGroupVersionLogs returns logs with version in (afterVersion, maxVersion].
func (r *groupRepository) ListJoinGroupVersionLogs(ctx context.Context, userID string, afterVersion, maxVersion uint64) ([]*types.JoinGroupVersionLog, error) {
	var logs []*types.JoinGroupVersionLog
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND version > ? AND version <= ?", userID, afterVersion, maxVersion).
		Order("version ASC, id ASC").
		Find(&logs).Error
	return logs, err
}

// ListJoinGroupIDs returns ordered group IDs the user has joined.
func (r *groupRepository) ListJoinGroupIDs(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&types.GroupMember{}).
		Where("user_id = ?", userID).
		Order("join_time DESC").
		Pluck("group_id", &ids).Error
	return ids, err
}
