package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/pkg/auth"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	return r
}

// ========== Auth 中间件测试 ==========

func TestAuthMiddleware_NoToken(t *testing.T) {
	r := setupTestRouter()
	r.GET("/protected", Auth(), func(c *gin.Context) {
		response.Success(c, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 401 {
		t.Errorf("业务码 = %d, 期望 401", resp.Code)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	r := setupTestRouter()
	r.GET("/protected", Auth(), func(c *gin.Context) {
		response.Success(c, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "InvalidFormat token123")
	r.ServeHTTP(w, req)

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 401 {
		t.Errorf("业务码 = %d, 期望 401", resp.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	r := setupTestRouter()
	r.GET("/protected", Auth(), func(c *gin.Context) {
		serverID := GetServerID(c)
		userDN := GetUserDN(c)
		response.Success(c, gin.H{"server_id": serverID, "user_dn": userDN})
	})

	token, _ := auth.GenerateToken(42, "cn=admin,dc=test,dc=com", time.Hour)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("认证失败, resp = %s", w.Body.String())
	}

	// 验证上下文中的数据
	dataBytes, _ := json.Marshal(resp.Data)
	var data map[string]interface{}
	json.Unmarshal(dataBytes, &data)

	if int(data["server_id"].(float64)) != 42 {
		t.Errorf("server_id = %v, 期望 42", data["server_id"])
	}
	if data["user_dn"] != "cn=admin,dc=test,dc=com" {
		t.Errorf("user_dn = %v, 期望 cn=admin,dc=test,dc=com", data["user_dn"])
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	r := setupTestRouter()
	r.GET("/protected", Auth(), func(c *gin.Context) {
		response.Success(c, "ok")
	})

	token, _ := auth.GenerateToken(1, "cn=user,dc=example,dc=com", -1) // 过期的 token

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 401 {
		t.Errorf("过期 token 应该返回 401, 得到 %d", resp.Code)
	}
}

// ========== TraceID 中间件测试 ==========

func TestTraceIDMiddleware(t *testing.T) {
	r := setupTestRouter()
	r.Use(TraceID())
	r.GET("/test", func(c *gin.Context) {
		traceID := GetTraceID(c)
		c.JSON(http.StatusOK, gin.H{"trace_id": traceID})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200", w.Code)
	}

	// 应该有 X-Trace-ID 响应头
	if w.Header().Get("X-Trace-ID") == "" {
		t.Error("应该设置 X-Trace-ID 响应头")
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["trace_id"] == "" {
		t.Error("应该有 trace_id 字段")
	}
}

func TestTraceIDMiddleware_ExistingHeader(t *testing.T) {
	r := setupTestRouter()
	r.Use(TraceID())
	r.GET("/test", func(c *gin.Context) {
		traceID := GetTraceID(c)
		c.JSON(http.StatusOK, gin.H{"trace_id": traceID})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Trace-ID", "custom-trace-id-123")
	r.ServeHTTP(w, req)

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["trace_id"] != "custom-trace-id-123" {
		t.Errorf("应该使用请求头中的 TraceID, 得到 %q", body["trace_id"])
	}
}

// ========== Logger 中间件测试 ==========

func TestLoggerMiddleware(t *testing.T) {
	r := setupTestRouter()
	var buf bytes.Buffer
	r.Use(TraceID())
	r.Use(Logger())
	r.GET("/hello", func(c *gin.Context) {
		c.String(http.StatusOK, "world")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hello", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200", w.Code)
	}

	_ = buf // Logger 写到 gin.DefaultWriter，这里不验证具体输出
}

// ========== StatusOK 测试 ==========

func TestStatusOK(t *testing.T) {
	r := setupTestRouter()
	r.GET("/health", StatusOK)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 200", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, 期望 'ok'", body["status"])
	}
}
