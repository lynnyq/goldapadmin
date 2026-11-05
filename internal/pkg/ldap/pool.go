package ldap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// poolKey 连接池分池 key
//
// 必须同时包含 (host, port, bindDN, TLS 模式, 证书校验策略, 密码摘要)：
//   - 缺少 TLS 标记会导致要求 TLS 的会话复用早先的明文连接（安全策略被绕过）
//   - 缺少密码摘要会导致密码轮换后仍复用旧凭据建立的连接
//
// 密码只保存 SHA-256 摘要，避免在内存 map 中留下明文。
type poolKey struct {
	host          string
	port          int
	bindDN        string
	useTLS        bool
	useSSL        bool
	skipTLSVerify bool
	passwordHash  string
}

// passwordDigest 计算密码摘要（不可逆，仅用于区分不同凭据）
func passwordDigest(password string) string {
	if password == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// poolStats 池子运行时指标
type poolStats struct {
	GetCount      int64 // 累计 Get() 次数
	PutCount      int64 // 累计 Put() 次数
	MissCount     int64 // 池子空时新建连接次数
	HitsCount     int64 // 命中池中闲置连接次数
	CloseCount    int64 // 主动关闭连接次数
	IdleCount     int64 // 当前闲置连接数
	ActiveCount   int64 // 当前活跃连接数（已借出）
	MaxActiveSeen int64 // 历史峰值活跃连接数
}

// Pool LDAP 连接池
//
// 设计要点：
//  1. 按 (host, port, bindDN) 三元组分池，避免跨服务器/跨账号混用
//  2. 使用 buffered channel 做 idle 队列，MaxIdle=空闲上限，MaxActive=并发上限
//  3. Get() 优先从 idle 取，拿不到则新建；若活跃连接已达 MaxActive 则阻塞等待
//  4. Put() 归还前检查 IsClosing() + TCP keepalive 健康探测
//  5. 空闲连接超过 IdleTimeout 自动关闭（后台 goroutine 定期清理）
//  6. 连接建立后立即 SetTimeout（读写超时），避免 LDAP 操作无限挂起
type Pool struct {
	mu sync.RWMutex

	// pools 按 key 分池
	pools map[poolKey]*ChanPool

	// 全局配置（创建时注入）
	cfg PoolConfig

	// 已建立连接总数（峰值用于监控）
	totalCreated int64
	totalClosed  int64
}

// ChanPool 单个 LDAP 服务池（按三元组分池）
type ChanPool struct {
	key poolKey

	// idle 闲置连接队列
	idle chan *ldap.Conn

	// active 当前已借出数
	active int64

	// lastGet 最近一次 Get 时间（用于空闲回收判断）
	lastAccess atomic.Int64

	// cfg 该池子的配置副本
	cfg PoolConfig

	// 统计
	stats   poolStats
	statsMu sync.Mutex
}

// PoolConfig 连接池配置
type PoolConfig struct {
	// MaxIdle 每个 LDAP 服务最多保留的闲置连接数
	// 建议 5-10，OpenLDAP/AD 一般能扛住
	MaxIdle int

	// MaxActive 每个 LDAP 服务最大活跃连接数（硬上限，超出后 Get() 阻塞）
	// 建议 20-50，超过 OpenLDAP 默认 maxconn(500) 会被服务器拒绝
	MaxActive int

	// IdleTimeout 闲置超过此时间的连接会被后台清理协程关闭
	IdleTimeout time.Duration

	// OpTimeout 连接上的读写超时（SetTimeout 统一应用到后续所有 LDAP 操作）
	// 建议 10-30s，防止网络闪断导致 goroutine 挂死
	OpTimeout time.Duration

	// KeepaliveInterval TCP keepalive 探测间隔（Dialer 级别）
	KeepaliveInterval time.Duration

	// DialTimeout 建连超时（net.Dialer.Timeout）
	DialTimeout time.Duration

	// BindTimeout 绑定超时（Bind/UnauthenticatedBind 执行超时）
	BindTimeout time.Duration
}

// DefaultPoolConfig 默认连接池配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxIdle:           10,
		MaxActive:         30,
		IdleTimeout:       5 * time.Minute,
		OpTimeout:         15 * time.Second,
		KeepaliveInterval: 30 * time.Second,
		DialTimeout:       10 * time.Second,
		BindTimeout:       10 * time.Second,
	}
}

// NewPool 创建 LDAP 连接池
func NewPool(cfg PoolConfig) *Pool {
	if cfg.MaxIdle <= 0 {
		cfg.MaxIdle = DefaultPoolConfig().MaxIdle
	}
	if cfg.MaxActive <= 0 {
		cfg.MaxActive = DefaultPoolConfig().MaxActive
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = DefaultPoolConfig().IdleTimeout
	}
	if cfg.OpTimeout <= 0 {
		cfg.OpTimeout = DefaultPoolConfig().OpTimeout
	}
	if cfg.KeepaliveInterval <= 0 {
		cfg.KeepaliveInterval = DefaultPoolConfig().KeepaliveInterval
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = DefaultPoolConfig().DialTimeout
	}
	if cfg.BindTimeout <= 0 {
		cfg.BindTimeout = DefaultPoolConfig().BindTimeout
	}

	p := &Pool{
		pools: make(map[poolKey]*ChanPool),
		cfg:   cfg,
	}
	go p.idleCleanup()
	return p
}

// getPool 获取或创建某个 key 对应的池
func (p *Pool) getPool(key poolKey) *ChanPool {
	p.mu.RLock()
	cp, ok := p.pools[key]
	p.mu.RUnlock()
	if ok {
		return cp
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if cp, ok = p.pools[key]; ok {
		return cp
	}
	cp = &ChanPool{
		key:  key,
		idle: make(chan *ldap.Conn, p.cfg.MaxIdle),
		cfg:  p.cfg,
	}
	p.pools[key] = cp
	return cp
}

// Get 从池中获取一个 LDAP 连接，可能复用闲置连接或新建
//
// 返回的连接已完成 Bind（凭据正确）且已设置读写超时，调用方可直接使用。
// 使用完毕后必须调用 Put() 归还。若连接已损坏（IsClosing=true），Put() 会关闭它。
func (p *Pool) Get(info ConnInfo) (*ldap.Conn, *ChanPool, error) {
	key := poolKey{
		host:          info.Host,
		port:          info.Port,
		bindDN:        info.BindDN,
		useTLS:        info.UseTLS,
		useSSL:        info.UseSSL,
		skipTLSVerify: info.SkipTLSVerify,
		passwordHash:  passwordDigest(info.Password),
	}
	cp := p.getPool(key)

	atomic.AddInt64(&cp.stats.GetCount, 1)
	cp.lastAccess.Store(time.Now().UnixNano())

	// 1. 尝试从 idle 队列取
	select {
	case conn := <-cp.idle:
		// 从 idle 拿到了，先做健康检查
		if conn != nil && !conn.IsClosing() {
			// 健康检查：用 WhoAmI 验证连接仍可用，避免把服务器已关闭的连接交给上层
			if _, err := conn.WhoAmI(nil); err == nil {
				if atomic.LoadInt64(&cp.active) >= int64(cp.cfg.MaxActive) {
					// 理论上 idle 连接不会同时触发 active 上限，防御性判断
					conn.Close()
					return p.createConn(info, cp)
				}
				atomic.AddInt64(&cp.active, 1)
				atomic.AddInt64(&cp.stats.HitsCount, 1)
				cp.recordPeak()
				return conn, cp, nil
			}
			// 连接已失效，关闭后走新建
			conn.Close()
			atomic.AddInt64(&cp.stats.CloseCount, 1)
		}
		// fallthrough 到新建
	default:
		// idle 队列为空，走新建
	}

	// 2. 检查 active 上限
	if atomic.LoadInt64(&cp.active) >= int64(cp.cfg.MaxActive) {
		// 阻塞等待，或者可以返回 error。这里选择阻塞等待直到有连接归还
		select {
		case conn := <-cp.idle:
			if conn != nil && !conn.IsClosing() {
				if _, err := conn.WhoAmI(nil); err == nil {
					atomic.AddInt64(&cp.active, 1)
					atomic.AddInt64(&cp.stats.HitsCount, 1)
					cp.recordPeak()
					return conn, cp, nil
				}
				conn.Close()
				atomic.AddInt64(&cp.stats.CloseCount, 1)
			}
			// 归还的也是坏的，继续尝试新建
		case <-time.After(p.cfg.BindTimeout + 5*time.Second):
			return nil, cp, fmt.Errorf("LDAP connection pool exhausted for %s:%d (max %d active), waited %s",
				info.Host, info.Port, p.cfg.MaxActive, p.cfg.BindTimeout+5*time.Second)
		}
	}

	return p.createConn(info, cp)
}

// createConn 新建一条 LDAP 连接并执行 Bind
func (p *Pool) createConn(info ConnInfo, cp *ChanPool) (*ldap.Conn, *ChanPool, error) {
	atomic.AddInt64(&p.totalCreated, 1)
	atomic.AddInt64(&cp.stats.MissCount, 1)

	// 覆盖连接池级别的超时设置
	poolInfo := info
	if info.Timeout == 0 || info.Timeout > int(p.cfg.DialTimeout.Seconds()) {
		poolInfo.Timeout = int(p.cfg.DialTimeout.Seconds())
	}

	conn, err := Connect(poolInfo)
	if err != nil {
		return nil, cp, err
	}

	// 统一设置 LDAP 读写超时，避免 Search/Bind 等操作无限挂起
	if cp.cfg.OpTimeout > 0 {
		conn.SetTimeout(cp.cfg.OpTimeout)
	}

	atomic.AddInt64(&cp.active, 1)
	cp.recordPeak()

	return conn, cp, nil
}

// Put 归还连接到池中。若连接已损坏或 IsClosing=true 则直接关闭。
// 若归还后池中活跃数将超过 MaxIdle，则关闭（背压回收）。
func (p *Pool) Put(cp *ChanPool, conn *ldap.Conn) {
	if cp == nil || conn == nil {
		return
	}

	atomic.AddInt64(&cp.active, -1)
	atomic.AddInt64(&cp.stats.PutCount, 1)
	cp.lastAccess.Store(time.Now().UnixNano())

	// 连接已损坏，直接关闭
	if conn.IsClosing() {
		conn.Close()
		atomic.AddInt64(&cp.stats.CloseCount, 1)
		return
	}

	// 尝试放回 idle 队列；池满则关闭
	select {
	case cp.idle <- conn:
		// 归还成功，等下次 Get 时做 WhoAmI 健康检查
	default:
		// 池满，直接关闭
		conn.Close()
		atomic.AddInt64(&cp.stats.CloseCount, 1)
	}
}

// Close 关闭池内所有连接（服务退出时调用）
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, cp := range p.pools {
		close(cp.idle) // 先关闭 channel 禁止新的归还
		for conn := range cp.idle {
			if conn != nil {
				conn.Close()
				atomic.AddInt64(&cp.stats.CloseCount, 1)
			}
		}
		delete(p.pools, key)
	}
}

// Stats 返回池的整体统计快照
func (p *Pool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	perPool := make(map[string]map[string]interface{}, len(p.pools))
	for key, cp := range p.pools {
		snapshot := cp.snapshot()
		perPool[fmt.Sprintf("%s:%d/%s", key.host, key.port, key.bindDN)] = snapshot
	}

	return map[string]interface{}{
		"total_pools":   len(p.pools),
		"total_created": atomic.LoadInt64(&p.totalCreated),
		"total_closed":  atomic.LoadInt64(&p.totalClosed),
		"pools":         perPool,
	}
}

// snapshot 安全读取池状态
func (cp *ChanPool) snapshot() map[string]interface{} {
	cp.statsMu.Lock()
	defer cp.statsMu.Unlock()
	return map[string]interface{}{
		"get_count":       cp.stats.GetCount,
		"put_count":       cp.stats.PutCount,
		"miss_count":      cp.stats.MissCount,
		"hits_count":      cp.stats.HitsCount,
		"close_count":     cp.stats.CloseCount,
		"idle_count":      int64(len(cp.idle)),
		"active_count":    atomic.LoadInt64(&cp.active),
		"max_active_seen": cp.stats.MaxActiveSeen,
	}
}

func (cp *ChanPool) recordPeak() {
	current := atomic.LoadInt64(&cp.active)
	cp.statsMu.Lock()
	if current > cp.stats.MaxActiveSeen {
		cp.stats.MaxActiveSeen = current
	}
	cp.statsMu.Unlock()
}

// idleCleanup 后台协程，定期关闭超过 IdleTimeout 的闲置连接
func (p *Pool) idleCleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.RLock()
		pools := make([]*ChanPool, 0, len(p.pools))
		for _, cp := range p.pools {
			pools = append(pools, cp)
		}
		p.mu.RUnlock()

		for _, cp := range pools {
			p.cleanupPool(cp)
		}
	}
}

// cleanupPool 清理某个池中过期的闲置连接
func (p *Pool) cleanupPool(cp *ChanPool) {
	cutoff := time.Now().Add(-cp.cfg.IdleTimeout)

	// 把 idle 队列里的全部扫出来，逐个检查 lastAccess；过期的关闭，其余放回
	var kept []*ldap.Conn
drainLoop:
	for {
		select {
		case conn := <-cp.idle:
			// 检查 lastAccess：因为我们不知道单条连接的具体空闲时长，
			// 这里保守地用 channel drain 时机近似——若 drain 时 active=0 且超过 cutoff 就全清
			// 简化做法：检查池子整体 active==0 且 lastAccess 过期
			lastAccess := cp.lastAccess.Load()
			noAccess := time.Unix(0, lastAccess).Before(cutoff)
			if atomic.LoadInt64(&cp.active) == 0 && noAccess {
				if conn != nil {
					conn.Close()
					atomic.AddInt64(&cp.stats.CloseCount, 1)
				}
			} else {
				kept = append(kept, conn)
			}
		default:
			break drainLoop
		}
	}

	// 放回未过期的连接
	for _, conn := range kept {
		select {
		case cp.idle <- conn:
		default:
			conn.Close()
			atomic.AddInt64(&cp.stats.CloseCount, 1)
		}
	}
}

// ---- Dialer 辅助：带 TCP keepalive 的 Dialer ----

// NewDialerWithKeepalive 创建带 TCP keepalive 和超时的 net.Dialer
// 保证池化连接不会因为防火墙/NAT 空闲超时被静默断开
func NewDialerWithKeepalive(dialTimeout, keepaliveInterval time.Duration) *net.Dialer {
	return &net.Dialer{
		Timeout: dialTimeout,
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Interval: keepaliveInterval,
		},
	}
}
