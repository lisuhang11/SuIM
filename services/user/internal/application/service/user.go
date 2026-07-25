package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm/logger"

	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"user/internal/types"
	"user/internal/types/interfaces"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecretOnce sync.Once
	jwtSecret     string

	// ErrPasswordPolicy is returned when a newly chosen password does not
	// meet the product's public 8-32 character, letter-and-number contract.
	// It is exported so HTTP handlers can translate the failure to a 400
	// without exposing bcrypt or persistence errors.
	ErrPasswordPolicy = errors.New("password must be 8-32 characters and contain at least one letter and one number")
)

// ValidatePasswordPolicy keeps administrative password resets aligned with
// the registration form's documented policy. Password bytes are never logged
// or included in the returned error.
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

// getJwtSecret retrieves the JWT secret from the environment, falling back to a securely generated random secret.
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

// userService implements the UserService interface
type userService struct {
	userRepo  interfaces.UserRepository
	tokenRepo interfaces.AuthTokenRepository

	config *config.Config
}

// NewUserService creates a new user service instance
func NewUserService(
	configInfo *config.Config,
	userRepo interfaces.UserRepository,
	tokenRepo interfaces.AuthTokenRepository,
) interfaces.UserService {
	return &userService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		config:    configInfo,
	}
}

// Register creates a new user account
func (s *userService) Register(ctx context.Context, req *types.RegisterRequest) (*types.User, error) {
	logger.Info(ctx, "Start user registration")

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("username, email and password are required")
	}

	// Check if user already exists
	existingUser, _ := s.userRepo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, errors.New("user with this email already exists")
	}

	existingUser, _ = s.userRepo.GetUserByUsername(ctx, req.Username)
	if existingUser != nil {
		return nil, errors.New("user with this username already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Errorf(ctx, "Failed to hash password: %v", err)
		return nil, errors.New("failed to process password")
	}

	// Create user
	user := &types.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.userRepo.CreateUser(ctx, user)
	if err != nil {
		logger.Errorf(ctx, "Failed to create user: %v", err)
		if createdTenant != nil {
			if rollbackErr := s.tenantService.DeleteTenant(ctx, createdTenant.ID); rollbackErr != nil {
				logger.Errorf(ctx, "Failed to roll back tenant %d after user creation failure: %v", createdTenant.ID, rollbackErr)
			}
		}
		return nil, errors.New("failed to create user")
	}

	logger.Info(ctx, "User registered successfully")
	return user, nil
}

// Login authenticates a user and returns tokens
func (s *userService) Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	logger.Info(ctx, "Start user login")
	// Get user by email
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		logger.Errorf(ctx, "Failed to get user by email: %v", err)
		return &types.LoginResponse{
			Success: false,
			Message: "Invalid email or password",
		}, nil
	}
	if user == nil {
		logger.Warn(ctx, "User not found for email")
		return &types.LoginResponse{
			Success: false,
			Message: "Invalid email or password",
		}, nil
	}

	// Check if user is active
	if !user.IsActive {
		logger.Warn(ctx, "User account is disabled")
		return &types.LoginResponse{
			Success: false,
			Message: "Account is disabled",
		}, nil
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		logger.Warn(ctx, "Password verification failed")
		return &types.LoginResponse{
			Success: false,
			Message: "Invalid email or password",
		}, nil
	}
	logger.Info(ctx, "Password verification successful")

	// Generate tokens. Resolve the target tenant once so the JWT claim
	// and the tenant we return below agree — otherwise an honoured
	// "last active tenant" preference would mint a token for tenant N
	// but tell the client they're in their home tenant.
	logger.Info(ctx, "Generating tokens")
	resolvedTenantID := s.resolveLoginTenantID(ctx, user)
	accessToken, refreshToken, err := s.generateTokensForTenant(ctx, user, resolvedTenantID)
	if err != nil {
		logger.Errorf(ctx, "Failed to generate tokens: %v", err)
		return &types.LoginResponse{
			Success: false,
			Message: "Login failed",
		}, nil
	}
	logger.Info(ctx, "Tokens generated successfully")

	memberships := s.buildMembershipsForUser(ctx, user, tenant)

	logger.Info(ctx, "User logged in successfully")
	return &types.LoginResponse{
		Success:      true,
		Message:      "Login successful",
		User:         user,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// GetUserByID gets a user by ID
func (s *userService) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}

// GetUsersByIDs proxies to the repository batch fetch. Returns an empty
// map for an empty input; missing ids are absent from the result.
func (s *userService) GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error) {
	return s.userRepo.GetUsersByIDs(ctx, ids)
}

// GetUserByEmail gets a user by email
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	return s.userRepo.GetUserByEmail(ctx, email)
}

// UpdateUser updates user information
func (s *userService) UpdateUser(ctx context.Context, user *types.User) error {
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// DeleteUser deletes a user
func (s *userService) DeleteUser(ctx context.Context, id string) error {
	return s.userRepo.DeleteUser(ctx, id)
}

// ChangePassword changes user password
func (s *userService) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify old password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword))
	if err != nil {
		return errors.New("invalid old password")
	}

	// Hash new password
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

// ValidatePassword validates user password
func (s *userService) ValidatePassword(ctx context.Context, userID string, password string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// GenerateTokens generates access and refresh tokens for user. The
// access token's tenant_id claim defaults to user.TenantID (home), but
// if the user has persisted a still-valid "last active tenant"
// preference we honour it instead — so login (and the refresh-token
// rotation path that also calls into here) lands the user back where
// they left off across devices. SwitchTenant remains the explicit tool
// for switching to an arbitrary membership.
func (s *userService) GenerateTokens(
	ctx context.Context,
	user *types.User,
) (accessToken, refreshToken string, err error) {
	return s.generateTokensForTenant(ctx, user, s.resolveLoginTenantID(ctx, user))
}

// ValidateToken validates an access token. The second return value is
// the JWT's `tenant_id` claim — i.e. the tenant the token was minted
// for, which may differ from user.TenantID after a /auth/switch-tenant
// call. Tokens minted before tenant-level RBAC don't carry the claim;
// in that case we fall back to user.TenantID for backward compatibility.
func (s *userService) ValidateToken(ctx context.Context, tokenString string) (*types.User, uint64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getJwtSecret()), nil
	})

	if err != nil || !token.Valid {
		return nil, 0, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, 0, errors.New("invalid token claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return nil, 0, errors.New("invalid user ID in token")
	}

	if isRefreshTokenClaims(claims) {
		return nil, 0, errors.New("refresh token cannot be used as access token")
	}

	// Check if token is revoked
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return nil, 0, errors.New("token is revoked")
	}
	if tokenRecord.TokenType == "refresh_token" {
		return nil, 0, errors.New("refresh token cannot be used as access token")
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	// Extract active tenant from the JWT. Anything missing or unparseable
	// falls back to the user's home tenant so old tokens (and tokens issued
	// by code paths that don't yet set the claim) keep working.
	activeTenantID := tenantIDFromClaims(claims, user.TenantID)

	return user, activeTenantID, nil
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

// RefreshToken refreshes access token using refresh token
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

	// Check if token is revoked
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, refreshTokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return "", "", errors.New("refresh token is revoked")
	}
	if tokenRecord.TokenType != "refresh_token" {
		return "", "", errors.New("not a refresh token")
	}

	// Get user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}

	// Revoke old refresh token
	tokenRecord.IsRevoked = true
	_ = s.tokenRepo.UpdateToken(ctx, tokenRecord)

	// Generate new tokens
	return s.GenerateTokens(ctx, user)
}

// Logout invalidates every outstanding session for the user identified by
// the presented JWT. Access and refresh tokens are both accepted so clients
// can end the session without refreshing first; expired tokens are allowed
// so logout still works after the access token TTL.
func (s *userService) Logout(ctx context.Context, tokenString string) error {
	userID, err := userIDFromSignedToken(tokenString)
	if err != nil {
		return err
	}
	return s.tokenRepo.RevokeTokensByUserID(ctx, userID)
}

// RevokeToken revokes a token
func (s *userService) RevokeToken(ctx context.Context, tokenString string) error {
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil {
		return err
	}

	tokenRecord.IsRevoked = true
	tokenRecord.UpdatedAt = time.Now()

	return s.tokenRepo.UpdateToken(ctx, tokenRecord)
}

// SearchUsers searches users by username or email
func (s *userService) SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error) {
	if query == "" {
		return []*types.User{}, nil
	}
	return s.userRepo.SearchUsers(ctx, query, limit)
}

func (s *userService) generateOIDCUsername(ctx context.Context, info *types.OIDCUserInfo) string {
	base := sanitizeUsernameCandidate(info.Username)
	if base == "" {
		base = sanitizeUsernameCandidate(strings.Split(info.Email, "@")[0])
	}
	if base == "" {
		base = "oidc-user"
	}

	candidate := base
	for i := 0; i < 20; i++ {
		existing, err := s.userRepo.GetUserByUsername(ctx, candidate)
		if isUserLookupNotFound(err) || (err == nil && existing == nil) {
			return candidate
		}
		if err != nil && !isUserLookupNotFound(err) {
			logger.Warnf(ctx, "Failed to check existing OIDC username %q: %v", candidate, err)
		}
		candidate = fmt.Sprintf("%s-%d", base, i+1)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func generateRandomString(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func decodeJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func extractClaimAsString(claims map[string]interface{}, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func sanitizeUsernameCandidate(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-._")
	if len(result) > 50 {
		result = strings.Trim(result[:50], "-._")
	}
	return result
}

func isUserLookupNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, apprepo.ErrUserNotFound) || strings.Contains(strings.ToLower(err.Error()), "user not found")
}
