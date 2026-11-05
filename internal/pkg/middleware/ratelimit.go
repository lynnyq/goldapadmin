package middleware

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/goldapadmin/internal/pkg/response"
)

// rateLimiter 滑动窗口限流器
//
// 每个 IP 维护一个请求计数的滑动窗口，按 interval 时间窗滑动，
// 窗口内请求数超过 limit 则返回 429 Too Many Requests。
// 相比令牌桶实现更简单、对 burst 更严格（窗口内绝对不超过 limit），
// 非常适合防御密码喷洒攻击和 LDAP 连接洪泛。
type rateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*slidingWindow
	limit    int           // 窗口内允许的最大请求数
	interval time.Duration // 滑动窗口大小
}

type slidingWindow struct {
	count     int
	windowEnd time.Time // 当前窗口结束时间
}

func newRateLimiter(limit int, interval time.Duration) *rateLimiter {
	rl := &rateLimiter{
		windows:  make(map[string]*slidingWindow),
		limit:    limit,
		interval: interval,
	}
	go rl.cleanup()
	return rl
}

// Allow 判断 ip 在当前窗口是否允许再发起一次请求
// 返回 (是否允许, 剩余秒数) — 不允许时剩余秒数是需要等待的时长
func (rl *rateLimiter) Allow(ip string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	sw, ok := rl.windows[ip]
	if !ok || now.After(sw.windowEnd) {
		// 新窗口：重置计数
		rl.windows[ip] = &slidingWindow{
			count:     1,
			windowEnd: now.Add(rl.interval),
		}
		return true, 0
	}

	sw.count++
	if sw.count > rl.limit {
		return false, sw.windowEnd.Sub(now)
	}
	return true, 0
}

// cleanup 后台清理过期窗口（防止 map 无限增长）
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now()
		for ip, sw := range rl.windows {
			if cutoff.After(sw.windowEnd) {
				delete(rl.windows, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// globalLimiter 全局默认限流器：每个 IP 每分钟最多 120 次请求
// 这个量级足够正常开发/运维使用，但能有效阻止暴力扫描
var globalLimiter = newRateLimiter(120, time.Minute)

// strictLimiter 严格限流器：每个 IP 每分钟最多 20 次
// 专用于 Login / TestConnection 等可能被暴力破解的端点
var strictLimiter = newRateLimiter(20, time.Minute)

// RateLimit 全局限流中间件（默认宽松模式）
func RateLimit() gin.HandlerFunc {
	return rateLimitWith(globalLimiter)
}

// RateLimitStrict 严格限流中间件，专用于 Login / TestConnection
func RateLimitStrict() gin.HandlerFunc {
	return rateLimitWith(strictLimiter)
}

func rateLimitWith(rl *rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := clientIP(c.Request)
		allowed, wait := rl.Allow(ip)
		if !allowed {
			waitSeconds := int(wait.Seconds())
			if waitSeconds < 1 {
				waitSeconds = 1
			}
			c.Header("Retry-After", itoa(waitSeconds))
			c.JSON(http.StatusTooManyRequests, response.Response{
				Code:    http.StatusTooManyRequests,
				Message: "请求过于频繁，请 " + itoa(waitSeconds) + " 秒后重试",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// clientIP 从请求中提取真实客户端 IP
//
// 安全策略：默认只信任 TCP 连接对端地址（RemoteAddr），不解析任何客户端可控的
// X-Forwarded-For / X-Real-IP，避免伪造 XFF 绕过限流。仅当 RemoteAddr 命中
// SetTrustedProxies 配置的代理 IP/CIDR 时，才从右向左依次剥离可信代理并取第一个
// 不可信地址作为客户端 IP。
func clientIP(req *http.Request) string {
	remote := remoteIP(req)
	if !isTrustedProxy(remote) {
		return remote.String()
	}

	// 代理可信时，X-Forwarded-For 采用「从右到左」解析，防止客户端伪造最左值
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			addr, ok := parseIP(parts[i])
			if !ok {
				// 出现非法值说明链路异常，退化到当前可信地址
				break
			}
			if isTrustedProxy(addr) {
				remote = addr
				continue
			}
			return addr.String()
		}
		return remote.String()
	}

	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		if addr, ok := parseIP(xri); ok {
			return addr.String()
		}
	}
	return remote.String()
}

// remoteIP 解析 TCP 连接对端 IP（去掉端口）
func remoteIP(req *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	if addr, ok := parseIP(host); ok {
		return addr
	}
	return netip.Addr{}
}

// parseIP 解析并规范化 IP 字符串
func parseIP(s string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil || !addr.IsValid() {
		return netip.Addr{}, false
	}
	return addr, true
}

// trustedPrefixes 可信反向代理列表（IP 或 CIDR）
var (
	trustedPrefixesMu sync.RWMutex
	trustedPrefixes   []netip.Prefix
)

// SetTrustedProxies 设置可信反向代理 IP/CIDR 列表；非法值返回错误，启动阶段 fail-fast
func SetTrustedProxies(entries []string) error {
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if p, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, p)
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil || !addr.IsValid() {
			return fmt.Errorf("invalid trusted proxy %q: must be IP or CIDR", entry)
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}

	trustedPrefixesMu.Lock()
	trustedPrefixes = prefixes
	trustedPrefixesMu.Unlock()
	return nil
}

// isTrustedProxy 判断给定地址是否在可信代理列表中
func isTrustedProxy(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	trustedPrefixesMu.RLock()
	defer trustedPrefixesMu.RUnlock()
	for _, p := range trustedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// itoa 简易 int→string（避免 import strconv）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
