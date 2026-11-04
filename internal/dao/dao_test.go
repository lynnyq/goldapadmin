package dao

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lynnyq/goldapadmin/internal/model"
)

// setupTestDB 初始化测试数据库（临时文件）
func setupTestDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := Init(dbPath); err != nil {
		t.Fatalf("Init DB 失败: %v", err)
	}
	t.Cleanup(func() {
		GetDB().Close()
		os.RemoveAll(tmpDir)
	})
	return dbPath
}

// ========== ServerDAO 测试 ==========

func TestServerDAO_CRUD(t *testing.T) {
	setupTestDB(t)
	dao := NewServerDAO()

	// Create
	srv := &model.LdapServer{
		Name:    "Test Server",
		Host:    "127.0.0.1",
		Port:    389,
		BaseDN:  "dc=example,dc=com",
		BindDN:  "cn=admin,dc=example,dc=com",
		UseTLS:  false,
		UseSSL:  false,
		Timeout: 30,
	}

	if err := dao.Create(srv); err != nil {
		t.Fatalf("Create Server 失败: %v", err)
	}
	if srv.ID == 0 {
		t.Error("创建后 ID 应该被设置")
	}
	if srv.CreatedAt.IsZero() {
		t.Error("CreatedAt 应该被设置")
	}

	// Get
	got, err := dao.Get(srv.ID)
	if err != nil {
		t.Fatalf("Get Server 失败: %v", err)
	}
	if got == nil {
		t.Fatal("应该能获取到服务器")
	}
	if got.Name != srv.Name {
		t.Errorf("Name = %q, 期望 %q", got.Name, srv.Name)
	}
	if got.Port != 389 {
		t.Errorf("Port = %d, 期望 389", got.Port)
	}

	// List
	list, err := dao.List()
	if err != nil {
		t.Fatalf("List Servers 失败: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List 数量 = %d, 期望 1", len(list))
	}

	// Update
	got.Host = "192.168.1.1"
	got.UseTLS = true
	if err := dao.Update(got); err != nil {
		t.Fatalf("Update Server 失败: %v", err)
	}

	updated, _ := dao.Get(srv.ID)
	if updated.Host != "192.168.1.1" {
		t.Errorf("更新后 Host = %q, 期望 %q", updated.Host, "192.168.1.1")
	}
	if !updated.UseTLS {
		t.Error("更新后 UseTLS 应该为 true")
	}

	// Delete
	if err := dao.Delete(srv.ID); err != nil {
		t.Fatalf("Delete Server 失败: %v", err)
	}

	deleted, err := dao.Get(srv.ID)
	if err != nil {
		t.Fatalf("Get 删除后应该不报错: %v", err)
	}
	if deleted != nil {
		t.Error("删除后 Get 应该返回 nil")
	}
}

func TestServerDAO_GetNotFound(t *testing.T) {
	setupTestDB(t)
	dao := NewServerDAO()

	got, err := dao.Get(999)
	if err != nil {
		t.Fatalf("不存在的 ID 不应该报错: %v", err)
	}
	if got != nil {
		t.Error("不存在的 ID 应该返回 nil")
	}
}

func TestServerDAO_MultipleServers(t *testing.T) {
	setupTestDB(t)
	dao := NewServerDAO()

	for i := 0; i < 3; i++ {
		srv := &model.LdapServer{
			Name:   "Server " + string(rune('a'+i)),
			Host:   "127.0.0.1",
			Port:   389,
			BaseDN: "dc=example,dc=com",
		}
		if err := dao.Create(srv); err != nil {
			t.Fatalf("第 %d 次 Create 失败: %v", i, err)
		}
	}

	list, err := dao.List()
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("List 数量 = %d, 期望 3", len(list))
	}
}

// ========== BookmarkDAO 测试 ==========

func TestBookmarkDAO_CRUD(t *testing.T) {
	setupTestDB(t)
	srvDAO := NewServerDAO()
	bmDAO := NewBookmarkDAO()

	srv := &model.LdapServer{Name: "Test", Host: "127.0.0.1", Port: 389}
	srvDAO.Create(srv)

	// Create
	bm := &model.Bookmark{
		ServerID: srv.ID,
		Name:     "Admin User",
		DN:       "cn=admin,dc=example,dc=com",
	}
	if err := bmDAO.Create(bm); err != nil {
		t.Fatalf("Create Bookmark 失败: %v", err)
	}
	if bm.ID == 0 {
		t.Error("创建后 ID 应该被设置")
	}

	// ListByServer
	list, err := bmDAO.ListByServer(srv.ID)
	if err != nil {
		t.Fatalf("ListByServer 失败: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListByServer 数量 = %d, 期望 1", len(list))
	}
	if list[0].Name != "Admin User" {
		t.Errorf("Name = %q, 期望 %q", list[0].Name, "Admin User")
	}

	// Delete
	if err := bmDAO.Delete(bm.ID); err != nil {
		t.Fatalf("Delete Bookmark 失败: %v", err)
	}

	list, _ = bmDAO.ListByServer(srv.ID)
	if len(list) != 0 {
		t.Errorf("删除后 ListByServer 应该返回 0 条, 得到 %d", len(list))
	}
}

// ========== LogDAO 测试 ==========

func TestLogDAO_CRUD(t *testing.T) {
	setupTestDB(t)
	srvDAO := NewServerDAO()
	logDAO := NewLogDAO()

	srv := &model.LdapServer{Name: "Test", Host: "127.0.0.1", Port: 389}
	srvDAO.Create(srv)

	// 创建多条日志
	for i := 0; i < 5; i++ {
		status := "success"
		if i == 2 {
			status = "failed"
		}
		log := &model.OperationLog{
			ServerID:   srv.ID,
			UserDN:     "cn=admin,dc=example,dc=com",
			Action:     "list",
			TargetDN:   "dc=example,dc=com",
			Status:     status,
			DurationMS: int64(10 + i),
		}
		if err := logDAO.Create(log); err != nil {
			t.Fatalf("Create Log %d 失败: %v", i, err)
		}
	}

	// List
	list, total, err := logDAO.List(srv.ID, 1, 10)
	if err != nil {
		t.Fatalf("List Logs 失败: %v", err)
	}
	if total != 5 {
		t.Errorf("Total = %d, 期望 5", total)
	}
	if len(list) != 5 {
		t.Errorf("List 数量 = %d, 期望 5", len(list))
	}

	// StatsByServer
	totalCnt, successCnt, failedCnt, avgMs, err := logDAO.StatsByServer(srv.ID)
	if err != nil {
		t.Fatalf("StatsByServer 失败: %v", err)
	}
	if totalCnt != 5 {
		t.Errorf("Total 统计 = %d, 期望 5", totalCnt)
	}
	if successCnt != 4 {
		t.Errorf("Success 统计 = %d, 期望 4", successCnt)
	}
	if failedCnt != 1 {
		t.Errorf("Failed 统计 = %d, 期望 1", failedCnt)
	}
	if avgMs <= 0 {
		t.Errorf("AvgMs = %d, 应该 > 0", avgMs)
	}

	// CountByAction
	byAction, err := logDAO.CountByAction(srv.ID)
	if err != nil {
		t.Fatalf("CountByAction 失败: %v", err)
	}
	if byAction["list"] != 5 {
		t.Errorf("list 操作计数 = %d, 期望 5", byAction["list"])
	}

	// RecentResponseTimes
	times, err := logDAO.RecentResponseTimes(srv.ID, 10)
	if err != nil {
		t.Fatalf("RecentResponseTimes 失败: %v", err)
	}
	if len(times) != 5 {
		t.Errorf("RecentResponseTimes 数量 = %d, 期望 5", len(times))
	}
}

func TestLogDAO_Pagination(t *testing.T) {
	setupTestDB(t)
	srvDAO := NewServerDAO()
	logDAO := NewLogDAO()

	srv := &model.LdapServer{Name: "Test", Host: "127.0.0.1", Port: 389}
	srvDAO.Create(srv)

	// 创建 25 条日志
	for i := 0; i < 25; i++ {
		logDAO.Create(&model.OperationLog{
			ServerID:   srv.ID,
			UserDN:     "cn=admin,dc=example,dc=com",
			Action:     "list",
			TargetDN:   "dc=example,dc=com",
			Status:     "success",
			DurationMS: 50,
		})
	}

	// 第 1 页，每页 10 条
	page1, total, _ := logDAO.List(srv.ID, 1, 10)
	if total != 25 {
		t.Errorf("Total = %d, 期望 25", total)
	}
	if len(page1) != 10 {
		t.Errorf("Page1 数量 = %d, 期望 10", len(page1))
	}

	// 第 3 页，应该有 5 条
	page3, _, _ := logDAO.List(srv.ID, 3, 10)
	if len(page3) != 5 {
		t.Errorf("Page3 数量 = %d, 期望 5", len(page3))
	}
}
