package dao

import (
	"time"
)

// JWTBlacklistDAO JWT 黑名单持久化
//
// 落库后进程重启不会让已登出的 token 重新生效。
type JWTBlacklistDAO struct{}

// NewJWTBlacklistDAO 创建 JWT 黑名单 DAO
func NewJWTBlacklistDAO() *JWTBlacklistDAO {
	return &JWTBlacklistDAO{}
}

// Add 写入黑名单
func (d *JWTBlacklistDAO) Add(token string, expiresAt time.Time) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`INSERT OR REPLACE INTO jwt_blacklist (token, expires_at) VALUES (?, ?)`, token, expiresAt.UTC().Format("2006-01-02 15:04:05"))
	return err
}

// Exists 判断 token 是否在黑名单中
func (d *JWTBlacklistDAO) Exists(token string) (bool, error) {
	if db == nil {
		return false, nil
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM jwt_blacklist WHERE token = ? AND expires_at > ?`,
		token, time.Now().UTC().Format("2006-01-02 15:04:05")).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// CleanExpired 清理已过期条目
func (d *JWTBlacklistDAO) CleanExpired(now time.Time) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`DELETE FROM jwt_blacklist WHERE expires_at <= ?`, now.UTC().Format("2006-01-02 15:04:05"))
	return err
}
