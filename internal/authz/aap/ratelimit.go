// Package aap AAP 限流器（IP + owner_did 双维度）。
//
// 算法：固定窗口计数器（cache.Incr）
//   - 每个维度组合一个 key：rl:aap:<ip>:<owner_did>:<window>
//   - Incr 自增 + TTL = window
//   - count > limit → 拒绝
//
// 固定窗口的优缺点：
//   - 优：实现简单 / 单次操作 / 强一致（Incr 原子）
//   - 缺：窗口边界处可能突刺（最多 2x 流量）
//   - v2.0.1 取简单方案；后续可升级为滑动窗口（Redis Sorted Set / Sliding Log）
//
// 维度组合：
//   - 同一 IP 不同 owner_did：分别限流（多租户友好）
//   - 同一 owner_did 不同 IP：分别限流（防单 owner 多 IP 滥用）
//   - 维度全无（兜底）：仅 IP 维度
//
// 失败模式（Fail Open）：
//   - Redis 不可用 → 放行（不阻断业务）
//   - 上游若需要 Fail Close，可在配置层加开关
package aap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrRateLimited 触发限流。
var ErrRateLimited = errors.New("aap: rate limit exceeded")

// =============================================================================
// Limiter
// =============================================================================

// Limiter 限流器配置。
type LimiterConfig struct {
	// Cache 缓存后端
	Cache cache.Cache
	// Limit 窗口内最大请求数（默认 10）
	Limit int64
	// Window 窗口大小（默认 1 分钟）
	Window time.Duration
	// KeyPrefix 自定义 key 前缀（默认 "rl:aap"）
	KeyPrefix string
	// Clock 时间源（测试用）
	Clock func() time.Time
	// FailOpen Redis 错误时是否放行（默认 true — 业务优先）
	FailOpen bool
}

// Limiter AAP 限流器。
type Limiter struct {
	cfg LimiterConfig
}

// NewLimiter 构造限流器。
func NewLimiter(cfg LimiterConfig) (*Limiter, error) {
	if cfg.Cache == nil {
		return nil, errors.New("aap: nil cache")
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 10
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "rl:aap"
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Limiter{cfg: cfg}, nil
}

// Allow 检查当前请求是否被允许。
//
// 步骤：
//  1. 拼 key
//  2. Incr + TTL
//  3. count > limit → ErrRateLimited
//  4. 否则放行
//
// 返回 (allowed, remaining, err)。
//   - allowed: 是否允许
//   - remaining: 剩余配额
//   - err: 错误（仅 Redis 错误时；FailOpen 时返回 nil 但 allowed=true）
func (l *Limiter) Allow(ctx context.Context, ip, ownerDID string) (allowed bool, remaining int64, err error) {
	key := l.key(ip, ownerDID)
	count, err := l.cfg.Cache.Incr(ctx, key, l.cfg.Window)
	if err != nil {
		if l.cfg.FailOpen {
			return true, l.cfg.Limit, nil
		}
		return false, 0, fmt.Errorf("aap: rate limit incr: %w", err)
	}
	if count > l.cfg.Limit {
		return false, 0, ErrRateLimited
	}
	return true, l.cfg.Limit - count, nil
}

// AllowWith 自定义 key 后缀（高级用法）。
func (l *Limiter) AllowWith(ctx context.Context, suffix string) (bool, int64, error) {
	key := l.cfg.KeyPrefix + ":" + suffix
	count, err := l.cfg.Cache.Incr(ctx, key, l.cfg.Window)
	if err != nil {
		if l.cfg.FailOpen {
			return true, l.cfg.Limit, nil
		}
		return false, 0, fmt.Errorf("aap: rate limit incr: %w", err)
	}
	if count > l.cfg.Limit {
		return false, 0, ErrRateLimited
	}
	return true, l.cfg.Limit - count, nil
}

// Reset 重置某个 key 的计数（管理用）。
func (l *Limiter) Reset(ctx context.Context, ip, ownerDID string) error {
	return l.cfg.Cache.Del(ctx, l.key(ip, ownerDID))
}

// Limit 返回配置的上限。
func (l *Limiter) Limit() int64 { return l.cfg.Limit }

// Window 返回配置的窗口。
func (l *Limiter) Window() time.Duration { return l.cfg.Window }

// =============================================================================
// HTTP 中间件
// =============================================================================

// WrapHTTP 包装 net/http.Handler，触发限流返回 429。
//
// IP 提取顺序：X-Forwarded-For > X-Real-IP > RemoteAddr
// owner_did：X-AAP-User-DID（业务在调用前注入）
func (l *Limiter) WrapHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		ownerDID := r.Header.Get("X-AAP-User-DID")
		allowed, remaining, err := l.Allow(r.Context(), ip, ownerDID)
		if err != nil && !errors.Is(err, ErrRateLimited) {
			// Redis 错误已由 FailOpen 吸收；这里不应该 reach
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// 注入配额 header（无论 allow/deny）
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(l.cfg.Limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(l.cfg.Window.Seconds())))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP 提取 client IP。
func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// 取第一个
		for i := 0; i < len(v); i++ {
			if v[i] == ',' {
				return v[:i]
			}
		}
		return v
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	// 退到 RemoteAddr
	addr := r.RemoteAddr
	for i := 0; i < len(addr); i++ {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// =============================================================================
// key 拼装
// =============================================================================

// key 拼装限流 key。
//
// 格式：<prefix>:<ip>:<owner_did>:<window-bucket>
//
// window-bucket：当前时间向下取整到 Window 边界
//   - 目的：相邻两窗口不互相影响（TTL 失效前不会跨窗）
//   - 例：Window=60s, now=12:34:45 → bucket=12:34:00
func (l *Limiter) key(ip, ownerDID string) string {
	now := l.cfg.Clock()
	bucket := now.Unix() - (now.Unix() % int64(l.cfg.Window.Seconds()))
	return fmt.Sprintf("%s:%s:%s:%d", l.cfg.KeyPrefix, ip, ownerDID, bucket)
}
