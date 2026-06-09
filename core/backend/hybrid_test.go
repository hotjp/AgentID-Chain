package backend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
)

// =============================================================================
// helpers
// =============================================================================

func newHybridTestBackend(t *testing.T) (*HybridBackend, *mock.MockAdapter) {
	t.Helper()
	adp := mock.NewMockAdapter()
	pers := NewMemoryPersistence()
	be, err := NewHybridBackend(adp, pers, nil, HybridConfig{
		SyncInterval:   50 * time.Millisecond, // 测试用：50ms
		SyncBatchSize:  10,
		CacheTTL:       time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return be, adp
}

func newHybridNoSync(t *testing.T) (*HybridBackend, *mock.MockAdapter) {
	t.Helper()
	adp := mock.NewMockAdapter()
	pers := NewMemoryPersistence()
	be, err := NewHybridBackend(adp, pers, nil, HybridConfig{
		SyncInterval:  0, // 禁用
		SyncBatchSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return be, adp
}

// =============================================================================
// BackendType
// =============================================================================

func TestHybridBackend_BackendType(t *testing.T) {
	be, _ := newHybridTestBackend(t)
	if got := be.BackendType(); got != TypeHybrid {
		t.Errorf("type = %q, want hybrid", got)
	}
}

func TestNewHybridBackend_NilAdapter(t *testing.T) {
	pers := NewMemoryPersistence()
	_, err := NewHybridBackend(nil, pers, nil, HybridConfig{})
	if err == nil {
		t.Error("expected error for nil adapter")
	}
}

func TestNewHybridBackend_NilPersistence(t *testing.T) {
	adp := mock.NewMockAdapter()
	_, err := NewHybridBackend(adp, nil, nil, HybridConfig{})
	if err == nil {
		t.Error("expected error for nil persistence")
	}
}

// =============================================================================
// Register
// =============================================================================

func TestHybridBackend_RegisterAgent(t *testing.T) {
	be, adp := newHybridNoSync(t)
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
	// 链上
	if adp.Count() != 1 {
		t.Errorf("chain count = %d, want 1", adp.Count())
	}
	// 本地
	if be.local.pers.Count() != 1 {
		t.Errorf("local count = %d, want 1", be.local.pers.Count())
	}
}

func TestHybridBackend_RegisterAgent_ChainError(t *testing.T) {
	be, adp := newHybridNoSync(t)
	adp.InjectError(func(op string) error {
		return errors.New("chain fail")
	})
	_, err := be.RegisterAgent(context.Background(), validRequest())
	if err == nil {
		t.Error("expected error from chain")
	}
	// 本地应未写入
	if be.local.pers.Count() != 0 {
		t.Errorf("local count = %d, want 0 (链上失败时不应写本地)", be.local.pers.Count())
	}
}

// =============================================================================
// Read
// =============================================================================

func TestHybridBackend_GetAgentInfo_LocalHit(t *testing.T) {
	be, _ := newHybridNoSync(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())

	// 直接调本地应命中
	info, err := be.GetAgentInfo(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != cred.UUID {
		t.Errorf("uuid = %q", info.UUID)
	}
}

func TestHybridBackend_GetAgentInfo_LocalMiss_ChainFallback(t *testing.T) {
	be, adp := newHybridNoSync(t)
	// 链上直接注册（绕过 hybrid，本地无记录）
	r, err := adp.RegisterAgent(context.Background(), &chain_adapter.RegisterRequest{
		UUID: "chain-only", Owner: "alice", Level: 1, Permission: 0xFF, PublicKey: "pk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("nil receipt")
	}

	// 读：本地无 → 回源链上
	info, err := be.GetAgentInfo(context.Background(), "chain-only")
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != "chain-only" {
		t.Errorf("uuid = %q", info.UUID)
	}
	// 回填后本地应有 1 个
	if be.local.pers.Count() != 1 {
		t.Errorf("local count = %d, want 1 (after fallback)", be.local.pers.Count())
	}
}

func TestHybridBackend_GetAgentInfo_NotFound(t *testing.T) {
	be, _ := newHybridNoSync(t)
	_, err := be.GetAgentInfo(context.Background(), "missing")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestHybridBackend_GetAgentInfo_EmptyUUID(t *testing.T) {
	be, _ := newHybridNoSync(t)
	_, err := be.GetAgentInfo(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty uuid")
	}
}

// =============================================================================
// Write 链上 → 本地镜像
// =============================================================================

func TestHybridBackend_UpdateAgentLevel(t *testing.T) {
	be, _ := newHybridNoSync(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	if err := be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "x"); err != nil {
		t.Fatal(err)
	}
	// 本地 + 链上均应反映
	local, _ := be.local.GetAgentInfo(context.Background(), cred.UUID)
	if local.Level != 2 {
		t.Errorf("local level = %d, want 2", local.Level)
	}
	chain, _ := be.chain.GetAgentInfo(context.Background(), cred.UUID)
	if chain.Level != 2 {
		t.Errorf("chain level = %d, want 2", chain.Level)
	}
}

func TestHybridBackend_BanAgent(t *testing.T) {
	be, _ := newHybridNoSync(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	if err := be.BanAgent(context.Background(), cred.UUID, "x"); err != nil {
		t.Fatal(err)
	}
	local, _ := be.local.GetAgentInfo(context.Background(), cred.UUID)
	if local.State != StateBanned {
		t.Errorf("local state = %q, want banned", local.State)
	}
}

func TestHybridBackend_UnbanAgent(t *testing.T) {
	be, _ := newHybridNoSync(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	_ = be.BanAgent(context.Background(), cred.UUID, "x")
	if err := be.UnbanAgent(context.Background(), cred.UUID); err != nil {
		t.Fatal(err)
	}
	local, _ := be.local.GetAgentInfo(context.Background(), cred.UUID)
	if local.State != StateActive {
		t.Errorf("local state = %q, want active", local.State)
	}
}

func TestHybridBackend_UnregisterAgent(t *testing.T) {
	be, _ := newHybridNoSync(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	if err := be.UnregisterAgent(context.Background(), cred.UUID); err != nil {
		t.Fatal(err)
	}
	local, _ := be.local.GetAgentInfo(context.Background(), cred.UUID)
	if local.State != StateUnregistered {
		t.Errorf("local state = %q, want unregistered", local.State)
	}
}

// =============================================================================
// GetChangeLogs
// =============================================================================

func TestHybridBackend_GetChangeLogs(t *testing.T) {
	be, _ := newHybridNoSync(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	_ = be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "x")
	logs, err := be.GetChangeLogs(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) < 2 {
		t.Errorf("logs = %d, want >= 2", len(logs))
	}
}

// =============================================================================
// Batch
// =============================================================================

func TestHybridBackend_BatchGetAgentInfo(t *testing.T) {
	be, _ := newHybridNoSync(t)
	c1, _ := be.RegisterAgent(context.Background(), validRequest())
	c2, _ := be.RegisterAgent(context.Background(), &RegisterRequest{
		Owner: "test-owner-2", Level: 1, PublicKey: "pk2",
	})
	infos, err := be.BatchGetAgentInfo(context.Background(), []string{c1.UUID, c2.UUID})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Errorf("len = %d, want 2", len(infos))
	}
}

func TestHybridBackend_BatchGetAgentInfo_Empty(t *testing.T) {
	be, _ := newHybridNoSync(t)
	infos, err := be.BatchGetAgentInfo(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Errorf("len = %d, want 0", len(infos))
	}
}

func TestHybridBackend_BatchGetAgentInfo_FallbackToChain(t *testing.T) {
	be, adp := newHybridNoSync(t)
	// 链上 1 个
	if _, err := adp.RegisterAgent(context.Background(), &chain_adapter.RegisterRequest{
		UUID: "chain-only", Owner: "alice", Level: 1, Permission: 0xFF, PublicKey: "pk",
	}); err != nil {
		t.Fatal(err)
	}
	infos, err := be.BatchGetAgentInfo(context.Background(), []string{"chain-only"})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 {
		t.Errorf("len = %d, want 1 (chain fallback)", len(infos))
	}
}

// =============================================================================
// 后台同步
// =============================================================================

func TestHybridBackend_StartStop(t *testing.T) {
	be, _ := newHybridTestBackend(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := be.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// 重复 Start 幂等
	if err := be.Start(ctx); err != nil {
		t.Error("re-start should be no-op")
	}
	if err := be.Close(ctx); err != nil {
		t.Errorf("Close = %v", err)
	}
}

func TestHybridBackend_SyncNow(t *testing.T) {
	be, adp := newHybridNoSync(t)
	// 链上直接注册 1 个
	if _, err := adp.RegisterAgent(context.Background(), &chain_adapter.RegisterRequest{
		UUID: "chain-only", Owner: "alice", Level: 1, Permission: 0xFF, PublicKey: "pk",
	}); err != nil {
		t.Fatal(err)
	}
	// 手动同步
	if err := be.SyncNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if be.local.pers.Count() != 1 {
		t.Errorf("local count after sync = %d, want 1", be.local.pers.Count())
	}
}

func TestHybridBackend_SyncNow_SkipsNewerLocal(t *testing.T) {
	be, adp := newHybridNoSync(t)
	// 链上 1 个
	if _, err := adp.RegisterAgent(context.Background(), &chain_adapter.RegisterRequest{
		UUID: "u1", Owner: "alice", Level: 1, Permission: 0xFF, PublicKey: "pk",
	}); err != nil {
		t.Fatal(err)
	}
	// 第一次同步：本地从无 → 写入
	_ = be.SyncNow(context.Background())
	// 现在本地和链上同步。手动在本地写一个更新的记录
	_ = be.local.pers.PutAgent(context.Background(), &AgentInfo{
		UUID: "u1", Owner: "alice", Level: 5, State: StateActive,
		UpdatedAt: time.Now().Add(time.Hour), // 比链上新
	})
	// 再次同步：本地的 UpdatedAt 更新 → 不应被链上覆盖
	_ = be.SyncNow(context.Background())
	got, _ := be.local.GetAgentInfo(context.Background(), "u1")
	if got.Level != 5 {
		t.Errorf("level after sync = %d, want 5 (local was newer)", got.Level)
	}
}
