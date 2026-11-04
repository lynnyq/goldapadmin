package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var embedFS embed.FS

// Register 注册前端静态资源
func Register(r *gin.Engine) {
	sub, err := fs.Sub(embedFS, "dist")
	if err != nil {
		panic("load embedded frontend: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	// 静态资源目录（Vite 产物带内容哈希文件名，可长缓存）
	r.GET("/assets/*filepath", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	// favicon
	r.GET("/favicon.svg", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.FileFromFS("/favicon.svg", http.FS(sub))
	})
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.FileFromFS("/favicon.svg", http.FS(sub))
	})
	r.GET("/icons.svg", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.FileFromFS("/icons.svg", http.FS(sub))
	})

	// SPA 回退：所有非 /api 请求返回 index.html
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
			return
		}
		// index.html 每次都重新校验，避免发版后浏览器加载旧版 JS
		c.Header("Cache-Control", "no-cache")
		// 直接返回 index.html 内容
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}
