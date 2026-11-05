package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// SecurityHeaders 注入基础安全响应头
//
// CSP 允许同源脚本与样式（Vue/Element Plus 会注入内联样式，故 style-src 需 unsafe-inline），
// 禁止内联脚本与对象嵌入，降低 XSS 与点击劫持风险。仅在直接以 TLS 提供服务时下发 HSTS，
// 反向代理终结 TLS 的场景由代理层负责（见 deploy/goldapadmin-nginx.conf）。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self' data:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"object-src 'none'")
		if c.Request.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// BodyLimit 限制请求体大小，超限直接返回 413
//
// 用于阻止 LDIF 导入、批量操作等超大报文耗尽内存。
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			c.Header("Connection", "close")
			c.JSON(http.StatusRequestEntityTooLarge, response.Response{
				Code:    http.StatusRequestEntityTooLarge,
				Message: "请求体过大",
				TraceID: traceIDOf(c),
			})
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// traceIDOf 读取当前请求 TraceID（避免与 response 包循环依赖）
func traceIDOf(c *gin.Context) string {
	if v, ok := c.Get("trace_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
