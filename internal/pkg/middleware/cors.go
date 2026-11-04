package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件：基于白名单允许跨域
//
// origins 为允许的 Origin 列表；传入空数组时，不设置任何 CORS 头（即只允许同源请求）。
// 支持通配符 "*" 表示允许所有来源（开发便利，生产环境禁止）。
func CORS(origins []string) gin.HandlerFunc {
	// 构建快速查找 map
	allowAll := false
	originSet := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "*" {
			allowAll = true
		} else if o != "" {
			originSet[strings.ToLower(o)] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			// 请求不带 Origin（同源请求或非浏览器），直接放行，不注入 CORS 头
			c.Next()
			return
		}

		// 匹配白名单
		allow := allowAll
		if !allowAll {
			if _, ok := originSet[strings.ToLower(origin)]; ok {
				allow = true
			}
		}

		if allow {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Trace-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
