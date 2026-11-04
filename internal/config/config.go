package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

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
	// SkipTLSVerify 全局默认：当 LdapServer 未显式指定时使用此值（生产建议 false）
	SkipTLSVerify bool `mapstructure:"skip_tls_verify"`
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
	v.SetDefault("jwt.secret", "goldapadmin-secret-key")
	v.SetDefault("jwt.ttl", 24)
	v.SetDefault("log.level", "info")
	v.SetDefault("security.skip_tls_verify", false)
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

	return &cfg, nil
}
