// Package a2a A2A Token 撤销管理（v2.0.1 §4.3.6）。
//
// 撤销模型：
//   - 单 token 撤销：在 cache 中写入 a2a:revoked:<jti> 标记，TTL = 剩余有效期（最多 1h）
//   - 批量按 agent 撤销：维护内存索引 map[agentUUID]map[jti]expireAt，遍历调用单 token 撤销
//   - 验签端：IsRevoked(jti) 检查 cache → 存在即拒绝
//
// 设计权衡：
//   - cache 层只暴露 KV 接口（无 Set/Scan），所以"按 agent 批量撤销"需内存索引
//   - 内存索引重启会丢，但被撤销标记仍在 cache，所以"漏撤销"风险仅限于
//     "新颁发但服务重启 + 重启后才需要撤销" 的窗口
//   - 多实例部署时索引不共享 → v2.0.1 单实例可接受；多实例需升级到 cluster-wide pub/sub
package a2a

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
)

// =============================================================================
// 错误与常量
// =============================================================================

// ErrJTIRequired jti 不能为空。
var ErrJTIRequired = errors.New("a2a: jti required")

// ErrCacheUnavailable cache 未配置。
var ErrCacheUnavailable = errors.New("a2a: cache unavailable")

// MaxRevocationTTL 撤销标记的最大保留时长（防止 token TTL 过长拖垮 cache）。
const MaxRevocationTTL = 1 * time.Hour

// =============================================================================
// 内部数据结构
// =============================================================================

// tokenEntry 跟踪某个 jti 的元数据。
type tokenEntry struct {
	agentUUID string
	expiresAt time.Time
}

// =============================================================================
// RevokerConfig / Revoker
// =============================================================================

// RevokerConfig 撤销器配置。
type RevokerConfig struct {
	// Cache 持久化 revoked 标记（必填）
	Cache cache.Cache
	// Clock 时间源（默认 time.Now）
	Clock func() time.Time
	// MaxTTL 单个撤销标记 TTL 上限（默认 MaxRevocationTTL）
	MaxTTL time.Duration
	// KeyPrefix cache key 前缀（默认 "a2a:revoked:"）
	KeyPrefix string
}

// Revoker A2A token 撤销管理器。
//
// 线程安全：内置 sync.RWMutex；可并发调用。
type Revoker struct {
	cfg RevokerConfig

	mu    sync.RWMutex
	index map[string]map[string]*tokenEntry // agentUUID → jti → entry
}

// NewRevoker 构造撤销器。
func NewRevoker(cfg RevokerConfig) (*Revoker, error) {
	if cfg.Cache == nil {
		return nil, ErrCacheUnavailable
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.MaxTTL <= 0 {
		cfg.MaxTTL = MaxRevocationTTL
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "a2a:revoked:"
	}
	return &Revoker{
		cfg:   cfg,
		index: make(map[string]map[string]*tokenEntry),
	}, nil
}

// =============================================================================
// Track + Revoke + IsRevoked
// =============================================================================

// Track 在颁发 token 时调用：记录 jti → agent → 过期时间到内存索引。
//
// 调用方应在 Issuer.Sign 成功后立即 Track。
func (r *Revoker) Track(agentUUID, jti string, expiresAt time.Time) error {
	if agentUUID == "" {
		return errors.New("a2a: empty agent uuid")
	}
	if jti == "" {
		return ErrJTIRequired
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	bucket, ok := r.index[agentUUID]
	if !ok {
		bucket = make(map[string]*tokenEntry)
		r.index[agentUUID] = bucket
	}
	bucket[jti] = &tokenEntry{agentUUID: agentUUID, expiresAt: expiresAt}
	return nil
}

// Revoke 单 jti 撤销，TTL 取 (expiresAt - now) 与 MaxTTL 的较小值。
func (r *Revoker) Revoke(ctx context.Context, jti string) error {
	if jti == "" {
		return ErrJTIRequired
	}
	now := r.cfg.Clock()
	ttl := r.computeTTL(jti, now)
	if ttl <= 0 {
		// 已过期，无需撤销（验签端会拒绝 expired token）
		r.untrack(jti)
		return nil
	}
	if err := r.cfg.Cache.Set(ctx, r.keyFor(jti), []byte("1"), ttl); err != nil {
		return fmt.Errorf("a2a: revoke set: %w", err)
	}
	r.untrack(jti)
	return nil
}

// RevokeByAgent 批量撤销某 agent 的所有 active token。
//
// 返回成功撤销的 jti 列表 + 第一个遇到的错误（其他 jti 仍尝试撤销）。
func (r *Revoker) RevokeByAgent(ctx context.Context, agentUUID string) ([]string, error) {
	if agentUUID == "" {
		return nil, errors.New("a2a: empty agent uuid")
	}
	r.mu.RLock()
	bucket, ok := r.index[agentUUID]
	jtis := make([]string, 0, len(bucket))
	if ok {
		for j := range bucket {
			jtis = append(jtis, j)
		}
	}
	r.mu.RUnlock()

	revoked := make([]string, 0, len(jtis))
	var firstErr error
	for _, j := range jtis {
		if err := r.Revoke(ctx, j); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		revoked = append(revoked, j)
	}
	return revoked, firstErr
}

// IsRevoked 查询 jti 是否被撤销（验签端调用）。
//
// cache 错误归为 "未撤销"（Fail Open），避免 cache 抖动拒绝合法 token；
// 真正的安全网是签名 + exp 校验。
func (r *Revoker) IsRevoked(ctx context.Context, jti string) bool {
	if jti == "" {
		return false
	}
	exists, err := r.cfg.Cache.Exists(ctx, r.keyFor(jti))
	if err != nil {
		return false
	}
	return exists
}

// ActiveJTIs 返回某 agent 当前内存索引中的活跃 jti 列表（仅用于调试 / 审计）。
func (r *Revoker) ActiveJTIs(agentUUID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bucket, ok := r.index[agentUUID]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(bucket))
	for j := range bucket {
		out = append(out, j)
	}
	return out
}

// GC 清理过期的内存索引条目（建议调用方定期触发，例如每 5 分钟）。
//
// 返回被清理的 entry 数量。
func (r *Revoker) GC() int {
	now := r.cfg.Clock()
	r.mu.Lock()
	defer r.mu.Unlock()
	cleaned := 0
	for uuid, bucket := range r.index {
		for j, e := range bucket {
			if !e.expiresAt.IsZero() && !now.Before(e.expiresAt) {
				delete(bucket, j)
				cleaned++
			}
		}
		if len(bucket) == 0 {
			delete(r.index, uuid)
		}
	}
	return cleaned
}

// =============================================================================
// 内部辅助
// =============================================================================

// keyFor 拼装 cache key。
func (r *Revoker) keyFor(jti string) string {
	if strings.HasPrefix(jti, r.cfg.KeyPrefix) {
		return jti
	}
	return r.cfg.KeyPrefix + jti
}

// untrack 从内存索引移除 jti（多 agent 桶安全）。
func (r *Revoker) untrack(jti string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for uuid, bucket := range r.index {
		if _, ok := bucket[jti]; ok {
			delete(bucket, jti)
			if len(bucket) == 0 {
				delete(r.index, uuid)
			}
		}
	}
}

// computeTTL 根据 jti 在索引中的过期时间计算剩余 TTL；找不到则用 MaxTTL。
func (r *Revoker) computeTTL(jti string, now time.Time) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, bucket := range r.index {
		if e, ok := bucket[jti]; ok && !e.expiresAt.IsZero() {
			remaining := e.expiresAt.Sub(now)
			if remaining > r.cfg.MaxTTL {
				return r.cfg.MaxTTL
			}
			return remaining
		}
	}
	// 索引找不到（可能服务重启），保守用 MaxTTL
	return r.cfg.MaxTTL
}
