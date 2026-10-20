package token

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

type Claims struct {
	UserID     string `json:"uid"`
	PlatformID int32  `json:"pid"`
	Type       string `json:"typ"`
	jwt.RegisteredClaims
}

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(cfg config.JWT) *Issuer {
	return &Issuer{
		secret: []byte(cfg.Secret),
		ttl:    time.Duration(cfg.ExpireSeconds) * time.Second,
	}
}

func (i *Issuer) ExpireSeconds() int64 { return int64(i.ttl / time.Second) }

func (i *Issuer) Sign(id, userID string, platformID int32, now time.Time) (string, time.Time, error) {
	if platformID <= 0 {
		return "", time.Time{}, fmt.Errorf("platform id is required")
	}
	exp := now.Add(i.ttl)
	claims := Claims{
		UserID:     userID,
		PlatformID: platformID,
		Type:       model.TokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        id,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (i *Issuer) Parse(raw string) (*Claims, error) {
	return i.parse(raw, false)
}

// ParseIgnoreExpiry verifies the signature and accepts an expired token.
// Logout uses it so an expired token can still end the session.
func (i *Issuer) ParseIgnoreExpiry(raw string) (*Claims, error) {
	return i.parse(raw, true)
}

func (i *Issuer) parse(raw string, ignoreExpiry bool) (*Claims, error) {
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()})}
	if ignoreExpiry {
		opts = append(opts, jwt.WithoutClaimsValidation())
	}
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return i.secret, nil
	}, opts...)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || claims.UserID == "" || claims.ID == "" || claims.Type == "" || claims.PlatformID <= 0 {
		return nil, fmt.Errorf("incomplete token claims")
	}
	if ignoreExpiry && claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return claims, nil
	}
	return claims, nil
}

func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
