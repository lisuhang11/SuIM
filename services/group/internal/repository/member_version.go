package repository

import (
	"context"
	"errors"

	"group/internal/types"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IncrMemberVersion bumps a group's member-list version and appends changelog rows.
// entityIDs are member userIDs or special markers (group/sort change).
func (r *groupRepository) IncrMemberVersion(ctx context.Context, groupID string, entityIDs []string, state int8) error {
	if groupID == "" || len(entityIDs) == 0 {
		return nil
	}
	return incrMemberVersionTx(r.db.WithContext(ctx), groupID, entityIDs, state)
}

func incrMemberVersionTx(tx *gorm.DB, groupID string, entityIDs []string, state int8) error {
	var ver types.GroupMemberVersion
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("group_id = ?", groupID).
		First(&ver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ver = types.GroupMemberVersion{
			GroupID:   groupID,
			VersionID: uuid.NewString(),
			Version:   0,
		}
		if err := tx.Create(&ver).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("group_id = ?", groupID).
			First(&ver).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	ver.Version++
	if err := tx.Model(&types.GroupMemberVersion{}).
		Where("group_id = ?", groupID).
		Update("version", ver.Version).Error; err != nil {
		return err
	}

	logs := make([]*types.GroupMemberVersionLog, 0, len(entityIDs))
	for _, eid := range entityIDs {
		logs = append(logs, &types.GroupMemberVersionLog{
			GroupID:  groupID,
			Version:  ver.Version,
			EntityID: eid,
			State:    state,
		})
	}
	return tx.Create(&logs).Error
}

// EnsureGroupMemberVersion creates a zero watermark row if missing.
func (r *groupRepository) EnsureGroupMemberVersion(ctx context.Context, groupID string) (*types.GroupMemberVersion, error) {
	var ver types.GroupMemberVersion
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).First(&ver).Error
	if err == nil {
		return &ver, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	ver = types.GroupMemberVersion{
		GroupID:   groupID,
		VersionID: uuid.NewString(),
		Version:   0,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&ver).Error; err != nil {
		return nil, err
	}
	return r.GetGroupMemberVersion(ctx, groupID)
}

// GetGroupMemberVersion returns the watermark; missing row returns zero values.
func (r *groupRepository) GetGroupMemberVersion(ctx context.Context, groupID string) (*types.GroupMemberVersion, error) {
	var ver types.GroupMemberVersion
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).First(&ver).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &types.GroupMemberVersion{GroupID: groupID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &ver, nil
}

// ListGroupMemberVersionLogs returns logs with version in (afterVersion, maxVersion].
func (r *groupRepository) ListGroupMemberVersionLogs(ctx context.Context, groupID string, afterVersion, maxVersion uint64) ([]*types.GroupMemberVersionLog, error) {
	var logs []*types.GroupMemberVersionLog
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND version > ? AND version <= ?", groupID, afterVersion, maxVersion).
		Order("version ASC, id ASC").
		Find(&logs).Error
	return logs, err
}

// ListMembersByIDs loads members for the given user IDs in a group.
func (r *groupRepository) ListMembersByIDs(ctx context.Context, groupID string, userIDs []string) ([]*types.GroupMember, error) {
	if groupID == "" || len(userIDs) == 0 {
		return nil, nil
	}
	var members []*types.GroupMember
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND user_id IN ?", groupID, userIDs).
		Find(&members).Error
	return members, err
}

// ListOrderedMemberUserIDs returns member userIDs ordered by role then join time.
func (r *groupRepository) ListOrderedMemberUserIDs(ctx context.Context, groupID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).
		Model(&types.GroupMember{}).
		Where("group_id = ?", groupID).
		Order("role_level DESC, join_time ASC").
		Pluck("user_id", &ids).Error
	return ids, err
}
