package auth

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ParseAccessToken 校验 access JWT，返回 (userID, platformID)。
// platform_id 缺失时默认 1（Web）。
func ParseAccessToken(tokenStr, secret string) (userID string, platformID int32, err error) {
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return "", 0, fmt.Errorf("empty token")
	}
	if secret == "" {
		return "", 0, fmt.Errorf("jwt secret not configured")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", 0, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", 0, fmt.Errorf("invalid token claims")
	}
	if typ, _ := claims["type"].(string); typ != "" && typ != "access" {
		return "", 0, fmt.Errorf("token type must be access")
	}
	uid, _ := claims["user_id"].(string)
	if uid == "" {
		return "", 0, fmt.Errorf("user_id missing in token")
	}
	platformID = 1
	switch v := claims["platform_id"].(type) {
	case float64:
		if v > 0 {
			platformID = int32(v)
		}
	case int64:
		if v > 0 {
			platformID = int32(v)
		}
	}
	return uid, platformID, nil
}
