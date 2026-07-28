// Package repository 提供 User 的 GORM 持久化实现。
package repository

import (
	"context"
	"errors"
	"user/internal/types"
	"user/internal/types/interfaces"

	"gorm.io/gorm"
)

var (
	// ErrUserNotFound 用户未找到错误。
	ErrUserNotFound = errors.New("user not found")
	// ErrUserAlreadyExists 用户已存在错误。
	ErrUserAlreadyExists = errors.New("user already exists")
)

// userRepository 实现 UserRepository 接口，基于 GORM 操作 User 表。
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建基于 GORM 的 UserRepository 实现。
func NewUserRepository(db *gorm.DB) interfaces.UserRepository {
	return &userRepository{db: db}
}

// CreateUser 创建新用户记录。
func (r *userRepository) CreateUser(ctx context.Context, user *types.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetUserByID 根据用户 ID 查询用户。
func (r *userRepository) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	var user types.User
	if err := r.db.WithContext(ctx).Where("user_id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetUsersByIDs 批量查询用户，返回 ID 到 User 的映射，不存在的 ID 不会出现在结果中。
func (r *userRepository) GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error) {
	out := make(map[string]*types.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var users []*types.User
	if err := r.db.WithContext(ctx).Where("user_id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, u := range users {
		out[u.UserID] = u
	}
	return out, nil
}

// GetUserByEmail 根据邮箱查询用户。
func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	var user types.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser 更新用户信息。
func (r *userRepository) UpdateUser(ctx context.Context, user *types.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// DeleteUser 根据 ID 删除用户。
func (r *userRepository) DeleteUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", id).Delete(&types.User{}).Error
}

// ListUsers 分页查询用户列表，按创建时间倒序。
func (r *userRepository) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, error) {
	var users []*types.User
	query := r.db.WithContext(ctx).Order("create_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// SearchUsers 仅按用户ID匹配活跃用户（添加好友专用）。
func (r *userRepository) SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error) {
	var users []*types.User
	searchPattern := "%" + query + "%"

	dbQuery := r.db.WithContext(ctx).
		Where("user_id LIKE ?", searchPattern).
		Where("is_active = ?", true).
		Order("user_id ASC")

	if limit > 0 {
		dbQuery = dbQuery.Limit(limit)
	} else {
		dbQuery = dbQuery.Limit(20)
	}

	if err := dbQuery.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
