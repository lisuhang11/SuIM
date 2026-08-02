package repository

import (
	"context"
	"errors"

	"relation/internal/types"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IncrVersion bumps owner friend list version and appends changelog rows.
// friendUserIDs are the entities that changed; state is insert/delete/update.
func (r *relationRepository) IncrVersion(ctx context.Context, ownerUserID string, friendUserIDs []string, state int8, isSort bool) error {
	if ownerUserID == "" || len(friendUserIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return incrVersionTx(tx, ownerUserID, friendUserIDs, state, isSort)
	})
}

func incrVersionTx(tx *gorm.DB, ownerUserID string, friendUserIDs []string, state int8, isSort bool) error {
	var ver types.FriendVersion
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("owner_user_id = ?", ownerUserID).
		First(&ver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ver = types.FriendVersion{
			OwnerUserID: ownerUserID,
			VersionID:   uuid.NewString(),
			Version:     0,
		}
		if err := tx.Create(&ver).Error; err != nil {
			return err
		}
		// Re-lock after create.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_user_id = ?", ownerUserID).
			First(&ver).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	ver.Version++
	if err := tx.Model(&types.FriendVersion{}).
		Where("owner_user_id = ?", ownerUserID).
		Update("version", ver.Version).Error; err != nil {
		return err
	}

	logs := make([]*types.FriendVersionLog, 0, len(friendUserIDs))
	for _, fid := range friendUserIDs {
		if fid == "" {
			continue
		}
		logs = append(logs, &types.FriendVersionLog{
			OwnerUserID:  ownerUserID,
			Version:      ver.Version,
			FriendUserID: fid,
			State:        state,
			IsSort:       isSort,
		})
	}
	if len(logs) == 0 {
		return nil
	}
	return tx.Create(&logs).Error
}

// EnsureFriendVersion creates a zero watermark row if missing.
func (r *relationRepository) EnsureFriendVersion(ctx context.Context, ownerUserID string) (*types.FriendVersion, error) {
	var ver types.FriendVersion
	err := r.db.WithContext(ctx).Where("owner_user_id = ?", ownerUserID).First(&ver).Error
	if err == nil {
		return &ver, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	ver = types.FriendVersion{
		OwnerUserID: ownerUserID,
		VersionID:   uuid.NewString(),
		Version:     0,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&ver).Error; err != nil {
		return nil, err
	}
	return r.GetFriendVersion(ctx, ownerUserID)
}

// GetFriendVersion returns the watermark row; missing row returns zero values with empty VersionID.
func (r *relationRepository) GetFriendVersion(ctx context.Context, ownerUserID string) (*types.FriendVersion, error) {
	var ver types.FriendVersion
	err := r.db.WithContext(ctx).Where("owner_user_id = ?", ownerUserID).First(&ver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &types.FriendVersion{OwnerUserID: ownerUserID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &ver, nil
}

// ListFriendVersionLogs returns logs with version in (afterVersion, maxVersion].
func (r *relationRepository) ListFriendVersionLogs(ctx context.Context, ownerUserID string, afterVersion, maxVersion uint64) ([]*types.FriendVersionLog, error) {
	var logs []*types.FriendVersionLog
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND version > ? AND version <= ?", ownerUserID, afterVersion, maxVersion).
		Order("version ASC, id ASC").
		Find(&logs).Error
	return logs, err
}

// ListFriendUserIDs returns ordered friend IDs for owner (pinned first).
func (r *relationRepository) ListFriendUserIDs(ctx context.Context, ownerUserID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&types.Friend{}).
		Where("owner_user_id = ?", ownerUserID).
		Order("is_pinned DESC, create_time DESC").
		Pluck("friend_user_id", &ids).Error
	return ids, err
}

// ListFriendsByIDs loads friend rows for the given friend user IDs.
func (r *relationRepository) ListFriendsByIDs(ctx context.Context, ownerUserID string, friendUserIDs []string) ([]*types.Friend, error) {
	if len(friendUserIDs) == 0 {
		return nil, nil
	}
	var friends []*types.Friend
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND friend_user_id IN ?", ownerUserID, friendUserIDs).
		Find(&friends).Error
	return friends, err
}

// FindOwnerUserIDsWhoFriended returns owners who have friendUserID in their friend list.
func (r *relationRepository) FindOwnerUserIDsWhoFriended(ctx context.Context, friendUserID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&types.Friend{}).
		Where("friend_user_id = ?", friendUserID).
		Pluck("owner_user_id", &ids).Error
	return ids, err
}
