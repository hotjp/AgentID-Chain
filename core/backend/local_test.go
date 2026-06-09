package backend

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestBackend() *LocalBackend {
	pers := NewMemoryPersistence()
	be, _ := NewLocalBackend(pers, nil, LocalConfig{})
	return be
}

func validRequest() *RegisterRequest {
	return &RegisterRequest{
		Owner:      "test-owner",
		Level:      1,
		Permission: 0xFF,
		PublicKey:  "fake-pubkey-bytes",
	}
}

func TestLocalBackend_RegisterAgent(t *testing.T) {
	be := newTestBackend()
	cred, err := be.RegisterAgent(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
	if cred.State != StateRegistered {
		t.Errorf("state = %q, want registered", cred.State)
	}
	if cred.Owner != "test-owner" {
		t.Errorf("owner = %q", cred.Owner)
	}
}

func TestLocalBackend_RegisterAgent_BadInput(t *testing.T) {
	be := newTestBackend()
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

func TestLocalBackend_GetAgentInfo_NotFound(t *testing.T) {
	be := newTestBackend()
	_, err := be.GetAgentInfo(context.Background(), "no-such-uuid")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestLocalBackend_GetAgentInfo_HappyPath(t *testing.T) {
	be := newTestBackend()
	cred, _ := be.RegisterAgent(context.Background(), validRequest())

	info, err := be.GetAgentInfo(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.UUID != cred.UUID {
		t.Errorf("uuid = %q", info.UUID)
	}
	if info.State != StateRegistered {
		t.Errorf("state = %q", info.State)
	}
}

func TestLocalBackend_UpdateAgentLevel(t *testing.T) {
	be := newTestBackend()
	cred, _ := be.RegisterAgent(context.Background(), &RegisterRequest{
		Owner: "x", Level: 1, PublicKey: "pk",
	})
	if err := be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "test upgrade"); err != nil {
		t.Fatal(err)
	}
	info, _ := be.GetAgentInfo(context.Background(), cred.UUID)
	if info.Level != 2 {
		t.Errorf("level = %d, want 2", info.Level)
	}
	// 重复升级到更高
	if err := be.UpdateAgentLevel(context.Background(), cred.UUID, 3, "test"); err != nil {
		t.Fatal(err)
	}
}

func TestLocalBackend_UpdateAgentLevel_Downgrade(t *testing.T) {
	be := newTestBackend()
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	err := be.UpdateAgentLevel(context.Background(), cred.UUID, 0, "downgrade")
	if err == nil {
		t.Error("expected downgrade error")
	}
}

func TestLocalBackend_UpdateAgentLevel_Banned(t *testing.T) {
	be := newTestBackend()
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	_ = be.BanAgent(context.Background(), cred.UUID, "test")
	err := be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "x")
	if err == nil {
		t.Error("expected error: cannot upgrade banned")
	}
}

func TestLocalBackend_BanUnban(t *testing.T) {
	be := newTestBackend()
	cred, _ := be.RegisterAgent(context.Background(), validRequest())

	// Ban
	if err := be.BanAgent(context.Background(), cred.UUID, "policy"); err != nil {
		t.Fatal(err)
	}
	info, _ := be.GetAgentInfo(context.Background(), cred.UUID)
	if info.State != StateBanned {
		t.Errorf("state = %q, want banned", info.State)
	}

	// 重复 ban（幂等）
	if err := be.BanAgent(context.Background(), cred.UUID, "x"); err != nil {
		t.Error("re-ban should be no-op")
	}

	// Unban
	if err := be.UnbanAgent(context.Background(), cred.UUID); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(context.Background(), cred.UUID)
	if info.State != StateActive {
		t.Errorf("state = %q, want active", info.State)
	}
}

func TestLocalBackend_UnregisterAgent(t *testing.T) {
	be := newTestBackend()
	cred, _ := be.RegisterAgent(context.Background(), validRequest())

	if err := be.UnregisterAgent(context.Background(), cred.UUID); err != nil {
		t.Fatal(err)
	}
	info, _ := be.GetAgentInfo(context.Background(), cred.UUID)
	if info.State != StateUnregistered {
		t.Errorf("state = %q, want unregistered", info.State)
	}
	// 幂等
	if err := be.UnregisterAgent(context.Background(), cred.UUID); err != nil {
		t.Error("re-unregister should be no-op")
	}
}

func TestLocalBackend_GetChangeLogs(t *testing.T) {
	be := newTestBackend()
	cred, _ := be.RegisterAgent(context.Background(), validRequest())
	_ = be.UpdateAgentLevel(context.Background(), cred.UUID, 2, "test")
	_ = be.BanAgent(context.Background(), cred.UUID, "test")
	_ = be.UnbanAgent(context.Background(), cred.UUID)

	logs, err := be.GetChangeLogs(context.Background(), cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 4 {
		t.Errorf("logs = %d, want 4 (register+upgrade+ban+unban)", len(logs))
	}
	// 倒序：最后发生的在最前
	if logs[0].Action != "unban" {
		t.Errorf("first log = %q, want unban", logs[0].Action)
	}
}

func TestLocalBackend_BatchGet(t *testing.T) {
	be := newTestBackend()
	c1, _ := be.RegisterAgent(context.Background(), validRequest())
	c2, _ := be.RegisterAgent(context.Background(), &RegisterRequest{
		Owner: "test-owner-2", Level: 1, PublicKey: "pk2",
	})
	infos, err := be.BatchGetAgentInfo(context.Background(), []string{c1.UUID, c2.UUID, "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Errorf("got %d, want 2 (missing skipped)", len(infos))
	}
}

func TestLocalBackend_BatchGet_Empty(t *testing.T) {
	be := newTestBackend()
	infos, err := be.BatchGetAgentInfo(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Errorf("len = %d, want 0", len(infos))
	}
}

func TestLocalBackend_Close(t *testing.T) {
	be := newTestBackend()
	if err := be.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLocalBackend_BackendType(t *testing.T) {
	be := newTestBackend()
	if got := be.BackendType(); got != TypeLocal {
		t.Errorf("type = %q, want local", got)
	}
}

func TestLocalBackend_NilPersistence(t *testing.T) {
	_, err := NewLocalBackend(nil, nil, LocalConfig{})
	if err == nil {
		t.Error("expected error for nil persistence")
	}
}

func TestLocalBackend_GetAgentInfo_EmptyUUID(t *testing.T) {
	be := newTestBackend()
	_, err := be.GetAgentInfo(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty uuid")
	}
}

// =============================================================================
// MemoryPersistence 单独测试
// =============================================================================

func TestMemoryPersistence_Basic(t *testing.T) {
	pers := NewMemoryPersistence()
	rec := &AgentInfo{UUID: "u1", Owner: "alice", Level: 1, State: StateRegistered}
	if err := pers.PutAgent(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	got, err := pers.GetAgent(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Owner != "alice" {
		t.Errorf("owner = %q", got.Owner)
	}
	if pers.Count() != 1 {
		t.Errorf("count = %d", pers.Count())
	}
}

func TestMemoryPersistence_ListByOwner(t *testing.T) {
	pers := NewMemoryPersistence()
	for i := 0; i < 3; i++ {
		_ = pers.PutAgent(context.Background(), &AgentInfo{
			UUID: "u" + string(rune('0'+i)), Owner: "alice", Level: 1, State: StateRegistered,
		})
	}
	agents, _ := pers.ListAgentsByOwner(context.Background(), "alice")
	if len(agents) != 3 {
		t.Errorf("count = %d", len(agents))
	}
	agents, _ = pers.ListAgentsByOwner(context.Background(), "bob")
	if len(agents) != 0 {
		t.Errorf("bob count = %d, want 0", len(agents))
	}
}

func TestMemoryPersistence_Logs(t *testing.T) {
	pers := NewMemoryPersistence()
	for i := 0; i < 5; i++ {
		_ = pers.AppendLog(context.Background(), &ChangeLog{
			UUID: "u1", Action: "test", OccurredAt: time.Now(),
		})
	}
	logs, _ := pers.ListLogs(context.Background(), "u1", 0)
	if len(logs) != 5 {
		t.Errorf("logs = %d", len(logs))
	}
	logs, _ = pers.ListLogs(context.Background(), "u1", 3)
	if len(logs) != 3 {
		t.Errorf("limit logs = %d, want 3", len(logs))
	}
}
