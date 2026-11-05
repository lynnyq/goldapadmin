package service

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/dao"
	"github.com/lynnyq/goldapadmin/internal/model"
	pkgldap "github.com/lynnyq/goldapadmin/internal/pkg/ldap"
	"github.com/lynnyq/goldapadmin/internal/pkg/middleware"
)

var errServerNotFound = errors.New("ldap server not found")

// ErrCredentialsExpired 登录凭据缓存失效（典型场景：服务重启后凭据缓存丢失，JWT 仍有效）
var ErrCredentialsExpired = errors.New("登录凭据已失效，请重新登录")

// SecurityOptions 服务层安全策略
type SecurityOptions struct {
	RequireLDAPTLS     bool
	AllowAnonymousBind bool
	AdminDNs           []string
}

// LdapService LDAP 业务服务
type LdapService struct {
	requireLDAPTLS     bool // 是否强制 LDAP 连接使用 TLS/SSL（来自 security.require_ldap_tls）
	allowAnonymousBind bool // 是否允许匿名绑定（来自 security.allow_anonymous_bind）
	adminDNs           map[string]struct{}
	serverDAO          *dao.ServerDAO
	bookmarkDAO        *dao.BookmarkDAO
	logDAO             *dao.LogDAO
	pool               *pkgldap.Pool // 连接池
}

// NewLdapService 创建 LDAP 服务
//
// requireLDAPTLS 对应配置项 security.require_ldap_tls，默认 false（允许明文连接）。
func NewLdapService(requireLDAPTLS bool) *LdapService {
	return NewLdapServiceWithOptions(SecurityOptions{RequireLDAPTLS: requireLDAPTLS})
}

// NewLdapServiceWithOptions 按完整安全策略创建 LDAP 服务，并启动凭据缓存清理协程
func NewLdapServiceWithOptions(opts SecurityOptions) *LdapService {
	admins := make(map[string]struct{}, len(opts.AdminDNs))
	for _, dn := range opts.AdminDNs {
		if normalized := normalizeDN(dn); normalized != "" {
			admins[normalized] = struct{}{}
		}
	}
	s := &LdapService{
		requireLDAPTLS:     opts.RequireLDAPTLS,
		allowAnonymousBind: opts.AllowAnonymousBind,
		adminDNs:           admins,
		serverDAO:          dao.NewServerDAO(),
		bookmarkDAO:        dao.NewBookmarkDAO(),
		logDAO:             dao.NewLogDAO(),
		pool:               pkgldap.NewPool(pkgldap.DefaultPoolConfig()),
	}
	go cleanupCredSessionsLoop()
	return s
}

// IsAdmin 判断登录 DN 是否在 security.admin_dns 白名单中（大小写不敏感、完整 DN 匹配）
func (s *LdapService) IsAdmin(userDN string) bool {
	normalized := normalizeDN(userDN)
	if normalized == "" || len(s.adminDNs) == 0 {
		return false
	}
	_, ok := s.adminDNs[normalized]
	return ok
}

// normalizeDN 规范化 DN 用于白名单比较（去空白、大小写不敏感）
func normalizeDN(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

// Pool 返回底层连接池，供监控接口查询状态
func (s *LdapService) Pool() *pkgldap.Pool {
	return s.pool
}

// Close 关闭连接池（服务优雅退出时调用）
func (s *LdapService) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// ConnParams 连接参数
type ConnParams struct {
	Server   *model.LdapServer
	BindDN   string
	Password string
}

// credEntry 会话缓存条目（绑定密码 + 服务器配置）
type credEntry struct {
	password  string
	server    *model.LdapServer
	expiresAt time.Time
}

// credTTL 会话缓存有效期，与 JWT 有效期保持一致
const credTTL = 24 * time.Hour

// maxCredCacheEntries 凭据缓存条目上限，防止大量登录耗尽内存
const maxCredCacheEntries = 10000

// credCache 登录会话内存缓存（key: JWT token）
var (
	credCache     sync.Map
	credCacheSize int64
)

// CacheSession 缓存登录成功后的服务器配置与绑定密码
//
// 未勾选“记住此服务器”的临时登录不会落库，服务器配置只保存在此缓存中，
// 供登录后的业务接口建立 LDAP 连接使用；server 会被复制，避免外部修改影响缓存。
func CacheSession(token string, server *model.LdapServer, password string) bool {
	if token == "" {
		return false
	}
	if _, exists := credCache.Load(token); !exists && atomic.LoadInt64(&credCacheSize) >= maxCredCacheEntries {
		// 达到上限先清理过期条目；仍满则拒绝新增，避免内存无限增长
		cleanupCredSessions(time.Now())
		if atomic.LoadInt64(&credCacheSize) >= maxCredCacheEntries {
			return false
		}
	}
	var serverCopy *model.LdapServer
	if server != nil {
		s := *server
		serverCopy = &s
	}
	credCache.Store(token, credEntry{password: password, server: serverCopy, expiresAt: time.Now().Add(credTTL)})
	atomic.AddInt64(&credCacheSize, 1)
	return true
}

// getCachedSession 按 token 获取会话缓存，过期条目惰性清除
func getCachedSession(token string) (credEntry, bool) {
	if token == "" {
		return credEntry{}, false
	}
	v, ok := credCache.Load(token)
	if !ok {
		return credEntry{}, false
	}
	e := v.(credEntry)
	if time.Now().After(e.expiresAt) {
		credCache.Delete(token)
		decrementCredCacheSize()
		return credEntry{}, false
	}
	return e, true
}

// getCachedPassword 按 token 获取缓存的密码
func getCachedPassword(token string) string {
	e, ok := getCachedSession(token)
	if !ok {
		return ""
	}
	return e.password
}

// ClearSession 清除指定 token 的会话缓存（用于 Logout）
func ClearSession(token string) {
	if token == "" {
		return
	}
	if _, loaded := credCache.LoadAndDelete(token); loaded {
		decrementCredCacheSize()
	}
}

// cleanupCredSessions 清理过期条目
func cleanupCredSessions(now time.Time) {
	credCache.Range(func(key, value any) bool {
		if e, ok := value.(credEntry); ok && now.After(e.expiresAt) {
			credCache.Delete(key)
			decrementCredCacheSize()
		}
		return true
	})
}

// decrementCredCacheSize 安全递减计数（避免测试直接写入缓存导致计数为负）
func decrementCredCacheSize() {
	if atomic.LoadInt64(&credCacheSize) > 0 {
		atomic.AddInt64(&credCacheSize, -1)
	}
}

// cleanupCredSessionsLoop 周期性清理过期凭据，避免依赖惰性清理导致内存长期占用
func cleanupCredSessionsLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		cleanupCredSessions(now)
	}
}

// GetConnParams 从请求上下文获取连接参数
//
// 服务器配置优先从数据库读取（server_id 命中已保存的服务器）；
// 未勾选“记住此服务器”的临时登录 server_id 为 0，或者服务器记录已被删除时，
// 回退到登录时写入内存缓存的服务器配置，保证会话在有效期内可用。
func (s *LdapService) GetConnParams(c *gin.Context) (*ConnParams, error) {
	token := middleware.GetToken(c)
	session, hasSession := getCachedSession(token)

	server, err := s.serverDAO.Get(middleware.GetServerID(c))
	if err != nil {
		return nil, err
	}
	if server == nil {
		if !hasSession || session.server == nil {
			return nil, errServerNotFound
		}
		server = session.server
	}

	password := ""
	if hasSession {
		password = session.password
	}
	bindDN := middleware.GetUserDN(c)
	if bindDN != "" && password == "" {
		return nil, ErrCredentialsExpired
	}
	return &ConnParams{
		Server:   server,
		BindDN:   bindDN,
		Password: password,
	}, nil
}

// ManagedConn 池化连接包装器
//
// 通过嵌入 *ldap.Conn 获得所有原生 LDAP 操作方法，
// 同时重写 Close() 把连接归还到池中而非关闭底层 TCP。
// sync.Once 保证 Close 幂等，多次调用只归还一次。
type ManagedConn struct {
	*ldap.Conn
	pool *pkgldap.Pool
	cp   *pkgldap.ChanPool
	once sync.Once
}

// Close 归还连接到池中（而非关闭底层 TCP）
// 若连接已损坏（IsClosing=true），Pool.Put 内部会关闭它。
func (m *ManagedConn) Close() error {
	m.once.Do(func() {
		m.pool.Put(m.cp, m.Conn)
	})
	return nil
}

// validateConnectionSecurity 校验 LDAP 连接安全策略
//
// 仅当 security.require_ldap_tls=true 时强制要求 TLS/SSL；默认 false，允许明文连接。
func (s *LdapService) validateConnectionSecurity(server *model.LdapServer) error {
	if s.requireLDAPTLS && !server.UseSSL && !server.UseTLS {
		return errors.New("安全策略禁止明文连接 LDAP（security.require_ldap_tls=true），请启用 TLS/SSL 或调整配置")
	}
	if server == nil || server.Host == "" {
		return errors.New("LDAP 服务器地址不能为空")
	}
	return nil
}

// VerifyBind 使用全新连接校验绑定凭据（不经过连接池）
//
// 登录与“测试连接”必须实际执行 Bind，不能复用连接池中同 DN 的连接，否则密码校验会被跳过。
// 同时统一执行 TLS 强制与匿名绑定策略，避免登录路径绕过安全策略。
func (s *LdapService) VerifyBind(server *model.LdapServer, bindDN, password string) (*ldap.Conn, error) {
	if err := s.validateConnectionSecurity(server); err != nil {
		return nil, err
	}
	if bindDN == "" && !s.allowAnonymousBind {
		return nil, errors.New("安全策略禁止匿名绑定（security.allow_anonymous_bind=false）")
	}
	return pkgldap.ConnectWithServer(server, bindDN, password, s.requireLDAPTLS, s.allowAnonymousBind)
}

// Connect 从连接池获取 LDAP 连接，返回 ManagedConn
//
// 返回的连接已完成 Bind 且带 SetTimeout，API 层代码可直接使用 defer conn.Close()
// 归还到池中，与原来每次新建连接的用法完全一致。
func (s *LdapService) Connect(params *ConnParams) (*ManagedConn, error) {
	if err := s.validateConnectionSecurity(params.Server); err != nil {
		return nil, err
	}
	if params.BindDN == "" && !s.allowAnonymousBind {
		return nil, errors.New("安全策略禁止匿名绑定（security.allow_anonymous_bind=false）")
	}

	info := pkgldap.ConnInfo{
		Host:               params.Server.Host,
		Port:               params.Server.Port,
		BindDN:             params.BindDN,
		Password:           params.Password,
		UseTLS:             params.Server.UseTLS,
		UseSSL:             params.Server.UseSSL,
		SkipTLSVerify:      params.Server.SkipTLSVerify,
		RequireTLS:         s.requireLDAPTLS,
		AllowAnonymousBind: s.allowAnonymousBind,
		Timeout:            params.Server.Timeout,
		Keepalive:          true,
	}
	conn, cp, err := s.pool.Get(info)
	if err != nil {
		return nil, err
	}
	return &ManagedConn{Conn: conn, pool: s.pool, cp: cp}, nil
}

// LogOperation 记录操作日志
func (s *LdapService) LogOperation(serverID int, userDN, action, targetDN, detail, status, errorMsg string, duration time.Duration) {
	log := &model.OperationLog{
		ServerID:   serverID,
		UserDN:     userDN,
		Action:     action,
		TargetDN:   targetDN,
		Detail:     detail,
		Status:     status,
		ErrorMsg:   errorMsg,
		DurationMS: duration.Milliseconds(),
	}
	_ = s.logDAO.Create(log)
}

// GetServerDAO 获取服务器 DAO
func (s *LdapService) GetServerDAO() *dao.ServerDAO {
	return s.serverDAO
}

// GetBookmarkDAO 获取书签 DAO
func (s *LdapService) GetBookmarkDAO() *dao.BookmarkDAO {
	return s.bookmarkDAO
}

// GetLogDAO 获取日志 DAO
func (s *LdapService) GetLogDAO() *dao.LogDAO {
	return s.logDAO
}
