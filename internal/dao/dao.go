package dao

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lynnyq/goldapadmin/internal/model"
)

// ServerDAO 服务器配置数据访问
type ServerDAO struct{}

// NewServerDAO 创建服务器 DAO
func NewServerDAO() *ServerDAO {
	return &ServerDAO{}
}

// List 获取所有服务器
func (d *ServerDAO) List() ([]model.LdapServer, error) {
	rows, err := db.Query(`SELECT id, name, host, port, base_dn, bind_dn, use_tls, use_ssl, skip_tls_verify, timeout, created_at, updated_at FROM ldap_servers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.LdapServer
	for rows.Next() {
		var s model.LdapServer
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.BaseDN, &s.BindDN, &s.UseTLS, &s.UseSSL, &s.SkipTLSVerify, &s.Timeout, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// Get 根据 ID 获取服务器
func (d *ServerDAO) Get(id int) (*model.LdapServer, error) {
	var s model.LdapServer
	err := db.QueryRow(`SELECT id, name, host, port, base_dn, bind_dn, use_tls, use_ssl, skip_tls_verify, timeout, created_at, updated_at FROM ldap_servers WHERE id = ?`, id).
		Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.BaseDN, &s.BindDN, &s.UseTLS, &s.UseSSL, &s.SkipTLSVerify, &s.Timeout, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Create 创建服务器
func (d *ServerDAO) Create(s *model.LdapServer) error {
	res, err := db.Exec(`INSERT INTO ldap_servers (name, host, port, base_dn, bind_dn, use_tls, use_ssl, skip_tls_verify, timeout) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Name, s.Host, s.Port, s.BaseDN, s.BindDN, s.UseTLS, s.UseSSL, s.SkipTLSVerify, s.Timeout)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = int(id)
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return nil
}

// Update 更新服务器
func (d *ServerDAO) Update(s *model.LdapServer) error {
	_, err := db.Exec(`UPDATE ldap_servers SET name=?, host=?, port=?, base_dn=?, bind_dn=?, use_tls=?, use_ssl=?, skip_tls_verify=?, timeout=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		s.Name, s.Host, s.Port, s.BaseDN, s.BindDN, s.UseTLS, s.UseSSL, s.SkipTLSVerify, s.Timeout, s.ID)
	return err
}

// Delete 删除服务器
func (d *ServerDAO) Delete(id int) error {
	_, err := db.Exec(`DELETE FROM ldap_servers WHERE id=?`, id)
	return err
}

// BookmarkDAO 书签数据访问
type BookmarkDAO struct{}

// NewBookmarkDAO 创建书签 DAO
func NewBookmarkDAO() *BookmarkDAO {
	return &BookmarkDAO{}
}

// ListByServer 获取服务器下的书签
func (d *BookmarkDAO) ListByServer(serverID int) ([]model.Bookmark, error) {
	rows, err := db.Query(`SELECT id, server_id, name, dn, created_at FROM bookmarks WHERE server_id=? ORDER BY id DESC`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Bookmark
	for rows.Next() {
		var b model.Bookmark
		if err := rows.Scan(&b.ID, &b.ServerID, &b.Name, &b.DN, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

// Create 创建书签
func (d *BookmarkDAO) Create(b *model.Bookmark) error {
	res, err := db.Exec(`INSERT INTO bookmarks (server_id, name, dn) VALUES (?, ?, ?)`, b.ServerID, b.Name, b.DN)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = int(id)
	b.CreatedAt = time.Now()
	return nil
}

// Delete 删除书签
func (d *BookmarkDAO) Delete(id int) error {
	_, err := db.Exec(`DELETE FROM bookmarks WHERE id=?`, id)
	return err
}

// LogDAO 操作日志数据访问
type LogDAO struct{}

// NewLogDAO 创建日志 DAO
func NewLogDAO() *LogDAO {
	return &LogDAO{}
}

// Create 创建日志
func (d *LogDAO) Create(l *model.OperationLog) error {
	res, err := db.Exec(`INSERT INTO operation_logs (server_id, user_dn, action, target_dn, detail, status, error_msg, duration_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ServerID, l.UserDN, l.Action, l.TargetDN, l.Detail, l.Status, l.ErrorMsg, l.DurationMS)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	l.ID = int(id)
	l.CreatedAt = time.Now()
	return nil
}

// List 分页查询日志
func (d *LogDAO) List(serverID, page, pageSize int) ([]model.OperationLog, int, error) {
	offset := (page - 1) * pageSize
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM operation_logs WHERE server_id=?`, serverID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`SELECT id, server_id, user_dn, action, target_dn, detail, status, error_msg, duration_ms, created_at FROM operation_logs WHERE server_id=? ORDER BY id DESC LIMIT ? OFFSET ?`, serverID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []model.OperationLog
	for rows.Next() {
		var l model.OperationLog
		if err := rows.Scan(&l.ID, &l.ServerID, &l.UserDN, &l.Action, &l.TargetDN, &l.Detail, &l.Status, &l.ErrorMsg, &l.DurationMS, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, l)
	}
	return list, total, rows.Err()
}

// StatsByServer 获取服务器操作统计
func (d *LogDAO) StatsByServer(serverID int) (total, success, failed int, avgMs int64, err error) {
	err = db.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status='success' THEN 1 ELSE 0 END), SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END), AVG(duration_ms) FROM operation_logs WHERE server_id=?`, serverID).
		Scan(&total, &success, &failed, &avgMs)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return total, success, failed, avgMs, nil
}

// CountByAction 按操作类型统计
func (d *LogDAO) CountByAction(serverID int) (map[string]int, error) {
	rows, err := db.Query(`SELECT action, COUNT(*) FROM operation_logs WHERE server_id=? GROUP BY action`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var action string
		var count int
		if err := rows.Scan(&action, &count); err != nil {
			return nil, err
		}
		result[action] = count
	}
	return result, rows.Err()
}

// RecentResponseTimes 获取最近响应时间
func (d *LogDAO) RecentResponseTimes(serverID, limit int) ([]struct {
	Time  string `json:"time"`
	AvgMS int64  `json:"avg_ms"`
}, error) {
	rows, err := db.Query(`SELECT strftime('%H:%M:%S', created_at) as t, duration_ms FROM operation_logs WHERE server_id=? ORDER BY id DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []struct {
		Time  string `json:"time"`
		AvgMS int64  `json:"avg_ms"`
	}
	for rows.Next() {
		var item struct {
			Time  string `json:"time"`
			AvgMS int64  `json:"avg_ms"`
		}
		if err := rows.Scan(&item.Time, &item.AvgMS); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	// reverse to chronological order
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// Ensure imports used
var _ = fmt.Sprintf
