package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/dao"
	"github.com/lynnyq/goldapadmin/internal/model"
	"github.com/lynnyq/goldapadmin/internal/pkg/auth"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
	"github.com/lynnyq/goldapadmin/internal/service"
)

// LoginRequest 登录请求
type LoginRequest struct {
	ServerID int    `json:"server_id"`
	Name     string `json:"name"`      // 记住服务器时可选的连接名称（为空则用 Host 兜底）
	Host     string `json:"host"`
	Port     int    `json:"port"`
	BaseDN   string `json:"base_dn"`
	BindDN   string `json:"bind_dn"`
	Password string `json:"password"`
	UseTLS   bool   `json:"use_tls"`
	UseSSL   bool   `json:"use_ssl"`
	Remember bool   `json:"remember"`
}

// 登录失败锁定策略：同一 (服务器, 绑定 DN) 连续失败 5 次后锁定 15 分钟
const (
	loginMaxFailures = 5
	loginLockWindow  = 15 * time.Minute
	loginFailWindow  = 15 * time.Minute
)

type loginAttempt struct {
	failures    int
	lockedUntil time.Time
	lastSeen    time.Time
}

var (
	loginAttempts    sync.Map // key: server|bindDN
	loginCleanupOnce sync.Once
	loginAttemptsMu  sync.Mutex
)

// loginAttemptKey 生成失败计数 key（临时服务器用 host:port 区分）
func loginAttemptKey(server *model.LdapServer, bindDN string) string {
	id := strconv.Itoa(server.ID)
	if server.ID == 0 {
		id = server.Host + ":" + strconv.Itoa(server.Port)
	}
	return id + "|" + strings.ToLower(strings.TrimSpace(bindDN))
}

// beginLoginAttempt 检查是否处于锁定期；返回剩余锁定时间
func beginLoginAttempt(key string) time.Duration {
	loginCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for now := range ticker.C {
				loginAttempts.Range(func(k, v any) bool {
					a := v.(loginAttempt)
					if now.Sub(a.lastSeen) > 2*loginLockWindow {
						loginAttempts.Delete(k)
					}
					return true
				})
			}
		}()
	})

	if v, ok := loginAttempts.Load(key); ok {
		a := v.(loginAttempt)
		if time.Now().Before(a.lockedUntil) {
			return time.Until(a.lockedUntil)
		}
	}
	return 0
}

// recordLoginFailure 记录一次失败，达到阈值后锁定
func recordLoginFailure(key string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	now := time.Now()
	var a loginAttempt
	if v, ok := loginAttempts.Load(key); ok {
		a = v.(loginAttempt)
		if now.Sub(a.lastSeen) > loginFailWindow {
			a.failures = 0
		}
	}
	a.failures++
	a.lastSeen = now
	if a.failures >= loginMaxFailures {
		a.lockedUntil = now.Add(loginLockWindow)
		a.failures = 0
	}
	loginAttempts.Store(key, a)
}

// resetLoginAttempt 登录成功后清除失败计数
func resetLoginAttempt(key string) {
	loginAttempts.Delete(key)
}

// Login 登录
func (r *Router) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "参数错误")
		return
	}

	// 参数组合校验：
	//   - bind_dn 非空但密码为空 → 拒绝（SIMPLE 绑定必须有密码）
	//   - bind_dn 为空 → 匿名登录，需 security.allow_anonymous_bind=true
	if req.BindDN != "" && req.Password == "" {
		response.Fail(c, 400, "已填写绑定 DN，但密码为空。SIMPLE 绑定需要密码；如需匿名连接请清空绑定 DN 字段")
		return
	}

	var server *model.LdapServer
	if req.ServerID > 0 {
		s, err := r.svc.GetServerDAO().Get(req.ServerID)
		if err != nil || s == nil {
			response.Fail(c, 404, "服务器不存在")
			return
		}
		server = s
	} else {
		// 使用临时服务器配置
		if strings.TrimSpace(req.Host) == "" {
			response.Fail(c, 400, "服务器地址不能为空")
			return
		}
		if req.Port < 0 || req.Port > 65535 {
			response.Fail(c, 400, "服务器端口非法")
			return
		}
		server = &model.LdapServer{
			Host:    req.Host,
			Port:    req.Port,
			BaseDN:  req.BaseDN,
			UseTLS:  req.UseTLS,
			UseSSL:  req.UseSSL,
			Timeout: 30,
		}
	}

	attemptKey := loginAttemptKey(server, req.BindDN)
	if wait := beginLoginAttempt(attemptKey); wait > 0 {
		response.Fail(c, 429, fmt.Sprintf("登录失败次数过多，请 %d 秒后重试", int(wait.Seconds())+1))
		return
	}

	start := time.Now()
	// 尝试 LDAP Bind 验证（走服务层统一执行 TLS/匿名策略，且不复用连接池）
	conn, err := r.svc.VerifyBind(server, req.BindDN, req.Password)
	if err != nil {
		recordLoginFailure(attemptKey)
		// 详细错误只写服务端日志/审计，响应返回通用提示，避免泄漏服务器内部信息
		detail := "LDAP 认证失败: " + err.Error()
		fmt.Printf("[LOGIN] failed server=%s:%d dn=%q err=%v\n", server.Host, server.Port, req.BindDN, err)
		r.svc.LogOperation(server.ID, req.BindDN, "login", "", "", "failed", detail, time.Since(start))
		response.Fail(c, 401, "LDAP 认证失败，请检查绑定 DN/密码或服务器配置")
		return
	}
	defer conn.Close()
	resetLoginAttempt(attemptKey)

	// 如果是记住服务器，保存到数据库（仅管理员，避免普通用户写入服务器配置）
	if req.Remember && req.ServerID == 0 && r.svc.IsAdmin(req.BindDN) {
		if strings.TrimSpace(req.Name) != "" {
			server.Name = strings.TrimSpace(req.Name)
		} else {
			server.Name = req.Host
		}
		server.BindDN = req.BindDN
		if err := r.svc.GetServerDAO().Create(server); err != nil {
			// 保存失败不影响登录
		}
	}

	// 生成 JWT（TTL 取自配置 jwt.ttl）
	token, err := auth.GenerateToken(server.ID, req.BindDN, r.opts.JWTTTL)
	if err != nil {
		response.Fail(c, 500, "生成 token 失败")
		return
	}

	// 缓存服务器配置与绑定凭据，供登录后的业务接口建立 LDAP 连接使用
	if !service.CacheSession(token, server, req.Password) {
		response.Fail(c, 503, "登录会话数已达上限，请稍后重试")
		return
	}
	r.svc.LogOperation(server.ID, req.BindDN, "login", "", "", "success", "", time.Since(start))

	// 返回服务器信息时剔除 bind_dn，避免登录成功后向普通用户泄漏管理员账号 DN
	response.Success(c, gin.H{
		"token":   token,
		"user_dn": req.BindDN,
		"server": gin.H{
			"id":      server.ID,
			"name":    server.Name,
			"host":    server.Host,
			"port":    server.Port,
			"base_dn": server.BaseDN,
			"use_tls": server.UseTLS,
			"use_ssl": server.UseSSL,
		},
	})
}

// Logout 登出：让 JWT 立即失效并清除服务端凭据缓存
//
// Logout 位于公开路由组（不需要 Auth 中间件），所以自行解析 Authorization header。
// 同时支持携带或不携带 token 的情况（客户端可能已清理本地 token 后再调用）。
func (r *Router) Logout(c *gin.Context) {
	tokenString := ""
	claims := (*auth.Claims)(nil)

	header := c.GetHeader("Authorization")
	if header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString = parts[1]
			// 尝试解析以拿 expire 时间；解析失败时 blacklist 也会用默认 24h
			if cl, err := auth.ParseToken(tokenString); err == nil {
				claims = cl
			}
		}
	}

	if tokenString != "" {
		auth.AddToBlacklist(tokenString, claims)
		service.ClearSession(tokenString)
	}

	response.Success(c, nil)
}

// GetServerDAO 获取服务器 DAO
func (r *Router) GetServerDAO() *dao.ServerDAO {
	return r.svc.GetServerDAO()
}

// 确保 middleware 包被引用
var _ = middleware.GetTraceID
var _ = http.StatusOK
