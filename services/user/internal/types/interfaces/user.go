package interfaces

import (
	"context"
	"user/internal/types"
)

type UserService interface {
	Register(ctx context.Context, req *types.RegisterRequest) (*types.RegisterResponse, error)
	Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error)
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	UpdateUser(ctx context.Context, user *types.User) error
	DeleteUser(ctx context.Context, id string) error
	ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error
	ValidatePassword(ctx context.Context, userID string, password string) error
	GenerateTokens(ctx context.Context, user *types.User) (accessToken, refreshToken string, err error)
	ValidateToken(ctx context.Context, token string) (user *types.User, err error)
	RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)
	RevokeToken(ctx context.Context, token string) (err error)
	Logout(ctx context.Context, token string) (err error)
	SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error)
}

type UserRepository interface {
	// CreateUser creates a user
	CreateUser(ctx context.Context, user *types.User) error
	// GetUserByID gets a user by ID
	GetUserByID(ctx context.Context, id string) (*types.User, error)
	// GetUsersByIDs batch-fetches users by id, returning a map keyed by
	// user id. Missing ids are simply absent from the result.
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
	// GetUserByEmail gets a user by email
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user *types.User) error
	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, id string) error
	// ListUsers lists users with pagination
	ListUsers(ctx context.Context, offset, limit int) ([]*types.User, error)
	// SearchUsers searches users by username or email
	SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error)
}

type AuthTokenRepository interface {
	// CreateToken creates an auth token
	CreateToken(ctx context.Context, token *types.AuthToken) error
	// GetTokenByValue gets a token by its value
	GetTokenByValue(ctx context.Context, tokenValue string) (*types.AuthToken, error)
	// GetTokensByUserID gets all tokens for a user
	GetTokensByUserID(ctx context.Context, userID string) ([]*types.AuthToken, error)
	// UpdateToken updates a token
	UpdateToken(ctx context.Context, token *types.AuthToken) error
	// DeleteToken deletes a token
	DeleteToken(ctx context.Context, id string) error
	// DeleteExpiredTokens deletes all expired tokens
	DeleteExpiredTokens(ctx context.Context) error
	// RevokeTokensByUserID revokes all tokens for a user
	RevokeTokensByUserID(ctx context.Context, userID string) error
}
