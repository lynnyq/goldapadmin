package api

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	"github.com/lynnyq/goldapadmin/internal/service"
)

// Router API 路由
type Router struct {
	svc *service.LdapService
}

// NewRouter 创建路由
func NewRouter(svc *service.LdapService) *Router {
	return &Router{svc: svc}
}

// mustConnParams 获取连接参数；凭据失效时写入 401 响应并返回 nil（前端收到 401 会跳转登录页）
func (r *Router) mustConnParams(c *gin.Context) *service.ConnParams {
	params, err := r.svc.GetConnParams(c)
	if err != nil {
		if errors.Is(err, service.ErrCredentialsExpired) {
			response.Fail(c, 401, err.Error())
		} else {
			response.Fail(c, 500, err.Error())
		}
		return nil
	}
	return params
}

// Register 注册路由
func (r *Router) Register(engine *gin.Engine) {
	api := engine.Group("/api")

	// 公开接口
	api.POST("/auth/login", r.Login)
	api.POST("/auth/logout", r.Logout)
	api.GET("/health", middleware.StatusOK)
	// 获取服务器列表需要公开，供登录页选择已保存的服务器
	api.GET("/servers", r.ListServers)

	// 需要认证的接口
	auth := api.Group("")
	auth.Use(middleware.Auth())
	{
		// 服务器管理（增删改需要认证）
		auth.POST("/servers", r.CreateServer)
		auth.PUT("/servers/:id", r.UpdateServer)
		auth.DELETE("/servers/:id", r.DeleteServer)
		auth.POST("/servers/test", r.TestConnection)

		// 条目操作
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
		// 对标 phpLDAPadmin compare.php — LDAP Compare 操作
		entry.POST("/compare", r.CompareEntry)
		// 对标 phpLDAPadmin view_jpeg_photo.php / download_binary_attr.php
		entry.GET("/binary", r.GetBinaryAttr)
		entry.GET("/binary-download", r.DownloadBinaryAttr)
		// 对标 phpLDAPadmin delete_form.php — 先列出子孙再确认
		entry.GET("/descendants", r.ListDescendants)
		// 对标 phpLDAPadmin mass_delete.php / mass_edit.php — 批量操作
		entry.DELETE("/batch", r.BatchDelete)
		entry.POST("/batch-modify", r.BatchModify)

		// rootDSE — 对标 phpLDAPadmin server_info.php
		auth.GET("/rootdse", r.GetRootDSE)

		// Schema
		schema := auth.Group("/schema")
		schema.GET("/objectclasses", r.GetObjectClasses)
		schema.GET("/attributetypes", r.GetAttributeTypes)
		schema.GET("/objectclass/:name", r.GetObjectClass)
		schema.GET("/attribute/:name", r.GetAttribute)
		schema.POST("/allowed-attributes", r.GetAllowedAttributes)

		// 搜索
		auth.GET("/search", r.Search)

		// LDIF
		ldif := auth.Group("/ldif")
		ldif.POST("/import", r.ImportLdif)
		ldif.GET("/export", r.ExportLdif)

		// 书签
		bookmark := auth.Group("/bookmarks")
		bookmark.GET("", r.ListBookmarks)
		bookmark.POST("", r.CreateBookmark)
		bookmark.DELETE("/:id", r.DeleteBookmark)

		// 监控
		auth.GET("/monitor/stats", r.GetMonitorStats)

		// 日志
		auth.GET("/logs", r.GetLogs)
	}
}
