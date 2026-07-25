package repository

import (
	"context"
	"errors"
	"user/internal/types"
	"user/internal/types/interfaces"

	"gorm.io/gorm"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) interfaces.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) CreateUser(ctx context.Context, user *types.User) error {
	// users.tenant_id is nullable in both PostgreSQL and SQLite. GORM would
	// otherwise serialise the uint64 zero value as 0, which violates the
	// PostgreSQL FK and loses the distinction between "not provisioned yet"
	// and a real tenant. Omitting the column stores SQL NULL; reads hydrate it
	// back as zero, the domain sentinel used by tenantless auth flows.
	if user != nil {
		return r.db.WithContext(ctx).Omit("tenant_id").Create(user).Error
	}
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	var user types.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error) {
	out := make(map[string]*types.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var users []*types.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, u := range users {
		out[u.UserID] = u
	}
	return out, nil
}

// GetUserByEmail gets a user by email
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

// GetUserByUsername gets a user by username
func (r *userRepository) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	var user types.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByTenantID gets the first user (owner) of a tenant
func (r *userRepository) GetUserByTenantID(ctx context.Context, tenantID uint64) (*types.User, error) {
	var user types.User
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates a user
func (r *userRepository) UpdateUser(ctx context.Context, user *types.User) error {
	if user != nil {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Preserve Save's all-fields behaviour while keeping the nullable
			// tenant column out of the struct write, then explicitly store NULL.
			// Writing uint64(0) would violate the PostgreSQL tenant FK.
			if err := tx.Omit("tenant_id").Save(user).Error; err != nil {
				return err
			}
			return tx.Model(&types.User{}).
				Where("id = ?", user.UserID).
				UpdateColumn("tenant_id", nil).Error
		})
	}
	return r.db.WithContext(ctx).Save(user).Error
}

// DeleteUser deletes a user
func (r *userRepository) DeleteUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&types.User{}).Error
}

// ListUsers lists users with pagination
func (r *userRepository) ListUsers(ctx context.Context, offset, limit int) ([]*types.User, error) {
	var users []*types.User
	query := r.db.WithContext(ctx).Order("created_at DESC")

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

// SearchUsers searches users by username or email
func (r *userRepository) SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error) {
	var users []*types.User
	searchPattern := "%" + query + "%"

	dbQuery := r.db.WithContext(ctx).
		Where("username ILIKE ? OR email ILIKE ?", searchPattern, searchPattern).
		Where("is_active = ?", true).
		Order("username ASC")

	if limit > 0 {
		dbQuery = dbQuery.Limit(limit)
	} else {
		dbQuery = dbQuery.Limit(20) // default limit
	}

	if err := dbQuery.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
