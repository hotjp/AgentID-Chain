// Package backend: HybridBackend（写上链 + 查本地 + 定时同步）。
//
// 职责：
//   - 写：先上链（强审计/跨机构互信），再写本地（高性能读 / 审计查询）
//   - 读：本地优先（cache-aside），本地未命中 → 链上回源
//   - 同步：后台定时把链上状态回填到本地（防丢、防漂移）
//
// 适用场景："既要审计追溯，又要高并发性能"的企业场景。
//
// 设计要点：
//   - 内部组合 OnchainBackend（写路径）+ LocalBackend（读路径）
//   - 写顺序：链上成功 → 写本地（best-effort，本地失败不致命）
//   - 链上失败时直接返回错误（不写本地）
//   - 后台同步可独立启停（Start / Stop）
package backend

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
)

// =============================================================================
// HybridConfig
// =============================================================================

// HybridConfig HybridBackend 配置。
type HybridConfig struct {
	// OwnerKey 本地缓存 key 前缀（默认 "agentid:hybrid:"）。
	OwnerKey string
	// CacheTTL 本地缓存 TTL（默认 10m）。
	CacheTTL time.Duration
	// SyncInterval 链上→本地同步间隔（默认 5m；0 = 禁用后台同步）。
	SyncInterval time.Duration
	// SyncBatchSize 单次同步批大小（默认 100；0 = 全量）。
	SyncBatchSize int
	// LocalTTL 本地持久化的额外覆盖（nil = 使用 LocalBackend 默认）。
	LocalTTL time.Duration
}

// HybridBackend 混合身份后端。
type HybridBackend struct {
	mu       sync.RWMutex //lint:ignore U1000 reserved for hybrid concurrency control
	chain    *OnchainBackend
	local    *LocalBackend
	cfg      HybridConfig
	syncer   *backgroundSyncer
	started  atomic.Bool
	stopOnce sync.Once
}

// =============================================================================
// 构造
// =============================================================================

// NewHybridBackend 构造（同时需要链适配器 + 本地持久化）。
func NewHybridBackend(adapter chain_adapter.BaseChainAdapter, pers Persistence, cache Cache, cfg HybridConfig) (*HybridBackend, error) {
	if adapter == nil {
		return nil, ErrBackendUnavailable
	}
	if pers == nil {
		return nil, ErrBackendUnavailable
	}
	if cfg.OwnerKey == "" {
		cfg.OwnerKey = "agentid:hybrid:"
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 10 * time.Minute
	}
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = 5 * time.Minute
	}
	if cfg.SyncBatchSize == 0 {
		cfg.SyncBatchSize = 100
	}
	if cache == nil {
		cache = noopCache{}
	}

	chainBe, err := NewOnchainBackend(adapter, cache, OnchainConfig{
		OwnerKey: cfg.OwnerKey + "chain:",
		CacheTTL: cfg.CacheTTL,
	})
	if err != nil {
		return nil, err
	}
	localBe, err := NewLocalBackend(pers, cache, LocalConfig{
		OwnerKey: cfg.OwnerKey + "local:",
		CacheTTL: cfg.CacheTTL,
	})
	if err != nil {
		return nil, err
	}

	hb := &HybridBackend{
		chain: chainBe,
		local: localBe,
		cfg:   cfg,
	}
	hb.syncer = &backgroundSyncer{backend: hb}
	return hb, nil
}

// BackendType 后端类型。
func (h *HybridBackend) BackendType() BackendType { return TypeHybrid }

// Close 关闭（停止后台同步 + 关闭子后端）。
func (h *HybridBackend) Close(ctx context.Context) error {
	h.stopOnce.Do(func() {
		if h.syncer != nil {
			h.syncer.Stop()
		}
	})
	if err := h.chain.Close(ctx); err != nil {
		return err
	}
	return h.local.Close(ctx)
}

// Start 启动后台同步（幂等）。
func (h *HybridBackend) Start(ctx context.Context) error {
	if !h.started.CompareAndSwap(false, true) {
		return nil
	}
	if h.cfg.SyncInterval <= 0 {
		return nil
	}
	return h.syncer.Start(ctx, h.cfg.SyncInterval, h.cfg.SyncBatchSize)
}

// =============================================================================
// 写：链上优先 → 本地镜像
// =============================================================================

// RegisterAgent 链上注册 + 本地镜像。
func (h *HybridBackend) RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
	cred, err := h.chain.RegisterAgent(ctx, req)
	if err != nil {
		return nil, err
	}
	// 链上成功，本地镜像（best-effort；失败不致命）
	_ = h.mirrorToLocal(ctx, cred)
	return cred, nil
}

// UpdateAgentLevel 链上升级 + 本地升级。
func (h *HybridBackend) UpdateAgentLevel(ctx context.Context, uuid string, newLevel uint8, reason string) error {
	if err := h.chain.UpdateAgentLevel(ctx, uuid, newLevel, reason); err != nil {
		return err
	}
	// 本地升级（best-effort）
	_ = h.local.UpdateAgentLevel(ctx, uuid, newLevel, reason)
	return nil
}

// BanAgent 链上封禁 + 本地封禁。
func (h *HybridBackend) BanAgent(ctx context.Context, uuid string, reason string) error {
	if err := h.chain.BanAgent(ctx, uuid, reason); err != nil {
		return err
	}
	_ = h.local.BanAgent(ctx, uuid, reason)
	return nil
}

// UnbanAgent 链上解封 + 本地解封。
func (h *HybridBackend) UnbanAgent(ctx context.Context, uuid string) error {
	if err := h.chain.UnbanAgent(ctx, uuid); err != nil {
		return err
	}
	_ = h.local.UnbanAgent(ctx, uuid)
	return nil
}

// UnregisterAgent 链上注销 + 本地注销。
func (h *HybridBackend) UnregisterAgent(ctx context.Context, uuid string) error {
	if err := h.chain.UnregisterAgent(ctx, uuid); err != nil {
		return err
	}
	_ = h.local.UnregisterAgent(ctx, uuid)
	return nil
}

// =============================================================================
// 读：本地优先 → 链上回源 → 回填本地
// =============================================================================

// GetAgentInfo 本地优先（cache-aside）。
func (h *HybridBackend) GetAgentInfo(ctx context.Context, uuid string) (*AgentInfo, error) {
	if uuid == "" {
		return nil, fmt.Errorf("%w: empty uuid", ErrBackendUnavailable)
	}

	// 1. 尝试本地
	info, err := h.local.GetAgentInfo(ctx, uuid)
	if err == nil {
		return info, nil
	}
	if err != ErrAgentNotFound {
		// 本地故障（DB 不可用等）→ 不阻塞，回源链上
		_ = err
	}

	// 2. 回源链上
	chainInfo, chainErr := h.chain.GetAgentInfo(ctx, uuid)
	if chainErr != nil {
		if chainErr == ErrAgentNotFound {
			return nil, ErrAgentNotFound
		}
		return nil, chainErr
	}

	// 3. 回填本地（best-effort）
	_ = h.local.pers.PutAgent(ctx, chainInfo)
	return chainInfo, nil
}

// GetChangeLogs 本地优先（链上不维护审计）。
func (h *HybridBackend) GetChangeLogs(ctx context.Context, uuid string) ([]ChangeLog, error) {
	return h.local.GetChangeLogs(ctx, uuid)
}

// BatchGetAgentInfo 本地优先（链上 fallback）。
func (h *HybridBackend) BatchGetAgentInfo(ctx context.Context, uuids []string) (map[string]*AgentInfo, error) {
	if len(uuids) == 0 {
		return map[string]*AgentInfo{}, nil
	}

	// 1. 本地批量
	localOut, _ := h.local.BatchGetAgentInfo(ctx, uuids)

	// 2. 找出本地未命中的
	missing := make([]string, 0, len(uuids)-len(localOut))
	for _, u := range uuids {
		if _, ok := localOut[u]; !ok {
			missing = append(missing, u)
		}
	}

	// 3. 链上补齐
	if len(missing) > 0 {
		chainOut, err := h.chain.BatchGetAgentInfo(ctx, missing)
		if err == nil {
			for u, info := range chainOut {
				localOut[u] = info
				// 回填本地
				_ = h.local.pers.PutAgent(ctx, info)
			}
		}
	}

	return localOut, nil
}

// =============================================================================
// 对账 / 同步辅助
// =============================================================================

// SyncNow 立即触发一次同步（测试 + 运维用）。
func (h *HybridBackend) SyncNow(ctx context.Context) error {
	return h.syncer.runOnce(ctx, h.cfg.SyncBatchSize)
}

// mirrorToLocal 把刚在链上注册的 agent 同步到本地。
func (h *HybridBackend) mirrorToLocal(ctx context.Context, cred *AgentCredential) error {
	if cred == nil {
		return nil
	}
	info := &AgentInfo{
		UUID:         cred.UUID,
		Owner:        cred.Owner,
		Level:        cred.Level,
		State:        cred.State,
		Permission:   cred.Permission,
		PublicKey:    cred.PublicKey,
		TxHash:       cred.TxHash,
		RegisteredAt: cred.CreatedAt,
		UpdatedAt:    cred.UpdatedAt,
	}
	if err := h.local.pers.PutAgent(ctx, info); err != nil {
		return err
	}
	// 追加本地审计（register 事件；保持与 LocalBackend.RegisterAgent 一致）
	_ = h.local.pers.AppendLog(ctx, &ChangeLog{
		UUID:       cred.UUID,
		Action:     "register",
		Actor:      cred.Owner,
		NewValue:   cred.State,
		TxHash:     cred.TxHash,
		OccurredAt: cred.CreatedAt,
	})
	return nil
}

// LocalBackend 暴露本地后端（给同步器用）。
func (h *HybridBackend) LocalBackend() *LocalBackend { return h.local }

// ChainBackend 暴露链上后端（给同步器用）。
func (h *HybridBackend) ChainBackend() *OnchainBackend { return h.chain }

// =============================================================================
// 后台同步
// =============================================================================

// backgroundSyncer 链上 → 本地后台同步器。
type backgroundSyncer struct {
	backend *HybridBackend
	cancel  context.CancelFunc
	stopped chan struct{}
}

// Start 启动同步循环。
func (s *backgroundSyncer) Start(parent context.Context, interval time.Duration, _ int) error {
	if s.stopped != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.stopped = make(chan struct{})

	go func() {
		defer close(s.stopped)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = s.runOnce(ctx, 0) // 0 = 使用 backend 配置
			}
		}
	}()
	return nil
}

// Stop 停止同步。
func (s *backgroundSyncer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.stopped != nil {
		<-s.stopped
	}
}

// runOnce 跑一次同步：列出链上所有 agent，逐个与本地对比，时间戳更新则回写。
func (s *backgroundSyncer) runOnce(parent context.Context, batchSize int) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	adp := s.backend.chain.Adapter()
	// 仅有 mock adapter 实现 ListAll；其他真实链驱动通过事件订阅/索引器获取变更。
	lister, ok := adp.(chainLister)
	if !ok {
		return nil // 不支持列表的对账（真实链通常通过事件订阅）
	}
	_ = batchSize // 预留：未来分批

	all := lister.ListAll()
	for _, c := range all {
		local, err := s.backend.local.pers.GetAgent(ctx, c.UUID)
		if err == nil && !local.UpdatedAt.Before(c.UpdatedAt) {
			continue // 本地更新或相同 → 跳过
		}
		// 链上更新 → 回填
		chainInfo, err := s.backend.chain.GetAgentInfo(ctx, c.UUID)
		if err != nil {
			continue
		}
		_ = s.backend.local.pers.PutAgent(ctx, chainInfo)
	}
	return nil
}

// chainLister 列出链上所有 agent 的能力（mock adapter 实现）。
// 真实链驱动（FISCO/Polygon/BSC）通常通过事件订阅 / indexer 暴露等效能力，
// 实际项目应新增等价的 indexer 实现。
type chainLister interface {
	ListAll() []*chain_adapter.AgentOnchain
}
