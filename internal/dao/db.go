package dao

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// Init 初始化数据库
func Init(dbPath string) error {
	dir := filepath.Dir(dbPath)
	// 目录 0750、数据库文件 0600：SQLite 内含服务器配置与操作日志（用户 DN），需限制其他用户读取
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	if err := os.Chmod(dir, 0750); err != nil {
		return fmt.Errorf("chmod db dir: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	// SQLite 首次打开才创建文件，此时收紧权限
	if err := os.Chmod(dbPath, 0600); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("chmod db file: %w", err)
	}

	if err := migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

// GetDB 获取数据库连接
func GetDB() *sql.DB {
	return db
}

func migrate() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS ldap_servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 389,
			base_dn TEXT NOT NULL DEFAULT '',
			bind_dn TEXT NOT NULL DEFAULT '',
			use_tls INTEGER NOT NULL DEFAULT 0,
			use_ssl INTEGER NOT NULL DEFAULT 0,
			skip_tls_verify INTEGER NOT NULL DEFAULT 0,
			timeout INTEGER NOT NULL DEFAULT 30,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bookmarks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			dn TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS operation_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_id INTEGER NOT NULL,
			user_dn TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			target_dn TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'success',
			error_msg TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS jwt_blacklist (
			token TEXT PRIMARY KEY,
			expires_at DATETIME NOT NULL
		)`,
	}

	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec schema: %w", err)
		}
	}

	// 向后兼容：为早期数据库补齐 skip_tls_verify 列
	if err := ensureColumn("ldap_servers", "skip_tls_verify", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("ensure skip_tls_verify: %w", err)
	}

	return nil
}

// ensureColumn 如果列不存在则 ALTER TABLE 补齐（SQLite 不支持 IF NOT EXISTS 的 ALTER）
func ensureColumn(table, column, colType string) error {
	// 先查 schema 确认列是否存在，避免盲目 ALTER 触发错误
	var colCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&colCount)
	if err != nil {
		return fmt.Errorf("查询 pragma_table_info 失败: %w", err)
	}
	if colCount > 0 {
		return nil // 列已存在，直接跳过
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
	return err
}
