// Package repository 提供群成员数据操作的 GORM 实现。
package repository

import (
	"context"
	"errors"

	"group/internal/types"

	"gorm.io/gorm"
)

// CreateMember 持久化群成员记录。
func (r *groupRepository) CreateMember(ctx context.Context, m *types.GroupMember) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetMember 根据群组和用户 ID 查询成员，不存在则返回 ErrMemberNotFound。
func (r *groupRepository) GetMember(ctx context.Context, groupID, userID string) (*types.GroupMember, error) {
	var m types.GroupMember
	if err := r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	return &m, nil
}

// UpdateMember 写入所有成员字段（包括零值）。
func (r *groupRepository) UpdateMember(ctx context.Context, m *types.GroupMember) error {
	return r.db.WithContext(ctx).Model(&types.GroupMember{}).Where("group_id = ? AND user_id = ?", m.GroupID, m.UserID).Select("*").Updates(m).Error
}

// DeleteMember 删除单个群成员记录。
func (r *groupRepository) DeleteMember(ctx context.Context, groupID, userID string) error {
	return r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&types.GroupMember{}).Error
}

// DeleteMembersByGroup 删除群组的所有成员记录。
func (r *groupRepository) DeleteMembersByGroup(ctx context.Context, groupID string) error {
	return r.db.WithContext(ctx).Where("group_id = ?", groupID).Delete(&types.GroupMember{}).Error
}

// ListMembers 分页查询群成员列表，按角色降序、加入时间升序排列。
func (r *groupRepository) ListMembers(ctx context.Context, groupID string, offset, limit int) ([]*types.GroupMember, int64, error) {
	var (
		members []*types.GroupMember
		total   int64
	)
	base := r.db.WithContext(ctx).Model(&types.GroupMember{}).Where("group_id = ?", groupID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base.Order("role_level DESC, join_time ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&members).Error; err != nil {
		return nil, 0, err
	}
	return members, total, nil
}

// ListMemberUserIDs 返回群全部成员 userID。
func (r *groupRepository) ListMemberUserIDs(ctx context.Context, groupID string) ([]string, error) {
	if groupID == "" {
		return nil, nil
	}
	var ids []string
	if err := r.db.WithContext(ctx).Model(&types.GroupMember{}).
		Where("group_id = ?", groupID).
		Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// MapGroupOwners 批量查询各群当前群主（role_level = owner）。
func (r *groupRepository) MapGroupOwners(ctx context.Context, groupIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(groupIDs))
	if len(groupIDs) == 0 {
		return out, nil
	}
	type row struct {
		GroupID string `gorm:"column:group_id"`
		UserID  string `gorm:"column:user_id"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&types.GroupMember{}).
		Select("group_id, user_id").
		Where("group_id IN ? AND role_level = ?", groupIDs, types.GroupMemberRoleOwner).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.GroupID != "" && r.UserID != "" {
			out[r.GroupID] = r.UserID
		}
	}
	return out, nil
}

// MapGroupMemberNum 批量统计各群成员数；缺失的 groupID 不出现在 map 中（视为 0）。
func (r *groupRepository) MapGroupMemberNum(ctx context.Context, groupIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(groupIDs))
	if len(groupIDs) == 0 {
		return out, nil
	}
	type row struct {
		GroupID string `gorm:"column:group_id"`
		Cnt     int64  `gorm:"column:cnt"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&types.GroupMember{}).
		Select("group_id, COUNT(*) AS cnt").
		Where("group_id IN ?", groupIDs).
		Group("group_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.GroupID] = r.Cnt
	}
	return out, nil
}

// MemberExists 判断指定用户在指定群组中是否已是成员。
func (r *groupRepository) MemberExists(ctx context.Context, groupID, userID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&types.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListGroupsOfUser 分页查询用户所属的群组列表。
func (r *groupRepository) ListGroupsOfUser(ctx context.Context, userID string, offset, limit int) ([]*types.GroupMember, int64, error) {
	var (
		members []*types.GroupMember
		total   int64
	)
	base := r.db.WithContext(ctx).Model(&types.GroupMember{}).Where("user_id = ?", userID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base.Order("join_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&members).Error; err != nil {
		return nil, 0, err
	}
	return members, total, nil
}
