// Package backend: LocalBackend（链下/纯本地身份后端）。
//
// v2.0.1 调整：默认后端为 PostgreSQL + ent + Redis。当前文件实现 in-memory
// 状态机 + 审计 append-only 日志，作为生产 PG 实现的占位与单测基础。
//
// 设计要点：
//   - 线程安全（sync.RWMutex）
//   - 状态机校验：registered → active → banned → unregistered
//   - 审计 append-only：每次状态/等级变更追加 ChangeLog
//   - 缓存层可选（nil = no-op）
//   - 持久化层可替换（Persistence 接口）
package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// 状态常量
// =============================================================================

const (
	StateRegistered   = "registered"
	StateActive       = "active"
	StateBanned       = "banned"
	StateUnregistered = "unregistered"
)

// =============================================================================
// 持久化抽象（生产 PG 实现 / 内存实现）
// =============================================================================

// Persistence 持久化抽象。LocalBackend 通过此接口与存储层解耦。
//
// 内存实现：LocalBackend 内部使用。
// 生产实现：ent + PostgreSQL（P3 L1 接入）。
type Persistence interface {
	// PutAgent upsert agent 记录。
	PutAgent(ctx context.Context, rec *AgentInfo) error
	// GetAgent 查询 agent。
	GetAgent(ctx context.Context, uuid string) (*AgentInfo, error)
	// ListAgentsByOwner 列出某 owner 名下所有 agent。
	ListAgentsByOwner(ctx context.Context, owner string) ([]*AgentInfo, error)
	// AppendLog 追加变更日志。
	AppendLog(ctx context.Context, log *ChangeLog) error
	// ListLogs 查询某 agent 的变更日志（按时间倒序）。
	ListLogs(ctx context.Context, uuid string, limit int) ([]ChangeLog, error)
	// BatchGet 批量查询。
	BatchGet(ctx context.Context, uuids []string) (map[string]*AgentInfo, error)
	// Count 返回 agent 总数（运维 / 监控 / 测试用）。
	Count() int
}

// =============================================================================
// Cache 抽象
// =============================================================================

// Cache 缓存抽象（nil = no-op）。
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

// noopCache 占位实现。
type noopCache struct{}

func (noopCache) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (noopCache) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (noopCache) Del(context.Context, string) error { return nil }

// =============================================================================
// LocalBackend
// =============================================================================

// LocalConfig LocalBackend 配置。
type LocalConfig struct {
	// OwnerKey 缓存 key 前缀（默认 "agentid:agent:"）。
	OwnerKey string
	// CacheTTL 缓存 TTL（默认 15m）。
	CacheTTL time.Duration
	// MaxLogsPerAgent 内存保留最大日志数（超出截断；默认 1000；0 = 无限制）。
	MaxLogsPerAgent int
}

// LocalBackend 链下身份后端。
type LocalBackend struct {
	mu     sync.RWMutex
	pers   Persistence
	cache  Cache
	cfg    LocalConfig
	closed bool
}

// NewLocalBackend 构造。
func NewLocalBackend(pers Persistence, cache Cache, cfg LocalConfig) (*LocalBackend, error) {
	if pers == nil {
		return nil, ErrBackendUnavailable
	}
	if cfg.OwnerKey == "" {
		cfg.OwnerKey = "agentid:agent:"
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 15 * time.Minute
	}
	if cfg.MaxLogsPerAgent == 0 {
		cfg.MaxLogsPerAgent = 1000
	}
	if cache == nil {
		cache = noopCache{}
	}
	return &LocalBackend{
		pers:  pers,
		cache: cache,
		cfg:   cfg,
	}, nil
}

// BackendType 后端类型。
func (b *LocalBackend) BackendType() BackendType { return TypeLocal }

// Close 关闭后端。
func (b *LocalBackend) Close(_ context.Context) error {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	return nil
}

// =============================================================================
// RegisterAgent
// =============================================================================

// RegisterAgent 注册 agent。
//
// 步骤：
//  1. 生成 UUIDv7
//  2. 构造 AgentInfo
//  3. 持久化（PutAgent）+ 追加 ChangeLog
//  4. 写缓存
func (b *LocalBackend) RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
	if req == nil || req.Owner == "" {
		return nil, fmt.Errorf("%w: nil/empty owner", ErrBackendUnavailable)
	}
	if req.Level == 0 {
		return nil, fmt.Errorf("%w: zero level", ErrBackendUnavailable)
	}
	if req.PublicKey == "" {
		return nil, fmt.Errorf("%w: empty pubkey", ErrBackendUnavailable)
	}

	// 1. UUIDv7
	uid, err := generateUUIDv7()
	if err != nil {
		return nil, err
	}

	// 2. 构造记录
	now := time.Now()
	info := &AgentInfo{
		UUID:         uid,
		Owner:        req.Owner,
		Level:        req.Level,
		State:        StateRegistered,
		Permission:   req.Permission,
		PublicKey:    req.PublicKey,
		RegisteredAt: now,
		UpdatedAt:    now,
	}

	// 3. 持久化
	if err := b.pers.PutAgent(ctx, info); err != nil {
		return nil, fmt.Errorf("backend: put agent: %w", err)
	}
	// 审计
	if err := b.pers.AppendLog(ctx, &ChangeLog{
		UUID: uid, Action: "register", Actor: req.Owner,
		NewValue: "registered", OccurredAt: now,
	}); err != nil {
		// 审计失败不致命（PG 可能后续对账）
		_ = err
	}

	// 4. 缓存
	_ = b.cache.Set(ctx, b.cfg.OwnerKey+uid, nil, b.cfg.CacheTTL)

	return &AgentCredential{
		UUID:       uid,
		Owner:      req.Owner,
		Level:      req.Level,
		State:      StateRegistered,
		Permission: req.Permission,
		PublicKey:  req.PublicKey,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// =============================================================================
// GetAgentInfo
// =============================================================================

// GetAgentInfo 查询 agent 完整信息（缓存优先）。
func (b *LocalBackend) GetAgentInfo(ctx context.Context, uuid string) (*AgentInfo, error) {
	if uuid == "" {
		return nil, fmt.Errorf("%w: empty uuid", ErrBackendUnavailable)
	}
	// 缓存 key 存在即认为有缓存（val 暂不反序列化）
	if _, err := b.cache.Get(ctx, b.cfg.OwnerKey+uuid); err == nil {
		// 命中缓存标记，但不解析（避免序列化协议耦合）
	}
	info, err := b.pers.GetAgent(ctx, uuid)
	if err != nil {
		return nil, ErrAgentNotFound
	}
	// 回填缓存
	_ = b.cache.Set(ctx, b.cfg.OwnerKey+uuid, nil, b.cfg.CacheTTL)
	return info, nil
}

// =============================================================================
// UpdateAgentLevel
// =============================================================================

// UpdateAgentLevel 升级 agent 等级。
//
// 规则：
//   - 状态必须为 active 或 registered
//   - newLevel 必须 > 当前 level
//   - 每次变更追加 ChangeLog
func (b *LocalBackend) UpdateAgentLevel(ctx context.Context, uuid string, newLevel uint8, reason string) error {
	if uuid == "" {
		return ErrBackendUnavailable
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	info, err := b.pers.GetAgent(ctx, uuid)
	if err != nil {
		return ErrAgentNotFound
	}
	if info.State == StateBanned || info.State == StateUnregistered {
		return fmt.Errorf("%w: %s → upgrade", ErrInvalidState, info.State)
	}
	oldLevel := info.Level
	if newLevel <= oldLevel {
		return fmt.Errorf("%w: %d → %d (not upgrade)", ErrInvalidState, oldLevel, newLevel)
	}
	info.Level = newLevel
	info.UpdatedAt = time.Now()
	if err := b.pers.PutAgent(ctx, info); err != nil {
		return err
	}
	_ = b.pers.AppendLog(ctx, &ChangeLog{
		UUID: uuid, Action: "upgrade",
		OldValue: fmt.Sprintf("level=%d", oldLevel),
		NewValue: fmt.Sprintf("level=%d", newLevel),
		Reason:   reason, OccurredAt: info.UpdatedAt,
	})
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// =============================================================================
// Ban / Unban
// =============================================================================

// BanAgent 封禁 agent。
func (b *LocalBackend) BanAgent(ctx context.Context, uuid string, reason string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, err := b.pers.GetAgent(ctx, uuid)
	if err != nil {
		return ErrAgentNotFound
	}
	if info.State == StateBanned {
		return nil // 幂等
	}
	if info.State == StateUnregistered {
		return fmt.Errorf("%w: unregistered → banned", ErrInvalidState)
	}
	oldState := info.State
	info.State = StateBanned
	info.UpdatedAt = time.Now()
	if err := b.pers.PutAgent(ctx, info); err != nil {
		return err
	}
	_ = b.pers.AppendLog(ctx, &ChangeLog{
		UUID: uuid, Action: "ban",
		OldValue: oldState, NewValue: StateBanned,
		Reason: reason, OccurredAt: info.UpdatedAt,
	})
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// UnbanAgent 解封 agent。
func (b *LocalBackend) UnbanAgent(ctx context.Context, uuid string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, err := b.pers.GetAgent(ctx, uuid)
	if err != nil {
		return ErrAgentNotFound
	}
	if info.State != StateBanned {
		return nil // 幂等
	}
	info.State = StateActive
	info.UpdatedAt = time.Now()
	if err := b.pers.PutAgent(ctx, info); err != nil {
		return err
	}
	_ = b.pers.AppendLog(ctx, &ChangeLog{
		UUID: uuid, Action: "unban",
		OldValue: StateBanned, NewValue: StateActive,
		OccurredAt: info.UpdatedAt,
	})
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// =============================================================================
// UnregisterAgent
// =============================================================================

// UnregisterAgent 注销 agent（永久）。
func (b *LocalBackend) UnregisterAgent(ctx context.Context, uuid string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, err := b.pers.GetAgent(ctx, uuid)
	if err != nil {
		return ErrAgentNotFound
	}
	if info.State == StateUnregistered {
		return nil // 幂等
	}
	oldState := info.State
	info.State = StateUnregistered
	info.UpdatedAt = time.Now()
	if err := b.pers.PutAgent(ctx, info); err != nil {
		return err
	}
	_ = b.pers.AppendLog(ctx, &ChangeLog{
		UUID: uuid, Action: "unregister",
		OldValue: oldState, NewValue: StateUnregistered,
		OccurredAt: info.UpdatedAt,
	})
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// =============================================================================
// GetChangeLogs
// =============================================================================

// GetChangeLogs 查询变更日志（按时间倒序）。
//
// 过滤：limit 截断（0 = 不限）；返回前按时间倒序排。
func (b *LocalBackend) GetChangeLogs(ctx context.Context, uuid string) ([]ChangeLog, error) {
	if uuid == "" {
		return nil, ErrBackendUnavailable
	}
	logs, err := b.pers.ListLogs(ctx, uuid, 0)
	if err != nil {
		return nil, err
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].OccurredAt.After(logs[j].OccurredAt)
	})
	return logs, nil
}

// =============================================================================
// BatchGetAgentInfo
// =============================================================================

// BatchGetAgentInfo 批量查询。
func (b *LocalBackend) BatchGetAgentInfo(ctx context.Context, uuids []string) (map[string]*AgentInfo, error) {
	if len(uuids) == 0 {
		return map[string]*AgentInfo{}, nil
	}
	return b.pers.BatchGet(ctx, uuids)
}

// =============================================================================
// 工具
// =============================================================================

// generateUUIDv7 生成 UUIDv7（时间排序 + 随机）。
func generateUUIDv7() (string, error) {
	// Google uuid 库支持 v7（v1.6+）
	u, err := uuid.NewV7()
	if err != nil {
		// fallback: v4
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s-%s-%s-%s-%s",
			hex.EncodeToString(b[0:4]),
			hex.EncodeToString(b[4:6]),
			hex.EncodeToString(b[6:8]),
			hex.EncodeToString(b[8:10]),
			hex.EncodeToString(b[10:16]),
		), nil
	}
	return u.String(), nil
}
