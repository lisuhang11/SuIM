// Package repository 提供群组服务的数据访问层，按功能聚合群组、成员和入群请求操作。
//
// groupRepository 聚合了群组、成员和入群请求三类数据操作于一体，
// 遵循按功能域组织仓库的模式，而非每个表一个仓库。
// 存储细节封装在此层，服务层仅依赖 interfaces.GroupRepository。
package repository

import (
	"context"
	"errors"

	"group/internal/types"
	"group/internal/types/interfaces"

	"gorm.io/gorm"
)

var (
	// ErrGroupNotFound 群组不存在错误。
	ErrGroupNotFound = errors.New("group not found")
	// ErrMemberNotFound 成员不存在错误。
	ErrMemberNotFound = errors.New("group member not found")
	// ErrRequestNotFound 入群请求不存在错误。
	ErrRequestNotFound = errors.New("group request not found")
)

// groupRepository 是 interfaces.GroupRepository 的 GORM 实现。
type groupRepository struct {
	db *gorm.DB
}

// NewGroupRepository 创建按功能聚合的群组仓库。
// 方法分别实现在 group.go、group_member.go、group_request.go 中，
// 均属于同一个 groupRepository 类型。
func NewGroupRepository(db *gorm.DB) interfaces.GroupRepository {
	return &groupRepository{db: db}
}

// CreateGroup 持久化群组记录。
func (r *groupRepository) CreateGroup(ctx context.Context, g *types.Group) error {
	return r.db.WithContext(ctx).Create(g).Error
}

// GetGroup 根据 ID 查询群组，不存在则返回 ErrGroupNotFound。
func (r *groupRepository) GetGroup(ctx context.Context, groupID string) (*types.Group, error) {
	var g types.Group
	if err := r.db.WithContext(ctx).Where("group_id = ?", groupID).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &g, nil
}

// UpdateGroup 写入所有群组字段，确保零值标记（如取消禁言状态）被正确持久化。
// GORM 的默认 struct 更新会跳过零值字段。
func (r *groupRepository) UpdateGroup(ctx context.Context, g *types.Group) error {
	return r.db.WithContext(ctx).Model(&types.Group{}).Where("group_id = ?", g.GroupID).Select("*").Updates(g).Error
}

// DeleteGroup 硬删除指定群组。
func (r *groupRepository) DeleteGroup(ctx context.Context, groupID string) error {
	return r.db.WithContext(ctx).Where("group_id = ?", groupID).Delete(&types.Group{}).Error
}

// ListGroupsByIDs 根据 ID 列表批量查询群组。
func (r *groupRepository) ListGroupsByIDs(ctx context.Context, groupIDs []string) ([]*types.Group, error) {
	if len(groupIDs) == 0 {
		return []*types.Group{}, nil
	}
	var groups []*types.Group
	if err := r.db.WithContext(ctx).Where("group_id IN ?", groupIDs).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}
