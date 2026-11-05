package ldap

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/lynnyq/goldapadmin/internal/model"
)

// ConnInfo 连接信息
type ConnInfo struct {
	Host               string
	Port               int
	BindDN             string
	Password           string
	UseTLS             bool
	UseSSL             bool
	SkipTLSVerify      bool
	RequireTLS         bool          // 安全策略：强制使用 TLS/SSL（明文连接直接拒绝）
	AllowAnonymousBind bool          // 安全策略：是否允许匿名绑定（默认拒绝，需显式开启）
	Timeout            int           // Dial 超时（秒）
	OpTimeout          time.Duration // 可选：后续 LDAP 操作读写超时（SetTimeout）
	Keepalive          bool          // 是否开启 TCP keepalive
	KeepaliveInterval  time.Duration
}

// Connect 建立 LDAP 连接并绑定
//
// Connect 会按 ConnInfo 执行以下步骤：
//  1. 创建 net.Dialer（带 Keepalive + Dial 超时）
//  2. DialURL 建立 TCP 连接
//  3. 可选 StartTLS
//  4. Bind/UnauthenticatedBind 认证
//  5. 若 OpTimeout>0 则 SetTimeout 到连接上，保护后续 Search/Bind 等操作
//
// 失败路径一律 conn.Close()，确保无资源泄漏。
func Connect(info ConnInfo) (*ldap.Conn, error) {
	if info.RequireTLS && !info.UseSSL && !info.UseTLS {
		return nil, fmt.Errorf("安全策略禁止明文连接 LDAP（security.require_ldap_tls=true），请启用 TLS/SSL 或调整配置")
	}
	if info.BindDN == "" && !info.AllowAnonymousBind {
		return nil, fmt.Errorf("安全策略禁止匿名绑定（security.allow_anonymous_bind=false），请提供绑定 DN 与密码")
	}

	dialTimeout := time.Duration(info.Timeout) * time.Second
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}

	// 构建带 keepalive 的 Dialer（如果启用）
	dialer := &net.Dialer{Timeout: dialTimeout}
	if info.Keepalive {
		interval := info.KeepaliveInterval
		if interval <= 0 {
			interval = 30 * time.Second
		}
		dialer.KeepAliveConfig = net.KeepAliveConfig{
			Enable:   true,
			Interval: interval,
		}
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: info.SkipTLSVerify,
		MinVersion:         tls.VersionTLS12,
	}

	var conn *ldap.Conn
	var err error

	if info.UseSSL {
		conn, err = ldap.DialURL(
			fmt.Sprintf("ldaps://%s:%d", info.Host, info.Port),
			ldap.DialWithDialer(dialer),
			ldap.DialWithTLSConfig(tlsConfig),
		)
	} else {
		conn, err = ldap.DialURL(
			fmt.Sprintf("ldap://%s:%d", info.Host, info.Port),
			ldap.DialWithDialer(dialer),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("dial ldap: %w", err)
	}

	// Dial 成功后、Bind 失败时也要关闭（已存在于后续 bind 失败分支，这里防御性再包一层）
	bindRequired := true
	defer func() {
		if bindRequired && err != nil {
			conn.Close()
		}
	}()

	if info.UseTLS && !info.UseSSL {
		if err := conn.StartTLS(tlsConfig); err != nil {
			bindRequired = false
			err = fmt.Errorf("starttls: %w", err)
			return nil, err
		}
	}

	// 统一设置 LDAP 操作读写超时（SetTimeout 会把同一个超时应用到所有后续请求/响应）
	if info.OpTimeout > 0 {
		conn.SetTimeout(info.OpTimeout)
	}

	// 根据 BindDN / Password 组合选择绑定模式：
	//   1. BindDN 为空    → 匿名绑定（UnauthenticatedBind）
	//   2. BindDN 非空且密码非空 → SIMPLE 绑定
	//   3. BindDN 非空但密码为空 → 明确报错，拒绝执行
	switch {
	case info.BindDN == "":
		if err = conn.UnauthenticatedBind(""); err != nil {
			err = fmt.Errorf("anonymous bind: %w", err)
			return nil, err
		}
	case info.Password == "":
		err = fmt.Errorf("bind: 已填写绑定 DN [%s]，但密码为空。SIMPLE 绑定需要密码；如需匿名连接请清空绑定 DN 字段", info.BindDN)
		return nil, err
	default:
		if err = conn.Bind(info.BindDN, info.Password); err != nil {
			err = fmt.Errorf("bind: %w", err)
			return nil, err
		}
	}

	bindRequired = false
	return conn, nil
}

// ConnectWithServer 使用服务器配置 + 密码连接
//
// requireTLS/allowAnonymous 由服务层按全局安全策略传入，保证登录、测试连接与业务
// 请求使用同一套安全校验（避免只校验业务请求而登录走明文）。
func ConnectWithServer(server *model.LdapServer, bindDN, password string, requireTLS, allowAnonymous bool) (*ldap.Conn, error) {
	info := ConnInfo{
		Host:               server.Host,
		Port:               server.Port,
		BindDN:             bindDN,
		Password:           password,
		UseTLS:             server.UseTLS,
		UseSSL:             server.UseSSL,
		SkipTLSVerify:      server.SkipTLSVerify,
		RequireTLS:         requireTLS,
		AllowAnonymousBind: allowAnonymous,
		Timeout:            server.Timeout,
		Keepalive:          true,
	}
	return Connect(info)
}
