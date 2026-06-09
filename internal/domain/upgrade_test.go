package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newActiveAgent(t *testing.T) *Agent {
	t.Helper()
	a, err := NewAgent("01234567-89ab-cdef-0123-456789abcdef", "test-owner", LevelBasic, "pk", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Activate(time.Now()); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestUpgradeAgent_HappyPath(t *testing.T) {
	now := time.Now()
	a := newActiveAgent(t)
	in := UpgradeInput{
		Agent:        a,
		NewLevel:     LevelAdvanced,
		NewPerms:     0x02,
		OperatorDID:  "did:agentid:user:admin",
		AgentSelfDID: "did:agentid:agent:" + string(a.UUID),
		Now:          now,
		Reason:       "promote to advanced",
	}
	out, err := UpgradeAgent(in)
	if err != nil {
		t.Fatalf("UpgradeAgent: %v", err)
	}
	if out.Agent.Level != LevelAdvanced {
		t.Errorf("Level = %s, want %s", out.Agent.Level, LevelAdvanced)
	}
	if out.Agent.Permissions != 0x02 {
		t.Errorf("Permissions = %#x, want 0x02", out.Agent.Permissions)
	}
	if out.Agent.LastUpgradeBy != "did:agentid:user:admin" {
		t.Errorf("LastUpgradeBy = %s", out.Agent.LastUpgradeBy)
	}
	if out.Event.EventType != EventAgentUpgradedV1 {
		t.Errorf("EventType = %s", out.Event.EventType)
	}
}

func TestUpgradeAgent_SkipLevel(t *testing.T) {
	a := newActiveAgent(t)
	_, err := UpgradeAgent(UpgradeInput{
		Agent:        a,
		NewLevel:     LevelPro, // 跳 2 级
		OperatorDID:  "did:agentid:user:admin",
		AgentSelfDID: "did:agentid:agent:" + string(a.UUID),
		Now:          time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidLevel) {
		t.Errorf("err should wrap ErrInvalidLevel: %v", err)
	}
}

func TestUpgradeAgent_Downgrade(t *testing.T) {
	a := newActiveAgent(t)
	_, err := UpgradeAgent(UpgradeInput{
		Agent:        a,
		NewLevel:     LevelTest, // 降级
		OperatorDID:  "did:agentid:user:admin",
		AgentSelfDID: "did:agentid:agent:" + string(a.UUID),
		Now:          time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpgradeAgent_SelfUpgrade(t *testing.T) {
	a := newActiveAgent(t)
	selfDID := "did:agentid:agent:" + string(a.UUID)
	_, err := UpgradeAgent(UpgradeInput{
		Agent:        a,
		NewLevel:     LevelAdvanced,
		OperatorDID:  selfDID, // 自升级
		AgentSelfDID: selfDID,
		Now:          time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrSelfUpgrade) {
		t.Errorf("err should wrap ErrSelfUpgrade: %v", err)
	}
}

func TestUpgradeAgent_BannedAgent(t *testing.T) {
	a := newActiveAgent(t)
	_ = a.Ban("violation", time.Now())
	_, err := UpgradeAgent(UpgradeInput{
		Agent:        a,
		NewLevel:     LevelAdvanced,
		OperatorDID:  "did:agentid:user:admin",
		AgentSelfDID: "did:agentid:agent:" + string(a.UUID),
		Now:          time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUpgradeInvalidState) {
		t.Errorf("err should wrap ErrUpgradeInvalidState: %v", err)
	}
}

func TestUpgradeAgent_NilAgent(t *testing.T) {
	_, err := UpgradeAgent(UpgradeInput{OperatorDID: "did", Now: time.Now()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpgradeAgent_EmptyOperator(t *testing.T) {
	a := newActiveAgent(t)
	_, err := UpgradeAgent(UpgradeInput{
		Agent: a, NewLevel: LevelAdvanced, Now: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpgradeAgent_ZeroNow(t *testing.T) {
	a := newActiveAgent(t)
	_, err := UpgradeAgent(UpgradeInput{
		Agent: a, NewLevel: LevelAdvanced, OperatorDID: "did",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpgradeAgent_PermissionsExceedLevel(t *testing.T) {
	a := newActiveAgent(t)
	// NewPerms 包含 LevelAdvanced 不允许的位
	bad := LevelAdvanced.DefaultMaxPermissions() | (uint64(1) << 50)
	_, err := UpgradeAgent(UpgradeInput{
		Agent:        a,
		NewLevel:     LevelAdvanced,
		NewPerms:     bad,
		OperatorDID:  "did:agentid:user:admin",
		AgentSelfDID: "did:agentid:agent:" + string(a.UUID),
		Now:          time.Now(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrPermissionExceedsLevel) {
		t.Errorf("err should wrap ErrPermissionExceedsLevel: %v", err)
	}
}

func TestUpgradeAgent_PermissionsMerged(t *testing.T) {
	a := newActiveAgent(t)
	// 已有权限位 1（来自 LevelBasic），新增 2（LevelAdvanced 允许）
	if err := a.Grant(0); err != nil {
		t.Fatal(err)
	}
	in := UpgradeInput{
		Agent:        a,
		NewLevel:     LevelAdvanced,
		NewPerms:     uint64(1) << 1, // bit 1
		OperatorDID:  "did:agentid:user:admin",
		AgentSelfDID: "did:agentid:agent:" + string(a.UUID),
		Now:          time.Now(),
	}
	out, err := UpgradeAgent(in)
	if err != nil {
		t.Fatal(err)
	}
	// bit 0 + bit 1 = 0x03
	if out.Agent.Permissions != 0x03 {
		t.Errorf("merged = %#x, want 0x03", out.Agent.Permissions)
	}
}

func TestCollectUpgradeOutbox(t *testing.T) {
	a := newActiveAgent(t)
	out, err := UpgradeAgent(UpgradeInput{
		Agent:        a,
		NewLevel:     LevelAdvanced,
		OperatorDID:  "did:agentid:user:admin",
		AgentSelfDID: "did:agentid:agent:" + string(a.UUID),
		Now:          time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	w := &stubOutboxWriter{}
	if err := CollectUpgradeOutbox(context.Background(), w, out); err != nil {
		t.Fatal(err)
	}
	if len(w.envelopes) != 1 {
		t.Errorf("got %d envelopes, want 1", len(w.envelopes))
	}
}

func TestCollectUpgradeOutbox_NilOutput(t *testing.T) {
	if err := CollectUpgradeOutbox(context.Background(), &stubOutboxWriter{}, nil); err == nil {
		t.Error("expected error")
	}
}

// =============================================================================
// 状态机全覆盖测试
// =============================================================================

func TestAgent_Activate_InvalidState(t *testing.T) {
	a := newActiveAgent(t) // 已 active
	err := a.Activate(time.Now())
	if err == nil {
		t.Fatal("expected error: already active")
	}
}

func TestAgent_Ban_InvalidState(t *testing.T) {
	a, _ := NewAgent("01234567-89ab-cdef-0123-456789abcdef", "test-owner", LevelBasic, "pk", time.Now())
	_ = a.Activate(time.Now())
	_ = a.Ban("r", time.Now())
	err := a.Ban("r2", time.Now())
	if err == nil {
		t.Fatal("expected error: already banned")
	}
}

func TestAgent_Ban_EmptyReason(t *testing.T) {
	a := newActiveAgent(t)
	err := a.Ban("", time.Now())
	if err == nil {
		t.Fatal("expected error: empty reason")
	}
}

func TestAgent_Unban_NotBanned(t *testing.T) {
	a := newActiveAgent(t)
	err := a.Unban(time.Now())
	if err == nil {
		t.Fatal("expected error: not banned")
	}
}

func TestAgent_Upgrade_InvalidLevel(t *testing.T) {
	a := newActiveAgent(t)
	err := a.Upgrade(LevelType(99), time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAgent_Grant_BitOutOfRange(t *testing.T) {
	a := newActiveAgent(t)
	if err := a.Grant(64); err == nil {
		t.Fatal("expected error: bit out of range")
	}
}

func TestAgent_HasPermission(t *testing.T) {
	a := newActiveAgent(t)
	if err := a.Grant(0); err != nil {
		t.Fatal(err)
	}
	if !a.HasPermission(0) {
		t.Error("bit 0 should be set")
	}
	if a.HasPermission(1) {
		t.Error("bit 1 should not be set")
	}
	if a.HasPermission(64) {
		t.Error("bit 64 should not be set")
	}
}

func TestAgent_IsActive(t *testing.T) {
	a := newActiveAgent(t)
	if !a.IsActive(time.Now()) {
		t.Error("should be active")
	}
	_ = a.Ban("r", time.Now())
	if a.IsActive(time.Now()) {
		t.Error("banned agent should not be active")
	}
}
