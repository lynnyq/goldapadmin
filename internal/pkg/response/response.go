package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	TraceID string      `json:"trace_id,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
		TraceID: getTraceID(c),
	})
}

// Fail 失败响应
func Fail(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    nil,
		TraceID: getTraceID(c),
	})
}

// FailWithData 带数据的失败响应
func FailWithData(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    data,
		TraceID: getTraceID(c),
	})
}

func getTraceID(c *gin.Context) string {
	if v, ok := c.Get("trace_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
