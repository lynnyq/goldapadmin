package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceID 注入 TraceID 中间件
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = "trace-" + uuid.NewString()[:8]
		}
		c.Set("trace_id", traceID)
		c.Header("X-Trace-ID", traceID)
		c.Next()
	}
}

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		traceID := ""
		if v, ok := c.Get("trace_id"); ok {
			if s, ok := v.(string); ok {
				traceID = s
			}
		}

		gin.DefaultWriter.Write([]byte(
			method + " " + path + " " +
				statusString(status) + " " +
				latency.String() + " trace_id=" + traceID + "\n",
		))
	}
}

func statusString(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "xxx"
	}
}
