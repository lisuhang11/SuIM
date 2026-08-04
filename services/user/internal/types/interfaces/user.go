// Package interfaces 定义用户服务的接口契约，解耦业务逻辑与具体实现。
package interfaces

import (
	"context"
	"user/internal/types"
)

// UserService 定义用户业务逻辑的接口契约。
type UserService interface {
	// Register 注册新用户，邮箱不能重复，密码需满足策略。成功返回创建的 User。
	Register(ctx context.Context, email, username, password string) (*types.User, error)
	// Login 验证邮箱和密码，成功返回用户、访问令牌和刷新令牌。
	Login(ctx context.Context, email, password string) (*types.User, string, string, error)
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// UpdateUser 全量保存用户（如改密）；资料局部更新请用 UpdateUserProfile。
	UpdateUser(ctx context.Context, user *types.User) error
	// UpdateUserProfile 按字段 patch 更新资料（对齐 OpenIM UpdateByMap）。
	UpdateUserProfile(ctx context.Context, userID string, patch *types.UserProfilePatch) error
	DeleteUser(ctx context.Context, id string) error
	ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error
	ValidatePassword(ctx context.Context, userID string, password string) error
	GenerateTokens(ctx context.Context, user *types.User) (accessToken, refreshToken string, err error)
	ValidateToken(ctx context.Context, token string) (user *types.User, err error)
	RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)
	RevokeToken(ctx context.Context, token string) (err error)
	Logout(ctx context.Context, token string) (err error)
	SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error)
	// SetGlobalRecvMessageOpt 设置用户全局消息接收选项（0/1/2）。
	SetGlobalRecvMessageOpt(ctx context.Context, userID string, opt int) error
	// GetGlobalRecvMessageOpt 获取用户全局消息接收选项。
	GetGlobalRecvMessageOpt(ctx context.Context, userID string) (int, error)
}

// UserRepository 定义用户持久化操作的接口契约。
type UserRepository interface {
	// CreateUser 创建用户。
	CreateUser(ctx context.Context, user *types.User) error
	// GetUserByID 根据 ID 获取用户。
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	// GetUsersByIDs 批量获取用户，返回 ID 到 User 的映射，不存在的 ID 不出现在结果中。
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	// GetUserByEmail 根据邮箱获取用户。
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// UpdateUser 全量保存。
	UpdateUser(ctx context.Context, user *types.User) error
	// UpdateUserByMap 仅更新 map 中出现的列（OpenIM UpdateByMap 语义）。
	UpdateUserByMap(ctx context.Context, userID string, fields map[string]any) error
	// UpdateGlobalRecvMsgOpt 仅更新全局消息接收选项。
	UpdateGlobalRecvMsgOpt(ctx context.Context, userID string, opt int) error
	// DeleteUser 删除用户。
	DeleteUser(ctx context.Context, id string) error
	// ListUsers 分页查询用户列表。
	ListUsers(ctx context.Context, offset, limit int) ([]*types.User, error)
	// SearchUsers 按用户 ID 精确查找。
	SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error)
}

// AuthTokenRepository 定义认证令牌持久化操作的接口契约。
type AuthTokenRepository interface {
	// CreateToken 创建认证令牌。
	CreateToken(ctx context.Context, token *types.AuthToken) error
	// GetTokenByValue 根据令牌值查询。
	GetTokenByValue(ctx context.Context, tokenValue string) (*types.AuthToken, error)
	// GetTokensByUserID 查询用户所有令牌。
	GetTokensByUserID(ctx context.Context, userID string) ([]*types.AuthToken, error)
	// UpdateToken 更新令牌记录。
	UpdateToken(ctx context.Context, token *types.AuthToken) error
	// DeleteToken 删除令牌。
	DeleteToken(ctx context.Context, id string) error
	// DeleteExpiredTokens 删除所有过期令牌。
	DeleteExpiredTokens(ctx context.Context) error
	// RevokeTokensByUserID 吊销用户所有令牌。
	RevokeTokensByUserID(ctx context.Context, userID string) error
}
