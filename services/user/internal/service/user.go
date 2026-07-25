package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"user/internal/types"
	"user/internal/types/interfaces"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecretOnce sync.Once
	jwtSecret     string

	ErrPasswordPolicy = errors.New("password must be 8-32 characters and contain at least one letter and one number")
)

// ValidatePasswordPolicy keeps administrative password resets aligned with
// the registration form's documented policy.
func ValidatePasswordPolicy(password string) error {
	length := utf8.RuneCountInString(password)
	if length < 8 || length > 32 {
		return ErrPasswordPolicy
	}
	hasLetter := false
	hasNumber := false
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return ErrPasswordPolicy
	}
	return nil
}

// getJwtSecret retrieves the JWT secret from the environment, falling back
// to a securely generated random secret.
func getJwtSecret() string {
	jwtSecretOnce.Do(func() {
		if envSecret := strings.TrimSpace(os.Getenv("JWT_SECRET")); envSecret != "" {
			jwtSecret = envSecret
			return
		}
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			panic(fmt.Sprintf("failed to generate JWT secret: %v", err))
		}
		jwtSecret = base64.StdEncoding.EncodeToString(randomBytes)
	})
	return jwtSecret
}

// userService implements the UserService interface.
type userService struct {
	userRepo  interfaces.UserRepository
	tokenRepo interfaces.AuthTokenRepository
}

// NewUserService creates a new user service instance.
func NewUserService(
	userRepo interfaces.UserRepository,
	tokenRepo interfaces.AuthTokenRepository,
) interfaces.UserService {
	return &userService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
	}
}

// Register creates a new user account.
func (s *userService) Register(ctx context.Context, req *types.RegisterRequest) (*types.RegisterResponse, error) {
	slog.InfoContext(ctx, "start user registration")

	if req.Username == "" || req.Email == "" || req.Password == "" {
		return &types.RegisterResponse{
			Success: false,
			Message: "username, email and password are required",
		}, nil
	}

	// Check if user already exists by email.
	existingUser, _ := s.userRepo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		slog.WarnContext(ctx, "email already registered", "email", req.Email)
		return &types.RegisterResponse{
			Success: false,
			Message: "user with this email already exists",
		}, nil
	}

	// Validate password policy.
	if err := ValidatePasswordPolicy(req.Password); err != nil {
		return &types.RegisterResponse{
			Success: false,
			Message: "password must be 8-32 characters with at least one letter and one number",
		}, nil
	}

	// Hash password.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "failed to hash password", "error", err)
		return &types.RegisterResponse{
			Success: false,
			Message: "failed to process password",
		}, nil
	}

	// Create user.
	now := time.Now()
	user := &types.User{
		UserID:       uuid.New().String(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Nickname:     req.Username,
		IsActive:     true,
		CreateTime:   now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		slog.ErrorContext(ctx, "failed to create user", "error", err)
		return &types.RegisterResponse{
			Success: false,
			Message: "failed to create user",
		}, nil
	}

	slog.InfoContext(ctx, "user registered successfully", "user_id", user.UserID)
	return &types.RegisterResponse{
		Success: true,
		Message: "registration successful",
		User:    user,
	}, nil
}

// Login authenticates a user and returns tokens.
func (s *userService) Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	slog.InfoContext(ctx, "start user login")

	// Get user by email.
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil || user == nil {
		return &types.LoginResponse{
			Success: false,
			Message: "invalid email or password",
		}, nil
	}

	// Check if user is active.
	if !user.IsActive {
		return &types.LoginResponse{
			Success: false,
			Message: "account is disabled",
		}, nil
	}

	// Verify password.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return &types.LoginResponse{
			Success: false,
			Message: "invalid email or password",
		}, nil
	}

	// Generate tokens.
	accessToken, refreshToken, err := s.GenerateTokens(ctx, user)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate tokens", "error", err)
		return &types.LoginResponse{
			Success: false,
			Message: "login failed",
		}, nil
	}

	slog.InfoContext(ctx, "user logged in successfully", "user_id", user.UserID)
	return &types.LoginResponse{
		Success:      true,
		Message:      "login successful",
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// GetUserByID gets a user by ID.
func (s *userService) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}

// GetUsersByIDs batch-fetches users.
func (s *userService) GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error) {
	return s.userRepo.GetUsersByIDs(ctx, ids)
}

// GetUserByEmail gets a user by email.
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	return s.userRepo.GetUserByEmail(ctx, email)
}

// UpdateUser updates user information.
func (s *userService) UpdateUser(ctx context.Context, user *types.User) error {
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// DeleteUser deletes a user.
func (s *userService) DeleteUser(ctx context.Context, id string) error {
	return s.userRepo.DeleteUser(ctx, id)
}

// ChangePassword changes user password.
func (s *userService) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify old password.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("invalid old password")
	}

	// Hash new password.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.UpdatedAt = time.Now()

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return err
	}

	// Invalidate every outstanding session so a stolen token cannot
	// survive a password rotation.
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// ValidatePassword validates user password.
func (s *userService) ValidatePassword(ctx context.Context, userID string, password string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// GenerateTokens generates access and refresh tokens for a user.
func (s *userService) GenerateTokens(
	ctx context.Context,
	user *types.User,
) (accessToken, refreshToken string, err error) {
	now := time.Now()

	// Access token: short-lived.
	accessClaims := jwt.MapClaims{
		"user_id":  user.UserID,
		"email":    user.Email,
		"type":     "access",
		"iat":      now.Unix(),
		"exp":      now.Add(24 * time.Hour).Unix(),
	}
	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(getJwtSecret()))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh token: long-lived.
	refreshClaims := jwt.MapClaims{
		"user_id": user.UserID,
		"type":    "refresh",
		"iat":     now.Unix(),
		"exp":     now.Add(30 * 24 * time.Hour).Unix(),
	}
	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(getJwtSecret()))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Persist both tokens.
	for _, t := range []struct {
		token, tokenType string
		expiresAt        time.Time
	}{
		{accessToken, "access_token", now.Add(24 * time.Hour)},
		{refreshToken, "refresh_token", now.Add(30 * 24 * time.Hour)},
	} {
		record := &types.AuthToken{
			ID:        uuid.New().String(),
			UserID:    user.UserID,
			Token:     t.token,
			TokenType: t.tokenType,
			ExpiresAt: t.expiresAt,
			IsRevoked: false,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.tokenRepo.CreateToken(ctx, record); err != nil {
			slog.ErrorContext(ctx, "failed to persist token", "token_type", t.tokenType, "error", err)
		}
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates an access token and returns the associated user.
func (s *userService) ValidateToken(ctx context.Context, tokenString string) (*types.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getJwtSecret()), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return nil, errors.New("invalid user ID in token")
	}

	if isRefreshTokenClaims(claims) {
		return nil, errors.New("refresh token cannot be used as access token")
	}

	// Check if token is revoked.
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return nil, errors.New("token is revoked")
	}
	if tokenRecord.TokenType == "refresh_token" {
		return nil, errors.New("refresh token cannot be used as access token")
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func isRefreshTokenClaims(claims jwt.MapClaims) bool {
	tokenType, ok := claims["type"].(string)
	return ok && tokenType == "refresh"
}

func userIDFromSignedToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getJwtSecret()), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil || token == nil || !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", errors.New("invalid user ID in token")
	}
	return userID, nil
}

// RefreshToken refreshes access token using refresh token.
func (s *userService) RefreshToken(
	ctx context.Context,
	refreshTokenString string,
) (accessToken, newRefreshToken string, err error) {
	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getJwtSecret()), nil
	})
	if err != nil || !token.Valid {
		return "", "", errors.New("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid token claims")
	}

	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return "", "", errors.New("not a refresh token")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", "", errors.New("invalid user ID in token")
	}

	// Check if token is revoked.
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, refreshTokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return "", "", errors.New("refresh token is revoked")
	}
	if tokenRecord.TokenType != "refresh_token" {
		return "", "", errors.New("not a refresh token")
	}

	// Get user.
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}

	// Revoke old refresh token.
	tokenRecord.IsRevoked = true
	_ = s.tokenRepo.UpdateToken(ctx, tokenRecord)

	// Generate new tokens.
	return s.GenerateTokens(ctx, user)
}

// Logout invalidates every outstanding session for the user.
func (s *userService) Logout(ctx context.Context, tokenString string) error {
	userID, err := userIDFromSignedToken(tokenString)
	if err != nil {
		return err
	}
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// RevokeToken revokes a specific token.
func (s *userService) RevokeToken(ctx context.Context, tokenString string) error {
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil {
		return err
	}
	tokenRecord.IsRevoked = true
	tokenRecord.UpdatedAt = time.Now()
	return s.tokenRepo.UpdateToken(ctx, tokenRecord)
}

// SearchUsers searches users by nickname or email.
func (s *userService) SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error) {
	if query == "" {
		return []*types.User{}, nil
	}
	return s.userRepo.SearchUsers(ctx, query, limit)
}
