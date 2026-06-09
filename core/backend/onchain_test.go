package backend

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
)

// =============================================================================
// helpers
// =============================================================================

func newOnchainTestBackend(t *testing.T) (*OnchainBackend, *mock.MockAdapter) {
	t.Helper()
	adp := mock.NewMockAdapter()
	be, err := NewOnchainBackend(adp, nil, OnchainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	return be, adp
}

// failingAdapter 注入错误的 chain adapter。
type failingAdapter struct {
	opErr error
}

func (f *failingAdapter) ChainType() chain_adapter.ChainType { return chain_adapter.ChainTypeMock }
func (f *failingAdapter) RegisterAgent(context.Context, *chain_adapter.RegisterRequest) (*chain_adapter.Receipt, error) {
	return nil, f.opErr
}
func (f *failingAdapter) UpdateLevel(context.Context, *chain_adapter.UpdateLevelRequest) (*chain_adapter.Receipt, error) {
	return nil, f.opErr
}
func (f *failingAdapter) BanAgent(context.Context, *chain_adapter.BanRequest) (*chain_adapter.Receipt, error) {
	return nil, f.opErr
}
func (f *failingAdapter) UnbanAgent(context.Context, string) (*chain_adapter.Receipt, error) {
	return nil, f.opErr
}
func (f *failingAdapter) RevokeAgent(context.Context, string) (*chain_adapter.Receipt, error) {
	return nil, f.opErr
}
func (f *failingAdapter) GetAgentState(context.Context, string) (*chain_adapter.AgentOnchain, error) {
	return nil, f.opErr
}
func (f *failingAdapter) HealthCheck(context.Context) error { return f.opErr }

// =============================================================================
// BackendType
// =============================================================================

func TestOnchainBackend_BackendType(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	if got := be.BackendType(); got != TypeOnchain {
		t.Errorf("type = %q, want onchain", got)
	}
}

func TestOnchainBackend_Close(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	if err := be.Close(context.Background()); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
}

func TestNewOnchainBackend_NilAdapter(t *testing.T) {
	_, err := NewOnchainBackend(nil, nil, OnchainConfig{})
	if err == nil {
		t.Error("expected error for nil adapter")
	}
}

// =============================================================================
// RegisterAgent
// =============================================================================

func TestOnchainBackend_RegisterAgent_HappyPath(t *testing.T) {
	be, adp := newOnchainTestBackend(t)
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
	if cred.TxHash == "" {
		t.Error("empty tx hash")
	}
	if cred.State != StateActive {
		t.Errorf("state = %q, want active", cred.State)
	}
	if adp.Count() != 1 {
		t.Errorf("chain count = %d, want 1", adp.Count())
	}
}

func TestOnchainBackend_RegisterAgent_BadInput(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	tests := []struct {
		name string
		req  *RegisterRequest
	}{
		{"nil", nil},
		{"empty owner", &RegisterRequest{Level: 1, PublicKey: "x"}},
		{"zero level", &RegisterRequest{Owner: "x", PublicKey: "x"}},
		{"empty pubkey", &RegisterRequest{Owner: "x", Level: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := be.RegisterAgent(context.Background(), tt.req)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestOnchainBackend_RegisterAgent_AdapterError(t *testing.T) {
	adp := &failingAdapter{opErr: &chain_adapter.ErrTxFailed{Reason: "rpc timeout"}}
	be, _ := NewOnchainBackend(adp, nil, OnchainConfig{})
	_, err := be.RegisterAgent(context.Background(), validRequest())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, &chain_adapter.ErrTxFailed{}) {
		// fallback：检查错误文本
		if got := err.Error(); !contains(got, "rpc timeout") {
			t.Errorf("err = %v, want to contain 'rpc timeout'", err)
		}
	}
}

func TestOnchainBackend_RegisterAgent_PropagatesUUID(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	info, err := be.GetAgentInfo(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != cred.UUID {
		t.Errorf("uuid = %q, want %q", info.UUID, cred.UUID)
	}
}

// =============================================================================
// GetAgentInfo
// =============================================================================

func TestOnchainBackend_GetAgentInfo_HappyPath(t *testing.T) {
	be, adp := newOnchainTestBackend(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	info, err := be.GetAgentInfo(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != cred.UUID {
		t.Errorf("uuid = %q", info.UUID)
	}
	if info.State != StateActive {
		t.Errorf("state = %q, want active", info.State)
	}
	if info.Level != cred.Level {
		t.Errorf("level = %d, want %d", info.Level, cred.Level)
	}
	// 链上只有 1 个 agent
	if adp.Count() != 1 {
		t.Errorf("chain count = %d", adp.Count())
	}
}

func TestOnchainBackend_GetAgentInfo_NotFound(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	_, err := be.GetAgentInfo(context.Background(), "missing")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestOnchainBackend_GetAgentInfo_EmptyUUID(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	_, err := be.GetAgentInfo(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty uuid")
	}
}

func TestOnchainBackend_GetAgentInfo_Banned(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	if err := be.BanAgent(context.Background(), cred.UUID, "test"); err != nil {
		t.Fatal(err)
	}
	info, err := be.GetAgentInfo(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.State != StateBanned {
		t.Errorf("state = %q, want banned", info.State)
	}
}

// =============================================================================
// UpdateAgentLevel
// =============================================================================

func TestOnchainBackend_UpdateAgentLevel(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	if err := be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "upgrade"); err != nil {
		t.Fatal(err)
	}
	info, _ := be.GetAgentInfo(context.Background(), cred.UUID)
	if info.Level != 2 {
		t.Errorf("level = %d, want 2", info.Level)
	}
}

func TestOnchainBackend_UpdateAgentLevel_EmptyUUID(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	if err := be.UpdateAgentLevel(context.Background(), "", 2, "x"); err == nil {
		t.Error("expected error for empty uuid")
	}
}

func TestOnchainBackend_UpdateAgentLevel_ZeroLevel(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	if err := be.UpdateAgentLevel(context.Background(), cred.UUID, 0, "x"); err == nil {
		t.Error("expected error for zero level")
	}
}

func TestOnchainBackend_UpdateAgentLevel_AdapterError(t *testing.T) {
	adp := mock.NewMockAdapter()
	be, _ := NewOnchainBackend(adp, nil, OnchainConfig{})
	cred, _ := be.RegisterAgent(context.Background(), validRequest())

	adp.InjectError(func(op string) error {
		return errors.New("rpc fail")
	})
	if err := be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "x"); err == nil {
		t.Error("expected error from injected failure")
	}
}

// =============================================================================
// Ban / Unban
// =============================================================================

func TestOnchainBackend_BanUnban(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())

	if err := be.BanAgent(context.Background(), cred.UUID, "policy"); err != nil {
		t.Fatal(err)
	}
	info, _ := be.GetAgentInfo(context.Background(), cred.UUID)
	if info.State != StateBanned {
		t.Errorf("state = %q, want banned", info.State)
	}
	if err := be.BanAgent(context.Background(), cred.UUID, "again"); err != nil {
		t.Error("re-ban should be idempotent")
	}
	if err := be.UnbanAgent(context.Background(), cred.UUID); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(context.Background(), cred.UUID)
	if info.State != StateActive {
		t.Errorf("state = %q, want active", info.State)
	}
}

func TestOnchainBackend_Ban_EmptyUUID(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	if err := be.BanAgent(context.Background(), "", "x"); err == nil {
		t.Error("expected error for empty uuid")
	}
}

// =============================================================================
// Unregister
// =============================================================================

func TestOnchainBackend_UnregisterAgent(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	if err := be.UnregisterAgent(context.Background(), cred.UUID); err != nil {
		t.Fatal(err)
	}
	info, _ := be.GetAgentInfo(context.Background(), cred.UUID)
	if info.State != StateUnregistered {
		t.Errorf("state = %q, want unregistered", info.State)
	}
}

func TestOnchainBackend_Unregister_EmptyUUID(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	if err := be.UnregisterAgent(context.Background(), ""); err == nil {
		t.Error("expected error for empty uuid")
	}
}

// =============================================================================
// GetChangeLogs
// =============================================================================

func TestOnchainBackend_GetChangeLogs(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	logs, err := be.GetChangeLogs(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 {
		t.Error("expected at least one log entry")
	}
	if logs[0].Action != "info" {
		t.Errorf("first log action = %q, want 'info'", logs[0].Action)
	}
}

func TestOnchainBackend_GetChangeLogs_EmptyUUID(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	if _, err := be.GetChangeLogs(context.Background(), ""); err == nil {
		t.Error("expected error for empty uuid")
	}
}

// =============================================================================
// BatchGetAgentInfo
// =============================================================================

func TestOnchainBackend_BatchGetAgentInfo(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	c1, _ := be.RegisterAgent(context.Background(), validRequest())
	c2, _ := be.RegisterAgent(context.Background(), &RegisterRequest{
		Owner: "test-owner-2", Level: 1, PublicKey: "pk2",
	})
	infos, err := be.BatchGetAgentInfo(context.Background(), []string{c1.UUID, c2.UUID, "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Errorf("got %d, want 2", len(infos))
	}
}

func TestOnchainBackend_BatchGetAgentInfo_Empty(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	infos, err := be.BatchGetAgentInfo(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Errorf("len = %d, want 0", len(infos))
	}
}

// =============================================================================
// 并发 / 缓存
// =============================================================================

func TestOnchainBackend_ConcurrentRegister(t *testing.T) {
	be, _ := newOnchainTestBackend(t)
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	uuids := make([]string, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			cred, err := be.RegisterAgent(context.Background(), &RegisterRequest{
				Owner: "o", Level: 1, PublicKey: "pk",
			})
			if err == nil {
				uuids[i] = cred.UUID
			}
		}(i)
	}
	wg.Wait()
	distinct := map[string]struct{}{}
	for _, u := range uuids {
		if u != "" {
			distinct[u] = struct{}{}
		}
	}
	if len(distinct) == 0 {
		t.Error("expected at least one successful register")
	}
}

// =============================================================================
// 工具
// =============================================================================

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
