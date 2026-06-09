// Package mock 内存版 ChainAdapter（开发 / 集成测试 / 本地启动用）。
//
// 特性：
//   - 线程安全（sync.RWMutex）
//   - 全量状态可查（ListAll / Dump / Restore）
//   - 可选：注入错误（InjectError）模拟链上失败
//   - 可选：网络延迟（Latency）模拟 RPC 抖动
//   - 可选：HTTP 暴露（通过 cmd/mock-chain/main.go）
//
// 不持久化：进程重启即清空。
package mock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
)

// MockAdapter 内存 mock 链适配器。
type MockAdapter struct {
	mu       sync.RWMutex
	agents   map[string]*chain_adapter.AgentOnchain // uuid → agent
	nonces   map[string]uint64                     // uuid → tx nonce
	chainID  uint64
	blockNo  uint64

	// 可注入的测试钩子
	latency   time.Duration          // 模拟 RPC 延迟
	errFn     func(op string) error  // 注入错误（返回非 nil 即视为链上失败）
	closed    atomic.Bool
}

// NewMockAdapter 构造默认 mock 适配器（chainID=1337，block=1）。
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		agents:  map[string]*chain_adapter.AgentOnchain{},
		nonces:  map[string]uint64{},
		chainID: 1337,
		blockNo: 1,
	}
}

// NewWithConfig 构造（指定 chainID / 起始 block）。
func NewWithConfig(chainID, startBlock uint64) *MockAdapter {
	return &MockAdapter{
		agents:  map[string]*chain_adapter.AgentOnchain{},
		nonces:  map[string]uint64{},
		chainID: chainID,
		blockNo: startBlock,
	}
}

// ChainType 返回驱动标识。
func (m *MockAdapter) ChainType() chain_adapter.ChainType {
	return chain_adapter.ChainTypeMock
}

// SetLatency 设置模拟 RPC 延迟（每次操作 sleep 该时长）。
func (m *MockAdapter) SetLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latency = d
}

// InjectError 注入错误回调（返回非 nil 即视为链上失败；nil = 恢复）。
func (m *MockAdapter) InjectError(fn func(op string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errFn = fn
}

// ChainID 返回链 ID。
func (m *MockAdapter) ChainID() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.chainID
}

// BlockNumber 返回当前 block number。
func (m *MockAdapter) BlockNumber() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.blockNo
}

// RegisterAgent 上链注册。
func (m *MockAdapter) RegisterAgent(ctx context.Context, req *chain_adapter.RegisterRequest) (*chain_adapter.Receipt, error) {
	if err := m.preflight("register"); err != nil {
		return nil, err
	}
	if req == nil || req.UUID == "" {
		return nil, &chain_adapter.ErrTxFailed{Reason: "empty uuid"}
	}
	if req.Owner == "" {
		return nil, &chain_adapter.ErrTxFailed{Reason: "empty owner"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[req.UUID]; exists {
		return nil, &chain_adapter.ErrTxFailed{Reason: "uuid already exists onchain"}
	}

	now := time.Now()
	m.blockNo++
	m.nonces[req.UUID] = 0
	m.agents[req.UUID] = &chain_adapter.AgentOnchain{
		UUID:       req.UUID,
		Owner:      req.Owner,
		Level:      req.Level,
		State:      chain_adapter.StateActive,
		Permission: req.Permission,
		PublicKey:  req.PublicKey,
		TxHash:     m.nextTxHashLocked(),
		UpdatedAt:  now,
	}

	return &chain_adapter.Receipt{
		TxHash:      m.agents[req.UUID].TxHash,
		BlockNumber: m.blockNo,
		GasUsed:     21000,
		ConfirmedAt: now,
	}, nil
}

// UpdateLevel 链上更新 Level。
func (m *MockAdapter) UpdateLevel(ctx context.Context, req *chain_adapter.UpdateLevelRequest) (*chain_adapter.Receipt, error) {
	if err := m.preflight("update_level"); err != nil {
		return nil, err
	}
	if req == nil || req.UUID == "" {
		return nil, &chain_adapter.ErrTxFailed{Reason: "empty uuid"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[req.UUID]
	if !ok {
		return nil, &chain_adapter.ErrAgentNotFoundOnchain{UUID: req.UUID}
	}
	if agent.State == chain_adapter.StateRevoked {
		return nil, &chain_adapter.ErrTxFailed{Reason: "agent revoked"}
	}

	now := time.Now()
	m.blockNo++
	agent.Level = req.NewLevel
	agent.UpdatedAt = now
	m.nonces[req.UUID]++
	txHash := m.nextTxHashLocked()

	return &chain_adapter.Receipt{
		TxHash:      txHash,
		BlockNumber: m.blockNo,
		GasUsed:     30000,
		ConfirmedAt: now,
	}, nil
}

// BanAgent 链上封禁。
func (m *MockAdapter) BanAgent(ctx context.Context, req *chain_adapter.BanRequest) (*chain_adapter.Receipt, error) {
	if err := m.preflight("ban"); err != nil {
		return nil, err
	}
	if req == nil || req.UUID == "" {
		return nil, &chain_adapter.ErrTxFailed{Reason: "empty uuid"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[req.UUID]
	if !ok {
		return nil, &chain_adapter.ErrAgentNotFoundOnchain{UUID: req.UUID}
	}
	if agent.State == chain_adapter.StateRevoked {
		return nil, &chain_adapter.ErrTxFailed{Reason: "agent revoked"}
	}
	if agent.State == chain_adapter.StateBanned {
		// 幂等：直接返回 noop
		return &chain_adapter.Receipt{
			TxHash:      agent.TxHash,
			BlockNumber: m.blockNo,
			GasUsed:     0,
			ConfirmedAt: time.Now(),
		}, nil
	}

	now := time.Now()
	m.blockNo++
	agent.State = chain_adapter.StateBanned
	agent.UpdatedAt = now
	m.nonces[req.UUID]++
	txHash := m.nextTxHashLocked()

	return &chain_adapter.Receipt{
		TxHash:      txHash,
		BlockNumber: m.blockNo,
		GasUsed:     30000,
		ConfirmedAt: now,
	}, nil
}

// UnbanAgent 链上解封。
func (m *MockAdapter) UnbanAgent(ctx context.Context, uuid string) (*chain_adapter.Receipt, error) {
	if err := m.preflight("unban"); err != nil {
		return nil, err
	}
	if uuid == "" {
		return nil, &chain_adapter.ErrTxFailed{Reason: "empty uuid"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[uuid]
	if !ok {
		return nil, &chain_adapter.ErrAgentNotFoundOnchain{UUID: uuid}
	}
	if agent.State != chain_adapter.StateBanned {
		// 幂等
		return &chain_adapter.Receipt{
			TxHash:      agent.TxHash,
			BlockNumber: m.blockNo,
			GasUsed:     0,
			ConfirmedAt: time.Now(),
		}, nil
	}

	now := time.Now()
	m.blockNo++
	agent.State = chain_adapter.StateActive
	agent.UpdatedAt = now
	m.nonces[uuid]++
	txHash := m.nextTxHashLocked()

	return &chain_adapter.Receipt{
		TxHash:      txHash,
		BlockNumber: m.blockNo,
		GasUsed:     30000,
		ConfirmedAt: now,
	}, nil
}

// RevokeAgent 链上注销。
func (m *MockAdapter) RevokeAgent(ctx context.Context, uuid string) (*chain_adapter.Receipt, error) {
	if err := m.preflight("revoke"); err != nil {
		return nil, err
	}
	if uuid == "" {
		return nil, &chain_adapter.ErrTxFailed{Reason: "empty uuid"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[uuid]
	if !ok {
		return nil, &chain_adapter.ErrAgentNotFoundOnchain{UUID: uuid}
	}
	if agent.State == chain_adapter.StateRevoked {
		// 幂等
		return &chain_adapter.Receipt{
			TxHash:      agent.TxHash,
			BlockNumber: m.blockNo,
			GasUsed:     0,
			ConfirmedAt: time.Now(),
		}, nil
	}

	now := time.Now()
	m.blockNo++
	agent.State = chain_adapter.StateRevoked
	agent.UpdatedAt = now
	m.nonces[uuid]++
	txHash := m.nextTxHashLocked()

	return &chain_adapter.Receipt{
		TxHash:      txHash,
		BlockNumber: m.blockNo,
		GasUsed:     30000,
		ConfirmedAt: now,
	}, nil
}

// GetAgentState 查询链上状态。
func (m *MockAdapter) GetAgentState(ctx context.Context, uuid string) (*chain_adapter.AgentOnchain, error) {
	if err := m.preflight("get_state"); err != nil {
		return nil, err
	}
	if uuid == "" {
		return nil, &chain_adapter.ErrTxFailed{Reason: "empty uuid"}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	agent, ok := m.agents[uuid]
	if !ok {
		return nil, &chain_adapter.ErrAgentNotFoundOnchain{UUID: uuid}
	}
	cp := *agent
	return &cp, nil
}

// HealthCheck 健康检查（始终 OK；除非注入错误）。
func (m *MockAdapter) HealthCheck(ctx context.Context) error {
	if m.closed.Load() {
		return &chain_adapter.ErrChainUnavailable{Reason: "closed"}
	}
	m.mu.RLock()
	fn := m.errFn
	m.mu.RUnlock()
	if fn != nil {
		if err := fn("health"); err != nil {
			return &chain_adapter.ErrChainUnavailable{Reason: err.Error()}
		}
	}
	if m.latency > 0 {
		select {
		case <-time.After(m.latency):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Close 关闭（之后 HealthCheck 返回 unavailable）。
func (m *MockAdapter) Close() error {
	m.closed.Store(true)
	return nil
}

// =============================================================================
// 测试 / 对账辅助
// =============================================================================

// ListAll 列出所有链上 agent（对账用）。
func (m *MockAdapter) ListAll() []*chain_adapter.AgentOnchain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*chain_adapter.AgentOnchain, 0, len(m.agents))
	for _, a := range m.agents {
		cp := *a
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UUID < out[j].UUID })
	return out
}

// Count 当前链上 agent 总数。
func (m *MockAdapter) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agents)
}

// Dump 导出全量状态（用于测试 snapshot）。
func (m *MockAdapter) Dump() map[string]*chain_adapter.AgentOnchain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[string]*chain_adapter.AgentOnchain{}
	for k, v := range m.agents {
		cp := *v
		out[k] = &cp
	}
	return out
}

// Restore 从 snapshot 恢复（测试 setup 用）。
func (m *MockAdapter) Restore(snap map[string]*chain_adapter.AgentOnchain) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents = map[string]*chain_adapter.AgentOnchain{}
	for k, v := range snap {
		cp := *v
		m.agents[k] = &cp
	}
}

// preflight 通用前置：ctx 取消 / 关闭检查 / 错误注入 / 延迟。
func (m *MockAdapter) preflight(op string) error {
	if m.closed.Load() {
		return &chain_adapter.ErrChainUnavailable{Reason: "closed"}
	}
	m.mu.RLock()
	fn := m.errFn
	lat := m.latency
	m.mu.RUnlock()
	if fn != nil {
		if err := fn(op); err != nil {
			return &chain_adapter.ErrTxFailed{Reason: err.Error()}
		}
	}
	if lat > 0 {
		time.Sleep(lat)
	}
	return nil
}

// nextTxHashLocked 生成伪 tx hash（32 字节 hex）。调用方需持锁。
func (m *MockAdapter) nextTxHashLocked() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return "0x" + hex.EncodeToString(b[:])
}

// 编译期检查
var _ chain_adapter.BaseChainAdapter = (*MockAdapter)(nil)
