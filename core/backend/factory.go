// Package backend: Backend Factory。
//
// 设计：根据 Config 返回对应 IdentityBackend 实现。
//   - TypeLocal   → LocalBackend（默认）
//   - TypeOnchain → OnchainBackend（需要 ChainAdapter）
//   - TypeHybrid  → HybridBackend（需要 ChainAdapter + Persistence）
//   - TypeMock    → LocalBackend with MemoryPersistence + MockAdapter（开发/测试）
//
// 用法（典型 wire）：
//
//	cfg := backend.Config{
//	    Type:         backend.TypeHybrid,
//	    ChainAdapter: adapter,
//	    Persistence:  pers,
//	    Cache:        redisCache,
//	}
//	be, err := backend.New(cfg)
package backend

import (
	"fmt"
	"time"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
)

// =============================================================================
// Config
// =============================================================================

// Config 工厂配置。
type Config struct {
	// Type 后端类型（local / onchain / hybrid / mock）。
	Type BackendType
	// OwnerKey 缓存 key 前缀（默认 "agentid:<type>:"）。
	OwnerKey string
	// CacheTTL 缓存 TTL（默认按 type 不同：local=15m, onchain=5m, hybrid=10m）。
	CacheTTL time.Duration

	// === Local / Hybrid 字段 ===
	// Persistence 本地持久化（Local / Hybrid 需要；nil = 默认内存实现）。
	Persistence Persistence
	// LocalTTL 内存保留最大日志数（0 = 默认 1000；仅 Local）。
	MaxLogsPerAgent int

	// === Onchain / Hybrid 字段 ===
	// ChainAdapter 链适配器（Onchain / Hybrid 需要）。
	ChainAdapter chain_adapter.BaseChainAdapter

	// === Hybrid 字段 ===
	// SyncInterval 链上→本地同步间隔（0 = 默认 5m；0 = 禁用）。
	SyncInterval time.Duration
	// SyncBatchSize 同步批大小（0 = 默认 100）。
	SyncBatchSize int

	// === Mock 字段（仅 TypeMock 生效）===
	// MockChainID mock 链 ID（默认 1337）。
	MockChainID uint64
}

// =============================================================================
// Factory
// =============================================================================

// NewBackend 工厂入口。
//
// 返回：
//   - IdentityBackend：根据 cfg.Type 路由到具体实现
//   - error：配置非法 / 依赖缺失
//
// 失败兜底（nil backend + err）：
//   - cfg.Type 为空 → 默认 TypeLocal
//   - cfg.Type 未知 → 返回 ErrBackendUnavailable
//   - 任何子构造失败 → 透传子错误
func NewBackend(cfg Config) (IdentityBackend, error) {
	// 1. 类型默认
	if cfg.Type == "" {
		cfg.Type = TypeLocal
	}

	// 2. CacheTTL 默认
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = defaultCacheTTL(cfg.Type)
	}

	// 3. OwnerKey 默认
	if cfg.OwnerKey == "" {
		cfg.OwnerKey = "agentid:" + string(cfg.Type) + ":"
	}

	switch cfg.Type {
	case TypeLocal:
		return newLocalFromConfig(cfg)
	case TypeOnchain:
		return newOnchainFromConfig(cfg)
	case TypeHybrid:
		return newHybridFromConfig(cfg)
	case TypeMock:
		return newMockFromConfig(cfg)
	default:
		return nil, fmt.Errorf("%w: unknown backend type %q", ErrBackendUnavailable, cfg.Type)
	}
}

func newLocalFromConfig(cfg Config) (IdentityBackend, error) {
	pers := cfg.Persistence
	if pers == nil {
		pers = NewMemoryPersistence()
	}
	return NewLocalBackend(pers, nil, LocalConfig{
		OwnerKey:        cfg.OwnerKey,
		CacheTTL:        cfg.CacheTTL,
		MaxLogsPerAgent: cfg.MaxLogsPerAgent,
	})
}

func newOnchainFromConfig(cfg Config) (IdentityBackend, error) {
	if cfg.ChainAdapter == nil {
		return nil, fmt.Errorf("%w: onchain backend requires ChainAdapter", ErrBackendUnavailable)
	}
	return NewOnchainBackend(cfg.ChainAdapter, nil, OnchainConfig{
		OwnerKey: cfg.OwnerKey,
		CacheTTL: cfg.CacheTTL,
	})
}

func newHybridFromConfig(cfg Config) (IdentityBackend, error) {
	if cfg.ChainAdapter == nil {
		return nil, fmt.Errorf("%w: hybrid backend requires ChainAdapter", ErrBackendUnavailable)
	}
	if cfg.Persistence == nil {
		return nil, fmt.Errorf("%w: hybrid backend requires Persistence", ErrBackendUnavailable)
	}
	return NewHybridBackend(cfg.ChainAdapter, cfg.Persistence, nil, HybridConfig{
		OwnerKey:       cfg.OwnerKey,
		CacheTTL:       cfg.CacheTTL,
		SyncInterval:   cfg.SyncInterval,
		SyncBatchSize:  cfg.SyncBatchSize,
	})
}

func newMockFromConfig(cfg Config) (IdentityBackend, error) {
	chainID := cfg.MockChainID
	if chainID == 0 {
		chainID = 1337
	}
	_ = chainID // 当前未用：mock.NewMockAdapter 不接受 chainID；如需定制化，调用方应自行构造 adapter
	adapter := mock.NewMockAdapter()
	// 包装成 hybrid 行为（mock 是开发/测试用，期望看到完整链路）
	pers := cfg.Persistence
	if pers == nil {
		pers = NewMemoryPersistence()
	}
	syncInt := cfg.SyncInterval
	if syncInt == 0 {
		syncInt = 1 * time.Minute
	}
	return NewHybridBackend(adapter, pers, nil, HybridConfig{
		OwnerKey:      cfg.OwnerKey,
		CacheTTL:      cfg.CacheTTL,
		SyncInterval:  syncInt,
		SyncBatchSize: cfg.SyncBatchSize,
	})
}

// =============================================================================
// 工具
// =============================================================================

// defaultCacheTTL 按后端类型返回默认缓存 TTL。
func defaultCacheTTL(t BackendType) time.Duration {
	switch t {
	case TypeLocal:
		return 15 * time.Minute
	case TypeOnchain:
		return 5 * time.Minute
	case TypeHybrid:
		return 10 * time.Minute
	case TypeMock:
		return 1 * time.Minute
	default:
		return 5 * time.Minute
	}
}
