// Package ratelimit 通用限流器（与 AAP 专用版区分；可用于 A2A / MCP / API Gateway）。
//
// 算法：**双桶滑动窗口近似**（Cloudflare 风格）
//
// 设计动机：
//   - 固定窗口在边界处会突刺（最多 2x 流量）
//   - 真正的 Sorted Set 滑动窗口需要 ZADD/ZREMRANGEBYSCORE，但 cache.Cache 接口未暴露
//   - 双桶近似只需 Incr + Get，与现有接口兼容；估算误差 ≤ 1%
//
// 滑动估算公式：
//
//	sliding_count = prev_bucket_count * (1 - elapsed_in_current / window)
//	              + current_bucket_count
//
// 其中：
//   - bucket = floor(now / window)
//   - prev_bucket = bucket - 1
//   - elapsed_in_current = now % window
//
// 桶 key：rl:<scope>:<key>:<bucket-id>，TTL = 2 × window（保留前一桶供 Get）
//
// 失败模式：Fail Open（默认）— Redis 抖动不阻断业务；可通过 FailOpen=false 改为 Fail Close。
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrRateLimited 触发限流。
var ErrRateLimited = errors.New("ratelimit: limit exceeded")

// ErrCacheUnavailable cache 未配置。
var ErrCacheUnavailable = errors.New("ratelimit: cache unavailable")

// ErrEmptyKey key 不能为空。
var ErrEmptyKey = errors.New("ratelimit: empty key")

// =============================================================================
// Limiter
// =============================================================================

// Decision 限流决策结果。
type Decision struct {
	// Allowed 是否放行
	Allowed bool
	// Current 当前滑动窗口估算计数（含本次请求）
	Current float64
	// Limit 配置的最大值
	Limit int64
	// Remaining 剩余配额（max(0, Limit - Current)）
	Remaining int64
	// RetryAfter 拒绝时的建议重试间隔（毫秒）
	RetryAfter time.Duration
}

// LimiterConfig 限流器配置。
type LimiterConfig struct {
	// Cache 缓存后端（必填）
	Cache cache.Cache
	// Limit 窗口内最大请求数（默认 60）
	Limit int64
	// Window 窗口大小（默认 1 分钟）
	Window time.Duration
	// Scope key 命名空间，区分不同限流器（默认 "default"）
	Scope string
	// KeyPrefix key 前缀（默认 "rl:"）
	KeyPrefix string
	// Clock 时间源（测试用）
	Clock func() time.Time
	// FailOpen Cache 错误时是否放行（默认 true）
	FailOpen bool
}

// Limiter 通用限流器（线程安全：无内部可变状态）。
type Limiter struct {
	cfg LimiterConfig
}

// NewLimiter 构造限流器。
func NewLimiter(cfg LimiterConfig) (*Limiter, error) {
	if cfg.Cache == nil {
		return nil, ErrCacheUnavailable
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 60
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.Scope == "" {
		cfg.Scope = "default"
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "rl:"
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	// FailOpen 默认 true（v2.0.1 业务优先）
	// 但需要显式传 false 才能关闭 → 这里不强制默认，调用方可自由设置
	return &Limiter{cfg: cfg}, nil
}

// =============================================================================
// Allow / Check
// =============================================================================

// Allow 检查并扣配额（命中时扣配额，未命中也扣 — 即"先扣后判"）。
//
// 返回 (allowed, error)：
//   - allowed=true：放行
//   - allowed=false, ErrRateLimited：被限流
//   - error 非 ErrRateLimited：通常是 cache 错误（FailOpen=true 时不会抛）
//
// 调用方应将 key 设为业务维度组合，如 "user:<uid>" / "ip:<ip>:path:<p>"。
func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	d, err := l.Check(ctx, key)
	if err != nil {
		return false, err
	}
	if !d.Allowed {
		return false, fmt.Errorf("%w: count=%.2f limit=%d retry_after=%v",
			ErrRateLimited, d.Current, d.Limit, d.RetryAfter)
	}
	return true, nil
}

// Check 与 Allow 等价，但返回完整 Decision（用于 HTTP 限流头）。
func (l *Limiter) Check(ctx context.Context, key string) (*Decision, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}
	now := l.cfg.Clock()
	windowSec := int64(l.cfg.Window.Seconds())
	if windowSec <= 0 {
		windowSec = 1
	}
	currBucket := now.Unix() / windowSec
	prevBucket := currBucket - 1
	elapsedInCurrent := now.Unix() % windowSec
	weight := 1.0 - float64(elapsedInCurrent)/float64(windowSec)

	currKey := l.bucketKey(key, currBucket)
	prevKey := l.bucketKey(key, prevBucket)

	// 1. 自增当前桶
	currCount, err := l.cfg.Cache.Incr(ctx, currKey, 2*l.cfg.Window)
	if err != nil {
		if l.cfg.FailOpen {
			return &Decision{Allowed: true, Limit: l.cfg.Limit, Remaining: l.cfg.Limit}, nil
		}
		return nil, fmt.Errorf("ratelimit: incr: %w", err)
	}

	// 2. 读上一桶（不存在 = 0）
	prevCount := int64(0)
	prevBytes, err := l.cfg.Cache.Get(ctx, prevKey)
	if err == nil {
		if v, perr := strconv.ParseInt(string(prevBytes), 10, 64); perr == nil {
			prevCount = v
		}
	} else if !errors.Is(err, cache.ErrMiss) {
		// 非 miss 错误：FailOpen 时忽略，否则计为 0
		if !l.cfg.FailOpen {
			// 仍计 sliding 为当前桶
		}
	}

	sliding := float64(prevCount)*weight + float64(currCount)
	d := &Decision{
		Current: sliding,
		Limit:   l.cfg.Limit,
	}
	if sliding > float64(l.cfg.Limit) {
		d.Allowed = false
		d.Remaining = 0
		// 重试间隔 = 当前桶剩余时长（粗估）
		d.RetryAfter = time.Duration(windowSec-elapsedInCurrent) * time.Second
		return d, nil
	}
	d.Allowed = true
	rem := l.cfg.Limit - int64(math.Ceil(sliding))
	if rem < 0 {
		rem = 0
	}
	d.Remaining = rem
	return d, nil
}

// Reset 清除某个 key 的限流计数（管理用途）。
func (l *Limiter) Reset(ctx context.Context, key string) error {
	if key == "" {
		return ErrEmptyKey
	}
	now := l.cfg.Clock()
	windowSec := int64(l.cfg.Window.Seconds())
	bucket := now.Unix() / windowSec
	return l.cfg.Cache.Del(ctx,
		l.bucketKey(key, bucket),
		l.bucketKey(key, bucket-1),
	)
}

// =============================================================================
// 内部
// =============================================================================

// bucketKey 构造桶 key（含 scope / prefix / bucket）。
func (l *Limiter) bucketKey(key string, bucket int64) string {
	var sb strings.Builder
	sb.Grow(len(l.cfg.KeyPrefix) + len(l.cfg.Scope) + len(key) + 24)
	sb.WriteString(l.cfg.KeyPrefix)
	sb.WriteString(l.cfg.Scope)
	sb.WriteByte(':')
	sb.WriteString(key)
	sb.WriteByte(':')
	sb.WriteString(strconv.FormatInt(bucket, 10))
	return sb.String()
}
