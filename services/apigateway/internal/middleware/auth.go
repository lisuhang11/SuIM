// Package middleware 提供 JWT 鉴权中间件。
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	headerAuthorization = "Authorization"
	bearerPrefix        = "Bearer "
	ctxKeyUserID        = "user_id"
	ctxKeyToken         = "token"
)

// AuthConfig 鉴权配置。
type AuthConfig struct {
	JWTSecret   string        // JWT HMAC 签名密钥
	CacheTTL    time.Duration // 令牌验证结果本地缓存时间
	PublicPaths []string      // 无需鉴权的路径前缀，如 ["/api/v1/users/login", "/api/v1/users/register"]
}

// AuthMiddleware 管理 JWT 鉴权与本地令牌缓存。
type AuthMiddleware struct {
	authCfg  AuthConfig
	cache    map[string]*cacheEntry
	cacheMu  sync.RWMutex
}

type cacheEntry struct {
	userID    string
	expiresAt time.Time
}

// NewAuthMiddleware 创建鉴权中间件实例。
func NewAuthMiddleware(cfg AuthConfig) *AuthMiddleware {
	m := &AuthMiddleware{
		authCfg: cfg,
		cache:   make(map[string]*cacheEntry),
	}
	// 后台定期清理过期缓存。
	go m.reapCacheLoop()
	return m
}

// UpdateSecret 热更新 JWT 密钥。
func (m *AuthMiddleware) UpdateSecret(secret string) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.authCfg.JWTSecret = secret
	m.cache = make(map[string]*cacheEntry) // 清空旧缓存
	slog.Info("[gateway] JWT secret updated, token cache cleared")
}

// Handler 返回 gin 鉴权中间件。
func (m *AuthMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 公开路径跳过鉴权。
		for _, pp := range m.authCfg.PublicPaths {
			if strings.HasPrefix(path, pp) {
				c.Next()
				return
			}
		}

		// 提取 Bearer token。
		authHeader := c.GetHeader(headerAuthorization)
		if authHeader == "" || !strings.HasPrefix(authHeader, bearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "missing or invalid authorization header",
			})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, bearerPrefix)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "empty token",
			})
			return
		}

		// 检查本地缓存。
		userID, ok := m.lookupCache(tokenStr)
		if ok {
			c.Set(ctxKeyUserID, userID)
			c.Set(ctxKeyToken, tokenStr)
			c.Next()
			return
		}

		// 解析并校验 JWT。
		claims, err := m.parseToken(tokenStr)
		if err != nil {
			slog.Warn("[gateway] invalid token", "request_id", GetRequestID(c), "error", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid or expired token",
			})
			return
		}

		// 提取 user_id（兼容两种常用字段）。
		if sub, err := claims.GetSubject(); err == nil && sub != "" {
			userID = sub
		}
		if uid, ok := claims["user_id"].(string); ok && uid != "" {
			userID = uid
		}
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "token missing user identity",
			})
			return
		}

		// 写入缓存。
		m.storeCache(tokenStr, userID, m.getExpiry(claims))

		c.Set(ctxKeyUserID, userID)
		c.Set(ctxKeyToken, tokenStr)
		c.Next()
	}
}

// GetUserID 从 gin.Context 提取已认证的 user_id。
func GetUserID(c *gin.Context) string {
	if uid, ok := c.Get(ctxKeyUserID); ok {
		return uid.(string)
	}
	return ""
}

// GetToken 从 gin.Context 提取原始 JWT 字符串。
func GetToken(c *gin.Context) string {
	if t, ok := c.Get(ctxKeyToken); ok {
		return t.(string)
	}
	return ""
}

func (m *AuthMiddleware) parseToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(m.authCfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

func (m *AuthMiddleware) getExpiry(claims jwt.MapClaims) time.Time {
	if exp, ok := claims["exp"].(float64); ok {
		return time.Unix(int64(exp), 0)
	}
	// 无 exp 字段时默认缓存 5 分钟。
	return time.Now().Add(5 * time.Minute)
}

// ---------- 本地缓存（避免每次请求都解析 JWT）----------

func (m *AuthMiddleware) lookupCache(token string) (string, bool) {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	entry, ok := m.cache[token]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.userID, true
}

func (m *AuthMiddleware) storeCache(token, userID string, expiresAt time.Time) {
	ttl := m.authCfg.CacheTTL
	if ttl <= 0 {
		ttl = 1 * time.Minute
	}
	maxExp := time.Now().Add(ttl)
	if expiresAt.After(maxExp) {
		expiresAt = maxExp
	}
	m.cacheMu.Lock()
	m.cache[token] = &cacheEntry{userID: userID, expiresAt: expiresAt}
	m.cacheMu.Unlock()
}

func (m *AuthMiddleware) reapCacheLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.reapCache()
	}
}

func (m *AuthMiddleware) reapCache() {
	now := time.Now()
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	for token, entry := range m.cache {
		if now.After(entry.expiresAt) {
			delete(m.cache, token)
		}
	}
}
