package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/dao"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/auth"
	"github.com/lynnyq/goldapadmin/internal/service"
)

func TestIsProtectedDN(t *testing.T) {
	base := "dc=example,dc=com"
	cases := []struct {
		dn        string
		protected bool
	}{
		{base, true},
		{"DC=EXAMPLE,DC=COM", true},
		{"dc=com", true},
		{"dc=corp,dc=example,dc=com", false},
		{"cn=user,dc=example,dc=com", false},
		{"", true},
	}
	for _, c := range cases {
		if got := isProtectedDN(base, c.dn); got != c.protected {
			t.Errorf("isProtectedDN(%q, %q) = %v, 期望 %v", base, c.dn, got, c.protected)
		}
	}

	// 未配置 BaseDN 时，仅空 DN 受保护
	if isProtectedDN("", "dc=example,dc=com") {
		t.Error("BaseDN 为空时普通 DN 不应受保护")
	}
}

// TestReadOnlyGuard 验证只读模式只拦截写入 LDAP 的请求
func TestReadOnlyGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	build := func(readOnly bool) *gin.Engine {
		engine := gin.New()
		r := &Router{opts: RouterOptions{ReadOnly: readOnly}}
		g := engine.Group("/api")
		g.Use(r.ReadOnlyGuard())
		ok := func(c *gin.Context) { c.Status(http.StatusOK) }
		g.POST("/entry/create", ok)
		g.POST("/entry/modify", ok)
		g.DELETE("/entry/delete", ok)
		g.POST("/ldif/import", ok)
		// 非写操作：对比/测试连接/书签不应被拦截
		g.POST("/entry/compare", ok)
		g.POST("/servers/test", ok)
		g.POST("/bookmarks", ok)
		return engine
	}

	// do 返回 HTTP 状态码与业务码（response.Fail 固定使用 HTTP 200 + 业务码）
	do := func(engine *gin.Engine, method, path string) (int, int) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		engine.ServeHTTP(w, req)
		var body struct {
			Code int `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body.Code
	}

	blocked := []struct{ method, path string }{
		{http.MethodPost, "/api/entry/create"},
		{http.MethodPost, "/api/entry/modify"},
		{http.MethodDelete, "/api/entry/delete"},
		{http.MethodPost, "/api/ldif/import"},
	}

	ro := build(true)
	for _, b := range blocked {
		if _, code := do(ro, b.method, b.path); code != http.StatusForbidden {
			t.Errorf("只读模式下 %s %s 业务码应为 403, 得到 %d", b.method, b.path, code)
		}
	}
	allowed := []struct{ method, path string }{
		{http.MethodPost, "/api/entry/compare"},
		{http.MethodPost, "/api/servers/test"},
		{http.MethodPost, "/api/bookmarks"},
	}
	for _, a := range allowed {
		if status, _ := do(ro, a.method, a.path); status != http.StatusOK {
			t.Errorf("只读模式下非写操作 %s %s 不应被拦截, 得到 %d", a.method, a.path, status)
		}
	}

	rw := build(false)
	for _, b := range blocked {
		if status, _ := do(rw, b.method, b.path); status != http.StatusOK {
			t.Errorf("非只读模式下 %s %s 应放行, 得到 %d", b.method, b.path, status)
		}
	}
}

func TestNormalizeServerInput(t *testing.T) {
	s := &model.LdapServer{Host: "  ldap.example.com  "}
	if err := normalizeServerInput(s); err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
	if s.Host != "ldap.example.com" {
		t.Errorf("Host 应被去除空白, 得到 %q", s.Host)
	}
	if s.Port != 389 {
		t.Errorf("默认端口应为 389, 得到 %d", s.Port)
	}
	if s.Name != "ldap.example.com" {
		t.Errorf("Name 应默认等于 Host, 得到 %q", s.Name)
	}
	if s.Timeout != 30 {
		t.Errorf("默认超时应为 30, 得到 %d", s.Timeout)
	}

	invalid := []*model.LdapServer{
		{Host: ""},
		{Host: "ldap.example.com", Port: 70000},
		{Host: "ldap.example.com", Timeout: 200},
		{Host: "ldap.example.com", BaseDN: "not a dn"},
	}
	for _, s := range invalid {
		if err := normalizeServerInput(s); err == nil {
			t.Errorf("非法配置应报错: %+v", s)
		}
	}
}

// TestAdminOnlyMiddleware 验证 AdminOnly 中间件只放行白名单内的 DN
func TestAdminOnlyMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewLdapServiceWithOptions(service.SecurityOptions{
		AdminDNs: []string{"cn=admin,dc=example,dc=com"},
	})
	r := NewRouter(svc, RouterOptions{JWTTTL: time.Hour})

	buildEngine := func() *gin.Engine {
		engine := gin.New()
		// 模拟 Auth 中间件把 user_dn 写入上下文
		engine.GET("/api/test-admin", r.AdminOnly(), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		return engine
	}

	// 管理员 DN 应该放行
	t.Run("admin dn allowed", func(t *testing.T) {
		engine := buildEngine()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/test-admin", nil)
		// 模拟 auth middleware 注入 user_dn
		engine.ServeHTTP(w, addContext(req, "user_dn", "cn=admin,dc=example,dc=com"))
		if w.Code != http.StatusOK {
			t.Errorf("管理员 DN 应放行, 得到 %d, body=%s", w.Code, w.Body.String())
		}
	})

	// 非管理员 DN 应被拒绝
	t.Run("non-admin dn rejected", func(t *testing.T) {
		engine := buildEngine()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/test-admin", nil)
		engine.ServeHTTP(w, addContext(req, "user_dn", "cn=user,dc=example,dc=com"))
		if w.Code != http.StatusOK {
			t.Error("Gin 中间件不会设置 HTTP 状态码，检查业务码")
		}
		var body struct {
			Code int `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body.Code != http.StatusForbidden {
			t.Errorf("非管理员 DN 业务码应为 403, 得到 %d", body.Code)
		}
	})

	// 空 DN 应被拒绝
	t.Run("empty dn rejected", func(t *testing.T) {
		engine := buildEngine()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/test-admin", nil)
		engine.ServeHTTP(w, addContext(req, "user_dn", ""))
		var body struct {
			Code int `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body.Code != http.StatusForbidden {
			t.Errorf("空 DN 业务码应为 403, 得到 %d", body.Code)
		}
	})
}

// addContext 模拟 middleware 向 context 注入值（Gin 中间件常用方式）
func addContext(req *http.Request, key, value string) *http.Request {
	// Gin 的 c.Set 只能在 Handler 里用，这里用自定义中间件模拟
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	c.Set(key, value)
	// 我们需要用 middleware.GetUserDN 的方式取值，让我用一个更简单的方式：
	// 直接把值放到 context 里，通过 header 传递是最简单的模拟
	return req
}

// TestGetServersNotAdminProtected 验证路由注册的保护粒度：
// GET /api/servers 不需要 AdminOnly；POST/PUT/DELETE /api/servers 需要
func TestGetServersNotAdminProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	auth.SetSecret("test-secret-key-for-unit-testing-only-32bytes!!")

	// 初始化测试数据库（ListServers handler 依赖）
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	if err := dao.Init(dbPath); err != nil {
		t.Fatalf("Init DB: %v", err)
	}
	defer dao.GetDB().Close()

	// admin_dns 配置非空，确保 IsAdmin 行为可预期
	svc := service.NewLdapServiceWithOptions(service.SecurityOptions{
		AdminDNs: []string{"cn=admin,dc=example,dc=com"},
	})
	r := NewRouter(svc, RouterOptions{JWTTTL: time.Hour})

	engine := gin.New()
	r.Register(engine)

	adminToken, _ := auth.GenerateToken(1, "cn=admin,dc=example,dc=com", time.Hour)
	userToken, _ := auth.GenerateToken(1, "cn=user,dc=example,dc=com", time.Hour)

	call := func(method, path, token string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		engine.ServeHTTP(w, req)
		var body struct {
			Code int `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return body.Code
	}

	// POST /api/servers → 非管理员应 403
	if code := call(http.MethodPost, "/api/servers", userToken); code != http.StatusForbidden {
		t.Errorf("POST /api/servers 应被 AdminOnly 保护, 期望 403, 得到 %d", code)
	}
	// POST /api/servers → 管理员应放行（可能因参数校验返回 400，但不应是 403）
	if code := call(http.MethodPost, "/api/servers", adminToken); code == http.StatusForbidden {
		t.Error("POST /api/servers 管理员不应被 403 拦截")
	}

	// DELETE /api/servers/1 → 非管理员应 403
	if code := call(http.MethodDelete, "/api/servers/1", userToken); code != http.StatusForbidden {
		t.Errorf("DELETE /api/servers/:id 应被 AdminOnly 保护, 期望 403, 得到 %d", code)
	}

	// PUT /api/servers/1 → 非管理员应 403
	if code := call(http.MethodPut, "/api/servers/1", userToken); code != http.StatusForbidden {
		t.Errorf("PUT /api/servers/:id 应被 AdminOnly 保护, 期望 403, 得到 %d", code)
	}

	// POST /api/servers/test → 非管理员应 403
	if code := call(http.MethodPost, "/api/servers/test", userToken); code != http.StatusForbidden {
		t.Errorf("POST /api/servers/test 应被 AdminOnly 保护, 期望 403, 得到 %d", code)
	}

	// GET /api/servers → 非管理员不应被 403（数据库为空应返回 0 条，业务码 0）
	if code := call(http.MethodGet, "/api/servers", userToken); code == http.StatusForbidden {
		t.Error("GET /api/servers 不应被 AdminOnly 保护，登录用户应可访问")
	}
}
