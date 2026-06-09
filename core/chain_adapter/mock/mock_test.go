package mock

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
)

func newReq(uuid, owner string) *chain_adapter.RegisterRequest {
	return &chain_adapter.RegisterRequest{
		UUID:       uuid,
		Owner:      owner,
		Level:      1,
		Permission: 0xFF,
		PublicKey:  "pk",
	}
}

func TestMockAdapter_ChainType(t *testing.T) {
	m := NewMockAdapter()
	if got := m.ChainType(); got != chain_adapter.ChainTypeMock {
		t.Errorf("ChainType = %q, want mock", got)
	}
}

func TestMockAdapter_RegisterAgent_HappyPath(t *testing.T) {
	m := NewMockAdapter()
	r, err := m.RegisterAgent(context.Background(), newReq("u1", "alice"))
	if err != nil {
		t.Fatal(err)
	}
	if r.TxHash == "" {
		t.Error("empty tx hash")
	}
	if r.BlockNumber == 0 {
		t.Error("block number should be > 0")
	}
	if m.Count() != 1 {
		t.Errorf("count = %d, want 1", m.Count())
	}
}

func TestMockAdapter_RegisterAgent_BadInput(t *testing.T) {
	m := NewMockAdapter()
	tests := []struct {
		name string
		req  *chain_adapter.RegisterRequest
	}{
		{"nil", nil},
		{"empty uuid", &chain_adapter.RegisterRequest{Owner: "x"}},
		{"empty owner", newReq("u1", "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.RegisterAgent(context.Background(), tt.req)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestMockAdapter_RegisterAgent_Duplicate(t *testing.T) {
	m := NewMockAdapter()
	ctx := context.Background()
	if _, err := m.RegisterAgent(ctx, newReq("u1", "alice")); err != nil {
		t.Fatal(err)
	}
	_, err := m.RegisterAgent(ctx, newReq("u1", "alice"))
	if err == nil {
		t.Error("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v, want 'already exists'", err)
	}
}

func TestMockAdapter_UpdateLevel(t *testing.T) {
	m := NewMockAdapter()
	ctx := context.Background()
	if _, err := m.RegisterAgent(ctx, newReq("u1", "alice")); err != nil {
		t.Fatal(err)
	}
	r, err := m.UpdateLevel(ctx, &chain_adapter.UpdateLevelRequest{UUID: "u1", NewLevel: 2, Reason: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if r.TxHash == "" {
		t.Error("empty tx hash")
	}
	state, _ := m.GetAgentState(ctx, "u1")
	if state.Level != 2 {
		t.Errorf("level = %d, want 2", state.Level)
	}
}

func TestMockAdapter_UpdateLevel_NotFound(t *testing.T) {
	m := NewMockAdapter()
	_, err := m.UpdateLevel(context.Background(), &chain_adapter.UpdateLevelRequest{UUID: "missing", NewLevel: 2})
	if err == nil {
		t.Error("expected not-found error")
	}
	var nfe *chain_adapter.ErrAgentNotFoundOnchain
	if !errors.As(err, &nfe) {
		t.Errorf("err type = %T, want ErrAgentNotFoundOnchain", err)
	}
}

func TestMockAdapter_BanUnban_RoundTrip(t *testing.T) {
	m := NewMockAdapter()
	ctx := context.Background()
	if _, err := m.RegisterAgent(ctx, newReq("u1", "alice")); err != nil {
		t.Fatal(err)
	}

	// Ban
	if _, err := m.BanAgent(ctx, &chain_adapter.BanRequest{UUID: "u1", Reason: "x"}); err != nil {
		t.Fatal(err)
	}
	state, _ := m.GetAgentState(ctx, "u1")
	if state.State != chain_adapter.StateBanned {
		t.Errorf("state = %q, want banned", state.State)
	}

	// Re-ban 幂等
	if _, err := m.BanAgent(ctx, &chain_adapter.BanRequest{UUID: "u1", Reason: "x"}); err != nil {
		t.Error("re-ban should be no-op")
	}

	// Unban
	if _, err := m.UnbanAgent(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	state, _ = m.GetAgentState(ctx, "u1")
	if state.State != chain_adapter.StateActive {
		t.Errorf("state = %q, want active", state.State)
	}
}

func TestMockAdapter_Ban_NotFound(t *testing.T) {
	m := NewMockAdapter()
	_, err := m.BanAgent(context.Background(), &chain_adapter.BanRequest{UUID: "missing"})
	if err == nil {
		t.Error("expected not-found")
	}
}

func TestMockAdapter_RevokeAgent(t *testing.T) {
	m := NewMockAdapter()
	ctx := context.Background()
	if _, err := m.RegisterAgent(ctx, newReq("u1", "alice")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RevokeAgent(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	state, _ := m.GetAgentState(ctx, "u1")
	if state.State != chain_adapter.StateRevoked {
		t.Errorf("state = %q, want revoked", state.State)
	}
	// UpdateLevel on revoked 应失败
	_, err := m.UpdateLevel(ctx, &chain_adapter.UpdateLevelRequest{UUID: "u1", NewLevel: 2})
	if err == nil {
		t.Error("expected error for revoked update")
	}
}

func TestMockAdapter_GetAgentState_NotFound(t *testing.T) {
	m := NewMockAdapter()
	_, err := m.GetAgentState(context.Background(), "missing")
	if err == nil {
		t.Error("expected not-found")
	}
}

func TestMockAdapter_HealthCheck(t *testing.T) {
	m := NewMockAdapter()
	if err := m.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck = %v, want nil", err)
	}
}

func TestMockAdapter_HealthCheck_WithErrorInjection(t *testing.T) {
	m := NewMockAdapter()
	m.InjectError(func(op string) error {
		return errors.New("rpc timeout")
	})
	err := m.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error from injection")
	}
	if !strings.Contains(err.Error(), "rpc timeout") {
		t.Errorf("err = %v, want 'rpc timeout'", err)
	}
	// 清除注入
	m.InjectError(nil)
	if err := m.HealthCheck(context.Background()); err != nil {
		t.Errorf("after clear: err = %v, want nil", err)
	}
}

func TestMockAdapter_HealthCheck_Latency(t *testing.T) {
	m := NewMockAdapter()
	m.SetLatency(50 * time.Millisecond)
	start := time.Now()
	if err := m.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 50ms", elapsed)
	}
}

func TestMockAdapter_HealthCheck_ContextCancel(t *testing.T) {
	m := NewMockAdapter()
	m.SetLatency(500 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.HealthCheck(ctx); err == nil {
		t.Error("expected ctx error")
	}
}

func TestMockAdapter_Close(t *testing.T) {
	m := NewMockAdapter()
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.HealthCheck(context.Background()); err == nil {
		t.Error("expected unavailable after close")
	}
}

func TestMockAdapter_ListAll_SortedByUUID(t *testing.T) {
	m := NewMockAdapter()
	ctx := context.Background()
	for _, u := range []string{"u3", "u1", "u2"} {
		if _, err := m.RegisterAgent(ctx, newReq(u, "alice")); err != nil {
			t.Fatal(err)
		}
	}
	all := m.ListAll()
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	if all[0].UUID != "u1" || all[1].UUID != "u2" || all[2].UUID != "u3" {
		t.Errorf("not sorted: %v", []string{all[0].UUID, all[1].UUID, all[2].UUID})
	}
}

func TestMockAdapter_DumpRestore(t *testing.T) {
	m := NewMockAdapter()
	ctx := context.Background()
	if _, err := m.RegisterAgent(ctx, newReq("u1", "alice")); err != nil {
		t.Fatal(err)
	}
	snap := m.Dump()

	m2 := NewMockAdapter()
	if m2.Count() != 0 {
		t.Errorf("new adapter count = %d, want 0", m2.Count())
	}
	m2.Restore(snap)
	if m2.Count() != 1 {
		t.Errorf("restored count = %d, want 1", m2.Count())
	}
	state, err := m2.GetAgentState(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Owner != "alice" {
		t.Errorf("owner = %q, want alice", state.Owner)
	}
}

func TestMockAdapter_ConcurrentRegister(t *testing.T) {
	m := NewMockAdapter()
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			uuid := string(rune('a'+(i%26))) + string(rune('0'+(i/26)))
			_, _ = m.RegisterAgent(context.Background(), newReq(uuid, "owner"))
		}(i)
	}
	wg.Wait()
	// 50 个 goroutine 中前 26 个 uuid 各重复 2 次（26*2=52 > 50，所以有 26 个 distinct）
	if m.Count() == 0 {
		t.Error("expected some agents")
	}
}

func TestMockAdapter_BlockIncrement(t *testing.T) {
	m := NewMockAdapter()
	ctx := context.Background()
	start := m.BlockNumber()
	if _, err := m.RegisterAgent(ctx, newReq("u1", "alice")); err != nil {
		t.Fatal(err)
	}
	if got := m.BlockNumber(); got != start+1 {
		t.Errorf("block = %d, want %d", got, start+1)
	}
	if _, err := m.BanAgent(ctx, &chain_adapter.BanRequest{UUID: "u1"}); err != nil {
		t.Fatal(err)
	}
	if got := m.BlockNumber(); got != start+2 {
		t.Errorf("block after ban = %d, want %d", got, start+2)
	}
}

func TestMockAdapter_NewWithConfig(t *testing.T) {
	m := NewWithConfig(424242, 100)
	if got := m.ChainID(); got != 424242 {
		t.Errorf("chainID = %d, want 424242", got)
	}
	if got := m.BlockNumber(); got != 100 {
		t.Errorf("blockNo = %d, want 100", got)
	}
}

func TestMockAdapter_InjectError_AllOps(t *testing.T) {
	m := NewMockAdapter()
	m.InjectError(func(op string) error {
		return errors.New("fail: " + op)
	})
	ctx := context.Background()
	if _, err := m.RegisterAgent(ctx, newReq("u1", "alice")); err == nil {
		t.Error("expected register error")
	}
	if _, err := m.GetAgentState(ctx, "u1"); err == nil {
		t.Error("expected get_state error")
	}
}
