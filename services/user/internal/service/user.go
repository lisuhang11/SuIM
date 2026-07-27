// Package service 实现用户业务逻辑，包括注册、登录、令牌管理等核心流程。
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	apperrors "user/internal/errors"

	"user/internal/config"
	"user/internal/types"
	"user/internal/types/interfaces"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecretOnce sync.Once
	jwtSecret     string

	// ErrPasswordPolicy 密码策略不满足的错误。
	ErrPasswordPolicy = errors.New("password must be 8-32 characters and contain at least one letter and one number")

	// emailRegex 校验邮箱格式的正则表达式。
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// ValidatePasswordPolicy 验证密码符合长度（8-32）且包含至少一个字母和一个数字的策略。
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

// getJwtSecret 获取 JWT 密钥，优先使用环境变量 JWT_SECRET，否则生成随机密钥。
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

// userService 实现 UserService 接口，封装用户业务逻辑。
type userService struct {
	userRepo  interfaces.UserRepository
	tokenRepo interfaces.AuthTokenRepository
	config    *config.Config
}

// NewUserService 创建用户服务实例。
func NewUserService(
	userRepo interfaces.UserRepository,
	tokenRepo interfaces.AuthTokenRepository,
	cfg *config.Config,
) interfaces.UserService {
	return &userService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		config:    cfg,
	}
}

// Register 注册新用户，校验邮箱唯一性和密码策略，创建用户记录。
// 注：username 参数最终存储到领域模型 User.Nickname 字段，作为用户的显示昵称。
func (s *userService) Register(ctx context.Context, email, username, password string) (*types.User, error) {
	slog.InfoContext(ctx, "start user registration")

	// 校验必填字段。
	if username == "" || email == "" || password == "" {
		return nil, apperrors.NewValidationError("username, email and password are required")
	}

	// 校验邮箱格式。
	if !emailRegex.MatchString(email) {
		return nil, apperrors.NewValidationError("invalid email format")
	}

	// 检查邮箱是否已被注册。
	existingUser, _ := s.userRepo.GetUserByEmail(ctx, email)
	if existingUser != nil {
		slog.WarnContext(ctx, "email already registered", "email", email)
		return nil, apperrors.NewUserExistsError()
	}

	// 验证密码策略。
	if err := ValidatePasswordPolicy(password); err != nil {
		return nil, apperrors.NewPasswordPolicyError()
	}

	// 使用 bcrypt 哈希密码。
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "failed to hash password", "error", err)
		return nil, apperrors.NewInternalError("failed to process password").WithDetails(err)
	}

	// 创建用户记录。
	now := time.Now()
	user := &types.User{
		UserID:       uuid.New().String(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		Nickname:     username,
		IsActive:     true,
		CreateTime:   now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		slog.ErrorContext(ctx, "failed to create user", "error", err)
		return nil, apperrors.NewInternalError("failed to create user").WithDetails(err)
	}

	slog.InfoContext(ctx, "user registered successfully", "user_id", user.UserID)
	return user, nil
}

// Login 验证用户邮箱和密码，生成并返回用户、访问令牌和刷新令牌。
func (s *userService) Login(ctx context.Context, email, password string) (*types.User, string, string, error) {
	slog.InfoContext(ctx, "start user login")

	// 校验必填字段。
	if email == "" || password == "" {
		return nil, "", "", apperrors.NewValidationError("email and password are required")
	}

	// 校验邮箱格式。
	if !emailRegex.MatchString(email) {
		return nil, "", "", apperrors.NewValidationError("invalid email format")
	}

	// 根据邮箱查询用户。
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, "", "", apperrors.NewPasswordInvalidError()
	}

	// 检查账户是否激活。
	if !user.IsActive {
		return nil, "", "", apperrors.NewUserInactiveError()
	}

	// 验证密码。
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", "", apperrors.NewPasswordInvalidError()
	}

	// 生成令牌对。
	accessToken, refreshToken, err := s.GenerateTokens(ctx, user)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate tokens", "error", err)
		return nil, "", "", apperrors.NewInternalError("login failed").WithDetails(err)
	}

	slog.InfoContext(ctx, "user logged in successfully", "user_id", user.UserID)
	return user, accessToken, refreshToken, nil
}

// GetUserByID 根据 ID 获取用户信息。
func (s *userService) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}

// GetUsersByIDs 批量获取用户信息。
func (s *userService) GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error) {
	return s.userRepo.GetUsersByIDs(ctx, ids)
}

// GetUserByEmail 根据邮箱获取用户信息。
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	return s.userRepo.GetUserByEmail(ctx, email)
}

// UpdateUser 更新用户信息，自动设置更新时间。
func (s *userService) UpdateUser(ctx context.Context, user *types.User) error {
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// DeleteUser 删除指定用户。
func (s *userService) DeleteUser(ctx context.Context, id string) error {
	return s.userRepo.DeleteUser(ctx, id)
}

// ChangePassword 修改用户密码：验证旧密码 → 哈希新密码 → 保存 → 吊销所有旧令牌。
func (s *userService) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// 验证旧密码。
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("invalid old password")
	}

	// 哈希新密码。
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.UpdatedAt = time.Now()

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return err
	}

	// 吊销所有旧令牌，防止密码变更后被窃取的令牌继续有效。
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// ValidatePassword 验证用户密码是否正确。
func (s *userService) ValidatePassword(ctx context.Context, userID string, password string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// GenerateTokens 为用户生成一对 JWT 令牌（访问令牌 + 刷新令牌），有效期从配置读取，并持久化。
func (s *userService) GenerateTokens(
	ctx context.Context,
	user *types.User,
) (accessToken, refreshToken string, err error) {
	now := time.Now()
	accessExpireAt := now.Add(s.config.AccessTokenTTL)
	refreshExpireAt := now.Add(s.config.RefreshTokenTTL)

	// 访问令牌：短时效，用于 API 鉴权。
	accessClaims := jwt.MapClaims{
		"user_id": user.UserID,
		"email":   user.Email,
		"type":    "access",
		"iat":     now.Unix(),
		"exp":     accessExpireAt.Unix(),
	}
	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(getJwtSecret()))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	// 刷新令牌：长时效，用于换取新的访问令牌。
	refreshClaims := jwt.MapClaims{
		"user_id": user.UserID,
		"type":    "refresh",
		"iat":     now.Unix(),
		"exp":     refreshExpireAt.Unix(),
	}
	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(getJwtSecret()))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// 将两类令牌持久化到数据库。
	for _, t := range []struct {
		token, tokenType string
		expiresAt        time.Time
	}{
		{accessToken, "access_token", accessExpireAt},
		{refreshToken, "refresh_token", refreshExpireAt},
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

// ValidateToken 验证访问令牌，检查签名、类型、吊销状态，返回关联的用户。
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

	// 不支持用刷新令牌替代访问令牌。
	if isRefreshTokenClaims(claims) {
		return nil, errors.New("refresh token cannot be used as access token")
	}

	// 检查令牌是否已吊销。
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

// isRefreshTokenClaims 判断 JWT claims 中的 type 是否为 refresh。
func isRefreshTokenClaims(claims jwt.MapClaims) bool {
	tokenType, ok := claims["type"].(string)
	return ok && tokenType == "refresh"
}

// userIDFromSignedToken 从已签名的 JWT 中提取 user_id（不做完整校验）。
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

// RefreshToken 使用刷新令牌换取新的令牌对，同时吊销旧的刷新令牌。
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

	// 检查刷新令牌是否已吊销。
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, refreshTokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return "", "", errors.New("refresh token is revoked")
	}
	if tokenRecord.TokenType != "refresh_token" {
		return "", "", errors.New("not a refresh token")
	}

	// 获取用户信息。
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}

	// 吊销旧的刷新令牌并生成新令牌对（令牌轮换）。
	tokenRecord.IsRevoked = true
	_ = s.tokenRepo.UpdateToken(ctx, tokenRecord)

	return s.GenerateTokens(ctx, user)
}

// Logout 吊销指定用户的所有令牌以实现登出。
func (s *userService) Logout(ctx context.Context, tokenString string) error {
	userID, err := userIDFromSignedToken(tokenString)
	if err != nil {
		return err
	}
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// RevokeToken 吊销特定的令牌。
func (s *userService) RevokeToken(ctx context.Context, tokenString string) error {
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil {
		return err
	}
	tokenRecord.IsRevoked = true
	tokenRecord.UpdatedAt = time.Now()
	return s.tokenRepo.UpdateToken(ctx, tokenRecord)
}

// SearchUsers 按昵称或邮箱搜索用户。
func (s *userService) SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error) {
	if query == "" {
		return []*types.User{}, nil
	}
	return s.userRepo.SearchUsers(ctx, query, limit)
}
