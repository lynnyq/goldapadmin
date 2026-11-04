package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/pkg/auth"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// Auth JWT 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.Fail(c, 401, "未登录")
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Fail(c, 401, "认证格式错误")
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 先查黑名单（Logout 过的 token 立即失效）
		if auth.IsBlacklisted(tokenString) {
			response.Fail(c, 401, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			response.Fail(c, 401, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		c.Set("token", tokenString)
		c.Set("server_id", claims.ServerID)
		c.Set("user_dn", claims.UserDN)
		c.Set("claims", claims)
		c.Next()
	}
}

// GetToken 从上下文中获取原始 JWT token
func GetToken(c *gin.Context) string {
	if v, ok := c.Get("token"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetServerID 从上下文中获取服务器 ID
func GetServerID(c *gin.Context) int {
	if v, ok := c.Get("server_id"); ok {
		if id, ok := v.(int); ok {
			return id
		}
	}
	return 0
}

// GetUserDN 从上下文中获取用户 DN
func GetUserDN(c *gin.Context) string {
	if v, ok := c.Get("user_dn"); ok {
		if dn, ok := v.(string); ok {
			return dn
		}
	}
	return ""
}

// GetTraceID 从上下文中获取 TraceID
func GetTraceID(c *gin.Context) string {
	if v, ok := c.Get("trace_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// StatusOK 健康检查
func StatusOK(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
