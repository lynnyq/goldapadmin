package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// 已知弱密钥/占位符特征（release 模式禁止启动）
var insecureSecretMarkers = []string{
	"goldapadmin-secret",
	"change-me",
	"changeme",
	"change_me",
	"replace_with",
	"replace-with",
	"your-secret",
	"xxxxxxxx",
}

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	Security SecurityConfig `mapstructure:"security"`
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
	TTL    int    `mapstructure:"ttl"` // 小时
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// SecurityConfig 安全相关配置
type SecurityConfig struct {
	// RequireLDAPTLS 是否强制 LDAP 连接必须使用 TLS/SSL（默认 false，允许明文连接）
	// 生产环境建议开启，避免管理员凭据在网络上明文传输
	RequireLDAPTLS bool `mapstructure:"require_ldap_tls"`
	// AllowAnonymousBind 是否允许匿名（无 BindDN）登录；默认 false，必须显式开启
	AllowAnonymousBind bool `mapstructure:"allow_anonymous_bind"`
	// ReadOnly 只读模式：开启后所有写操作（新增/修改/删除/导入/移动/重命名）一律拒绝，
	// 对标 phpLDAPadmin 的 read_only 能力，适合“只查询不改动”的巡检场景
	ReadOnly bool `mapstructure:"read_only"`
	// AdminDNs 允许管理服务器配置（新增/修改/删除/测试连接）的登录 DN 白名单
	// 为空时不授予任何人服务器管理权限（fail-closed）；支持大小写不敏感的全量 DN 精确匹配
	AdminDNs []string `mapstructure:"admin_dns"`
	// TrustedProxies 可信反向代理 IP/CIDR；只有来自这些地址的请求才解析 X-Forwarded-For
	// 为空时只信任 RemoteAddr，避免客户端伪造 XFF 绕过限流
	TrustedProxies []string `mapstructure:"trusted_proxies"`
	// MaxBodyBytes 单个请求体上限（字节），默认 2MiB；用于限制 LDIF 导入等大报文
	MaxBodyBytes int64 `mapstructure:"max_body_bytes"`
	// CORS CORS 配置
	CORS CORSConfig `mapstructure:"cors"`
}

// CORSConfig CORS 跨域配置
type CORSConfig struct {
	// Origins 允许的 Origin 白名单；为空数组时默认只允许同源请求（不跨域）
	// 开发环境可填 ["http://localhost:5173"] 等
	Origins []string `mapstructure:"origins"`
}

// Load 加载配置
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// 默认值
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "release")
	v.SetDefault("database.path", "./data/goldapadmin.db")
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.ttl", 24)
	v.SetDefault("log.level", "info")
	v.SetDefault("security.require_ldap_tls", false)
	v.SetDefault("security.allow_anonymous_bind", false)
	v.SetDefault("security.read_only", false)
	v.SetDefault("security.admin_dns", []string{})
	v.SetDefault("security.trusted_proxies", []string{})
	v.SetDefault("security.max_body_bytes", int64(2<<20))
	v.SetDefault("security.cors.origins", []string{})

	if err := v.ReadInConfig(); err != nil {
		// 配置文件不存在时使用默认值
		if os.IsNotExist(err) {
			fmt.Printf("config file not found, using defaults\n")
		} else {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate 校验安全相关配置，release 模式下拒绝弱密钥/非法值
//
// 参数：无 | 返回：配置错误 | 错误说明：release 模式下 JWT secret 必须 >= 32 位且不是已知弱值；
// trusted_proxies 必须是合法 IP/CIDR；max_body_bytes 非正时回退默认值。
func (c *Config) Validate() error {
	c.Server.Mode = strings.ToLower(strings.TrimSpace(c.Server.Mode))
	if c.Server.Mode != "debug" && c.Server.Mode != "release" && c.Server.Mode != "test" {
		return fmt.Errorf("config: server.mode 只能是 debug/release/test，当前为 %q", c.Server.Mode)
	}

	c.JWT.Secret = strings.TrimSpace(c.JWT.Secret)
	if c.JWT.Secret == "" {
		if c.Server.Mode == "release" {
			return fmt.Errorf("config: release 模式必须配置 jwt.secret（建议 openssl rand -base64 48）")
		}
		// 非 release 模式允许开发用固定弱密钥，便于本地调试
		c.JWT.Secret = "dev-only-insecure-secret-0123456789abcdef"
	}

	if c.Server.Mode == "release" {
		if err := validateReleaseSecret(c.JWT.Secret); err != nil {
			return err
		}
	}

	if c.JWT.TTL <= 0 {
		c.JWT.TTL = 24
	}
	if c.JWT.TTL > 168 {
		return fmt.Errorf("config: jwt.ttl 不能超过 168 小时")
	}

	if c.Security.MaxBodyBytes <= 0 {
		c.Security.MaxBodyBytes = 2 << 20
	}
	if c.Security.MaxBodyBytes > 32<<20 {
		return fmt.Errorf("config: security.max_body_bytes 不能超过 32MiB")
	}

	for _, p := range c.Security.TrustedProxies {
		if _, _, err := net.ParseCIDR(p); err != nil {
			if net.ParseIP(p) == nil {
				return fmt.Errorf("config: security.trusted_proxies 中的 %q 不是合法 IP/CIDR", p)
			}
		}
	}

	if c.Server.Mode == "release" && !c.Security.RequireLDAPTLS {
		fmt.Printf("[WARN] security.require_ldap_tls=false：管理员凭据可能以明文传输到 LDAP 服务器，生产环境请设为 true\n")
	}
	if c.Server.Mode == "release" && len(c.Security.AdminDNs) == 0 {
		fmt.Printf("[WARN] security.admin_dns 为空：服务器配置管理接口将对所有用户关闭（fail-closed）\n")
	}
	for _, origin := range c.Security.CORS.Origins {
		if origin == "*" && c.Server.Mode == "release" {
			return fmt.Errorf("config: release 模式禁止 security.cors.origins 使用通配符 \"*\"")
		}
	}

	return nil
}

// validateReleaseSecret 拒绝 release 模式下的弱密钥与占位符
func validateReleaseSecret(secret string) error {
	if len(secret) < 32 {
		return fmt.Errorf("config: jwt.secret 长度必须 >= 32 字节（当前 %d），建议 openssl rand -base64 48", len(secret))
	}
	lower := strings.ToLower(secret)
	for _, marker := range insecureSecretMarkers {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("config: jwt.secret 包含已知弱值/占位符 %q，请替换为随机密钥", marker)
		}
	}
	return nil
}
