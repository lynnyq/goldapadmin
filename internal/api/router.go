package api

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	"github.com/lynnyq/goldapadmin/internal/service"
)

// RouterOptions 路由层安全/运行参数
type RouterOptions struct {
	JWTTTL       time.Duration
	MaxBodyBytes int64
	ReadOnly     bool
}

// Router API 路由
type Router struct {
	svc  *service.LdapService
	opts RouterOptions
}

// NewRouter 创建路由
func NewRouter(svc *service.LdapService, opts RouterOptions) *Router {
	if opts.JWTTTL <= 0 {
		opts.JWTTTL = 24 * time.Hour
	}
	return &Router{svc: svc, opts: opts}
}

// ReadOnlyGuard 只读模式守卫：拒绝修改 LDAP 数据的请求
//
// 仅拦截真正写入目录的操作；登录、查询、测试连接、书签等不影响 LDAP 数据，不受限制。
func (r *Router) ReadOnlyGuard() gin.HandlerFunc {
	blocked := map[string]struct{}{
		"/api/entry/create":       {},
		"/api/entry/modify":       {},
		"/api/entry/delete":       {},
		"/api/entry/copy":         {},
		"/api/entry/move":         {},
		"/api/entry/rename":       {},
		"/api/entry/batch":        {},
		"/api/entry/batch-modify": {},
		"/api/ldif/import":        {},
	}
	return func(c *gin.Context) {
		if r.opts.ReadOnly {
			if _, ok := blocked[c.Request.URL.Path]; ok {
				response.Fail(c, 403, "系统处于只读模式，禁止修改 LDAP 数据")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// AdminOnly 仅允许 security.admin_dns 白名单内的登录 DN 访问（fail-closed）
func (r *Router) AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.svc.IsAdmin(middleware.GetUserDN(c)) {
			response.Fail(c, 403, "无权限：仅管理员可管理 LDAP 服务器配置")
			c.Abort()
			return
		}
		c.Next()
	}
}

// mustConnParams 获取连接参数；凭据失效时写入 401 响应并返回 nil
func (r *Router) mustConnParams(c *gin.Context) *service.ConnParams {
	params, err := r.svc.GetConnParams(c)
	if err != nil {
		if errors.Is(err, service.ErrCredentialsExpired) {
			response.Fail(c, 401, err.Error())
		} else {
			response.Fail(c, 500, "获取连接参数失败")
		}
		return nil
	}
	return params
}

// Register 注册路由
//
// 安全中间件叠加顺序（由外到内）：
//
//	engine 安全响应头 → /api 分组（体积上限 + 宽松限流 120/min/IP）
//	  → /api/auth/login 用 StrictRateLimit (20/min/IP)
//	  → 公开：/api/auth/login、/api/auth/logout、/api/health、/api/servers/public（仅最小字段）
//	  → auth group: Auth() 鉴权
//	     → admin group: AdminOnly()（服务器配置 CRUD / 测试连接）
//	     → 其余业务接口
func (r *Router) Register(engine *gin.Engine) {
	api := engine.Group("/api")
	api.Use(middleware.BodyLimit(r.opts.MaxBodyBytes))
	api.Use(middleware.RateLimit())

	// ---- 公开接口 ----

	// Login — 额外叠加严格限流，防暴力破解
	api.POST("/auth/login", middleware.RateLimitStrict(), r.Login)

	api.POST("/auth/logout", r.Logout)
	api.GET("/health", middleware.StatusOK)
	// 登录页只暴露展示所需的最小字段（不含 bind_dn），避免未认证泄漏管理员账号信息
	api.GET("/servers/public", r.ListPublicServers)

	// ---- 需要认证的接口 ----
	auth := api.Group("")
	auth.Use(middleware.Auth())
	auth.Use(r.ReadOnlyGuard())
	{
		// 服务器列表：登录用户可见（含完整字段），供顶部下拉选择器使用
		auth.GET("/servers", r.ListServers)

		// 服务器配置写操作（新增/修改/删除/测试连接）属于运维敏感操作，
		// 仅 security.admin_dns 白名单内的登录 DN 可访问，防止劫持 LDAP 连接
		admin := auth.Group("/servers")
		admin.Use(r.AdminOnly())
		{
			admin.POST("", r.CreateServer)
			admin.PUT("/:id", r.UpdateServer)
			admin.DELETE("/:id", r.DeleteServer)

			// TestConnection — 严格限流，防 LDAP 密码喷洒
			admin.POST("/test", middleware.RateLimitStrict(), r.TestConnection)
		}

		entry := auth.Group("/entry")
		entry.GET("/children", r.GetChildren)
		entry.GET("/tree", r.GetTree)
		entry.GET("/get", r.GetEntry)
		entry.GET("/next_id", r.NextID)
		entry.POST("/create", r.CreateEntry)
		entry.POST("/modify", r.ModifyEntry)
		entry.DELETE("/delete", r.DeleteEntry)
		entry.POST("/copy", r.CopyEntry)
		entry.POST("/move", r.MoveEntry)
		entry.POST("/rename", r.RenameEntry)
		entry.POST("/compare", r.CompareEntry)
		entry.GET("/binary", r.GetBinaryAttr)
		entry.GET("/binary-download", r.DownloadBinaryAttr)
		entry.GET("/descendants", r.ListDescendants)
		entry.DELETE("/batch", r.BatchDelete)
		entry.POST("/batch-modify", r.BatchModify)

		auth.GET("/rootdse", r.GetRootDSE)

		schema := auth.Group("/schema")
		schema.GET("/objectclasses", r.GetObjectClasses)
		schema.GET("/attributetypes", r.GetAttributeTypes)
		schema.GET("/objectclass/:name", r.GetObjectClass)
		schema.GET("/attribute/:name", r.GetAttribute)
		schema.POST("/allowed-attributes", r.GetAllowedAttributes)

		auth.GET("/search", r.Search)

		ldif := auth.Group("/ldif")
		ldif.POST("/import", r.ImportLdif)
		ldif.GET("/export", r.ExportLdif)

		bookmark := auth.Group("/bookmarks")
		bookmark.GET("", r.ListBookmarks)
		bookmark.POST("", r.CreateBookmark)
		bookmark.DELETE("/:id", r.DeleteBookmark)

		auth.GET("/monitor/stats", r.GetMonitorStats)

		auth.GET("/logs", r.GetLogs)
	}
}
