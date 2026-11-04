package auth

import (
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷
type Claims struct {
	ServerID int    `json:"server_id"`
	UserDN   string `json:"user_dn"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("goldapadmin-secret-key-change-in-production")

// SetSecret 设置 JWT 密钥
func SetSecret(secret string) {
	if secret != "" {
		jwtSecret = []byte(secret)
	}
}

// GenerateToken 生成 JWT Token
func GenerateToken(serverID int, userDN string, ttl time.Duration) (string, error) {
	claims := Claims{
		ServerID: serverID,
		UserDN:   userDN,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "goldapadmin",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken 解析 JWT Token，返回 Claims。不检查黑名单（由调用方额外校验）
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// ---------------------------------------------------------------------------
// JWT 黑名单（Logout 时使用）
// ---------------------------------------------------------------------------

// blacklistEntry 黑名单条目
type blacklistEntry struct {
	expiresAt time.Time
}

var (
	blacklist   sync.Map // key: JWT string, value: blacklistEntry
	blacklistMu sync.Mutex
)

// AddToBlacklist 将指定 token 加入黑名单，直到其自身过期。
// 若 token 无过期声明则默认 24h 后清除。
func AddToBlacklist(tokenString string, claims *Claims) {
	if tokenString == "" {
		return
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	if claims != nil && claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	blacklist.Store(tokenString, blacklistEntry{expiresAt: expiresAt})
}

// IsBlacklisted 检查 token 是否在黑名单中（含惰性清理已过期条目）
func IsBlacklisted(tokenString string) bool {
	if tokenString == "" {
		return false
	}
	v, ok := blacklist.Load(tokenString)
	if !ok {
		return false
	}
	e := v.(blacklistEntry)
	if time.Now().After(e.expiresAt) {
		blacklist.Delete(tokenString)
		return false
	}
	return true
}

// CleanExpiredBlacklist 清除黑名单中已过期的条目（可定时调用）
func CleanExpiredBlacklist() {
	now := time.Now()
	blacklist.Range(func(key, value any) bool {
		e := value.(blacklistEntry)
		if now.After(e.expiresAt) {
			blacklist.Delete(key)
		}
		return true
	})
}
