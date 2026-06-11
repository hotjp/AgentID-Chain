// Package backend: OnchainBackend（纯链上身份后端）。
//
// 职责：
//   - 每次注册/升级/封禁/解封/注销都通过 ChainAdapter 调用合约
//   - 链上 Receipt.TxHash 透传到 AgentCredential
//   - 读路径优先走缓存（cache miss → 适配器 GetAgentState）
//
// 适用场景：开放生态、跨机构互信、需要审计追溯的场景。
//
// 设计要点：
//   - 状态机校验在链上合约完成（信任合约），本后端只做缓存 / 错误映射
//   - 持久化层在 OnchainBackend 中退化为可选：仅用于"链下元数据缓存"（permisson / metadata）
//   - 链上不可用时返回 ErrBackendUnavailable（不 fallback 到 LocalBackend，那是 HybridBackend 的职责）
package backend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
)

// =============================================================================
// OnchainBackend
// =============================================================================

// OnchainConfig OnchainBackend 配置。
type OnchainConfig struct {
	// OwnerKey 缓存 key 前缀（默认 "agentid:onchain:"）。
	OwnerKey string
	// CacheTTL 缓存 TTL（默认 5m；链上状态变更后须能容忍 stale）。
	CacheTTL time.Duration
	// HealthCheckTimeout HealthCheck 超时（默认 3s）。
	HealthCheckTimeout time.Duration
}

// OnchainBackend 纯链上身份后端。
type OnchainBackend struct {
	mu      sync.RWMutex //lint:ignore U1000 reserved for chain access serialization
	adapter chain_adapter.BaseChainAdapter
	cache   Cache
	cfg     OnchainConfig
}

// NewOnchainBackend 构造。
func NewOnchainBackend(adapter chain_adapter.BaseChainAdapter, cache Cache, cfg OnchainConfig) (*OnchainBackend, error) {
	if adapter == nil {
		return nil, ErrBackendUnavailable
	}
	if cfg.OwnerKey == "" {
		cfg.OwnerKey = "agentid:onchain:"
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.HealthCheckTimeout == 0 {
		cfg.HealthCheckTimeout = 3 * time.Second
	}
	if cache == nil {
		cache = noopCache{}
	}
	return &OnchainBackend{
		adapter: adapter,
		cache:   cache,
		cfg:     cfg,
	}, nil
}

// BackendType 后端类型。
func (b *OnchainBackend) BackendType() BackendType { return TypeOnchain }

// Close 关闭（不关闭 adapter，由调用方负责）。
func (b *OnchainBackend) Close(_ context.Context) error { return nil }

// Adapter 暴露底层链适配器（HybridBackend 复用）。
func (b *OnchainBackend) Adapter() chain_adapter.BaseChainAdapter { return b.adapter }

// =============================================================================
// RegisterAgent
// =============================================================================

// RegisterAgent 链上注册。
//
// 流程：
//  1. 生成 UUIDv7
//  2. 调 adapter.RegisterAgent（链上确认）
//  3. 写缓存（TTL 短，因链上状态可能变）
//  4. 返回 AgentCredential（含 TxHash）
func (b *OnchainBackend) RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
	if req == nil || req.Owner == "" {
		return nil, fmt.Errorf("%w: nil/empty owner", ErrBackendUnavailable)
	}
	if req.Level == 0 {
		return nil, fmt.Errorf("%w: zero level", ErrBackendUnavailable)
	}
	if req.PublicKey == "" {
		return nil, fmt.Errorf("%w: empty pubkey", ErrBackendUnavailable)
	}

	uid, err := generateUUIDv7()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	receipt, err := b.adapter.RegisterAgent(ctx, &chain_adapter.RegisterRequest{
		UUID:       uid,
		Owner:      req.Owner,
		Level:      req.Level,
		Permission: req.Permission,
		PublicKey:  req.PublicKey,
		Metadata:   req.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("onchain register: %w", err)
	}

	// 缓存标记
	_ = b.cache.Set(ctx, b.cfg.OwnerKey+uid, nil, b.cfg.CacheTTL)

	cred := &AgentCredential{
		UUID:       uid,
		Owner:      req.Owner,
		Level:      req.Level,
		State:      StateActive,
		Permission: req.Permission,
		PublicKey:  req.PublicKey,
		TxHash:     receipt.TxHash,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return cred, nil
}

// =============================================================================
// GetAgentInfo
// =============================================================================

// GetAgentInfo 查询链上状态（缓存优先）。
func (b *OnchainBackend) GetAgentInfo(ctx context.Context, uuid string) (*AgentInfo, error) {
	if uuid == "" {
		return nil, fmt.Errorf("%w: empty uuid", ErrBackendUnavailable)
	}

	agent, err := b.adapter.GetAgentState(ctx, uuid)
	if err != nil {
		// 区分 "not found" 与其他错误
		if _, ok := err.(*chain_adapter.ErrAgentNotFoundOnchain); ok {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("onchain get state: %w", err)
	}

	// 映射链上状态 → 本地 state
	state := mapChainState(agent.State)
	_ = b.cache.Set(ctx, b.cfg.OwnerKey+uuid, nil, b.cfg.CacheTTL)

	return &AgentInfo{
		UUID:         agent.UUID,
		Owner:        agent.Owner,
		Level:        agent.Level,
		State:        state,
		Permission:   agent.Permission,
		PublicKey:    agent.PublicKey,
		TxHash:       agent.TxHash,
		RegisteredAt: agent.UpdatedAt, // 链上无独立注册时间，用 UpdatedAt 兜底
		UpdatedAt:    agent.UpdatedAt,
	}, nil
}

// =============================================================================
// UpdateAgentLevel
// =============================================================================

// UpdateAgentLevel 链上升级。
func (b *OnchainBackend) UpdateAgentLevel(ctx context.Context, uuid string, newLevel uint8, reason string) error {
	if uuid == "" {
		return ErrBackendUnavailable
	}
	if newLevel == 0 {
		return fmt.Errorf("%w: zero level", ErrInvalidState)
	}

	_, err := b.adapter.UpdateLevel(ctx, &chain_adapter.UpdateLevelRequest{
		UUID:     uuid,
		NewLevel: newLevel,
		Reason:   reason,
	})
	if err != nil {
		return fmt.Errorf("onchain update level: %w", err)
	}
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// =============================================================================
// Ban / Unban
// =============================================================================

// BanAgent 链上封禁。
func (b *OnchainBackend) BanAgent(ctx context.Context, uuid string, reason string) error {
	if uuid == "" {
		return ErrBackendUnavailable
	}
	if _, err := b.adapter.BanAgent(ctx, &chain_adapter.BanRequest{UUID: uuid, Reason: reason}); err != nil {
		return fmt.Errorf("onchain ban: %w", err)
	}
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// UnbanAgent 链上解封。
func (b *OnchainBackend) UnbanAgent(ctx context.Context, uuid string) error {
	if uuid == "" {
		return ErrBackendUnavailable
	}
	if _, err := b.adapter.UnbanAgent(ctx, uuid); err != nil {
		return fmt.Errorf("onchain unban: %w", err)
	}
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// UnregisterAgent 链上注销。
func (b *OnchainBackend) UnregisterAgent(ctx context.Context, uuid string) error {
	if uuid == "" {
		return ErrBackendUnavailable
	}
	if _, err := b.adapter.RevokeAgent(ctx, uuid); err != nil {
		return fmt.Errorf("onchain revoke: %w", err)
	}
	_ = b.cache.Del(ctx, b.cfg.OwnerKey+uuid)
	return nil
}

// =============================================================================
// GetChangeLogs
// =============================================================================

// GetChangeLogs 链上后端不维护本地审计日志。
//
// 设计决策：链上是不可变账本，链上事件可通过 adapter.HealthCheck / 节点 explorer 检索。
// 本地若需要"按时间/操作人过滤"则需 HybridBackend 模式（链上 + 本地 audit 表）。
func (b *OnchainBackend) GetChangeLogs(ctx context.Context, uuid string) ([]ChangeLog, error) {
	if uuid == "" {
		return nil, ErrBackendUnavailable
	}
	// 兜底：返回一条"链上不可查"标记，供上层展示
	return []ChangeLog{
		{
			UUID:       uuid,
			Action:     "info",
			NewValue:   "onchain backend does not maintain local audit logs",
			OccurredAt: time.Now(),
		},
	}, nil
}

// =============================================================================
// BatchGetAgentInfo
// =============================================================================

// BatchGetAgentInfo 批量查询（链上：按 uuid 依次 GetAgentState）。
func (b *OnchainBackend) BatchGetAgentInfo(ctx context.Context, uuids []string) (map[string]*AgentInfo, error) {
	if len(uuids) == 0 {
		return map[string]*AgentInfo{}, nil
	}

	// 并发查询（受 ctx 取消）
	type result struct {
		info *AgentInfo
		err  error
	}
	results := make(chan result, len(uuids))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // 限流：最多 8 并发
	for _, u := range uuids {
		wg.Add(1)
		sem <- struct{}{}
		go func(uuid string) {
			defer wg.Done()
			defer func() { <-sem }()
			info, err := b.GetAgentInfo(ctx, uuid)
			results <- result{info: info, err: err}
		}(u)
	}
	wg.Wait()
	close(results)

	out := map[string]*AgentInfo{}
	for r := range results {
		if r.err == nil && r.info != nil {
			out[r.info.UUID] = r.info
		}
	}
	return out, nil
}

// =============================================================================
// 工具
// =============================================================================

// mapChainState 映射链上状态 → 本地 state。
func mapChainState(s chain_adapter.AgentState) string {
	switch s {
	case chain_adapter.StateActive:
		return StateActive
	case chain_adapter.StateBanned:
		return StateBanned
	case chain_adapter.StateRevoked:
		return StateUnregistered
	default:
		return StateRegistered
	}
}

// generateUUIDv7 生成 UUIDv7 复用 local.go 已定义的实现。
// （同包内可直接调用 generateUUIDv7()，无须重复定义）
