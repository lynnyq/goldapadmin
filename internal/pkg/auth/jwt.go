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

// TokenIssuer JWT 签发者标识
const TokenIssuer = "goldapadmin"

var (
	jwtSecret   []byte
	jwtSecretMu sync.RWMutex
)

// SetSecret 设置 JWT 密钥（必须由配置校验后的强随机值）
func SetSecret(secret string) {
	if secret == "" {
		return
	}
	jwtSecretMu.Lock()
	jwtSecret = []byte(secret)
	jwtSecretMu.Unlock()
}

func getSecret() []byte {
	jwtSecretMu.RLock()
	defer jwtSecretMu.RUnlock()
	return jwtSecret
}

// GenerateToken 生成 JWT Token
func GenerateToken(serverID int, userDN string, ttl time.Duration) (string, error) {
	secret := getSecret()
	if len(secret) == 0 {
		return "", errors.New("jwt secret not configured")
	}
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	claims := Claims{
		ServerID: serverID,
		UserDN:   userDN,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    TokenIssuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ParseToken 解析 JWT Token，返回 Claims。不检查黑名单（由调用方额外校验）
//
// 校验项：签名算法必须为 HS256、issuer 必须为 TokenIssuer、exp 必须存在且未过期。
func ParseToken(tokenString string) (*Claims, error) {
	secret := getSecret()
	if len(secret) == 0 {
		return nil, errors.New("jwt secret not configured")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(TokenIssuer),
		jwt.WithExpirationRequired(),
	)
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

// BlacklistStore 黑名单持久化存储接口（由 dao 实现，避免 auth 包依赖数据库）
type BlacklistStore interface {
	Add(token string, expiresAt time.Time) error
	Exists(token string) (bool, error)
	CleanExpired(now time.Time) error
}

var blacklistStore BlacklistStore

// SetBlacklistStore 注入黑名单持久化存储；未注入时仅使用内存黑名单
func SetBlacklistStore(s BlacklistStore) {
	blacklistStore = s
}

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
	if blacklistStore != nil {
		_ = blacklistStore.Add(tokenString, expiresAt)
	}
}

// IsBlacklisted 检查 token 是否在黑名单中（含惰性清理已过期条目）
func IsBlacklisted(tokenString string) bool {
	if tokenString == "" {
		return false
	}
	v, ok := blacklist.Load(tokenString)
	if ok {
		e := v.(blacklistEntry)
		if time.Now().After(e.expiresAt) {
			blacklist.Delete(tokenString)
		} else {
			return true
		}
	}
	if blacklistStore != nil {
		if exists, err := blacklistStore.Exists(tokenString); err == nil && exists {
			return true
		}
	}
	return false
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
	if blacklistStore != nil {
		_ = blacklistStore.CleanExpired(now)
	}
}
