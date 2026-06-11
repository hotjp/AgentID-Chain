package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newAgentForTest(t *testing.T) *Agent {
	t.Helper()
	a, err := NewAgent("01234567-89ab-cdef-0123-456789abcdef", "test-owner", LevelBasic, "pubkey", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestStatusLegalInvariant(t *testing.T) {
	a := newAgentForTest(t)
	v := StatusLegalInvariant{}.Check(a, time.Now())
	if v != nil {
		t.Errorf("expected nil, got %+v", v)
	}

	a.State = AgentState("invalid")
	v = StatusLegalInvariant{}.Check(a, time.Now())
	if v == nil {
		t.Fatal("expected violation")
	}
	if v.Invariant != "status_legal" {
		t.Errorf("violation name = %s", v.Invariant)
	}
}

func TestNoSelfUpgradeInvariant_NoTrigger(t *testing.T) {
	a := newAgentForTest(t)
	a.LastUpgradeBy = "did:agentid:user:admin"
	selfDID := func(*Agent) string { return "did:agentid:agent:" + string(a.UUID) }
	v := NoSelfUpgradeInvariant{AgentSelfDID: selfDID}.Check(a, time.Now())
	if v != nil {
		t.Errorf("expected nil, got %+v", v)
	}
}

func TestNoSelfUpgradeInvariant_SelfUpgrade(t *testing.T) {
	a := newAgentForTest(t)
	selfDID := "did:agentid:agent:" + string(a.UUID)
	a.LastUpgradeBy = selfDID
	v := NoSelfUpgradeInvariant{AgentSelfDID: func(*Agent) string { return selfDID }}.Check(a, time.Now())
	if v == nil {
		t.Error("expected violation")
	}
}

func TestTempPermissionExpireInvariant(t *testing.T) {
	a := newAgentForTest(t)
	now := time.Now()

	// 无 TempPermissions：满足
	v := TemporaryPermissionExpireInvariant{}.Check(a, now)
	if v != nil {
		t.Errorf("nil temp perms: %+v", v)
	}

	// 临时权限未过期
	a.TempPermissions = map[uint]time.Time{0: now.Add(time.Hour)}
	a.Permissions = 0x01
	v = TemporaryPermissionExpireInvariant{}.Check(a, now)
	if v != nil {
		t.Errorf("future expiry: %+v", v)
	}

	// 临时权限已过期 + 位仍设置：违反
	a.TempPermissions = map[uint]time.Time{0: now.Add(-time.Hour)}
	v = TemporaryPermissionExpireInvariant{}.Check(a, now)
	if v == nil {
		t.Error("expired + bit set: expected violation")
	}

	// 已过期 + 位已清零：满足
	a.Permissions = 0
	v = TemporaryPermissionExpireInvariant{}.Check(a, now)
	if v != nil {
		t.Errorf("expired + bit cleared: %+v", v)
	}
}

func TestPermissionWithinLevelInvariant(t *testing.T) {
	a := newAgentForTest(t)
	a.Level = LevelBasic
	a.Permissions = 0x01
	v := PermissionWithinLevelInvariant{}.Check(a, time.Now())
	if v != nil {
		t.Errorf("basic perm: %+v", v)
	}

	a.Permissions = ^a.Level.DefaultMaxPermissions() // 所有不允许的位
	v = PermissionWithinLevelInvariant{}.Check(a, time.Now())
	if v == nil {
		t.Error("over-permission: expected violation")
	}
}

func TestExpiresAtAfterRegisteredAtInvariant(t *testing.T) {
	a := newAgentForTest(t)
	a.RegisteredAt = time.Now()
	exp := a.RegisteredAt.Add(time.Hour)
	a.ExpiresAt = &exp
	v := ExpiresAtAfterRegisteredAtInvariant{}.Check(a, time.Now())
	if v != nil {
		t.Errorf("valid: %+v", v)
	}

	exp2 := a.RegisteredAt.Add(-time.Hour)
	a.ExpiresAt = &exp2
	v = ExpiresAtAfterRegisteredAtInvariant{}.Check(a, time.Now())
	if v == nil {
		t.Error("expires before registered: expected violation")
	}
}

func TestCheckInvariants_OK(t *testing.T) {
	a := newAgentForTest(t)
	if err := CheckInvariants(a, time.Now()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckInvariants_Violation(t *testing.T) {
	a := newAgentForTest(t)
	a.State = AgentState("garbage")
	err := CheckInvariants(a, time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvariantViolation) {
		t.Errorf("err should wrap ErrInvariantViolation: %v", err)
	}
	if !strings.Contains(err.Error(), "status_legal") {
		t.Errorf("err should mention status_legal: %v", err)
	}
}

func TestCheckInvariantsAll(t *testing.T) {
	a := newAgentForTest(t)
	// 同时违反多条
	a.State = AgentState("garbage")
	exp := a.RegisteredAt.Add(-time.Hour)
	a.ExpiresAt = &exp

	vs := CheckInvariantsAll(a, time.Now(), DefaultInvariants(nil))
	if len(vs) < 2 {
		t.Errorf("expected ≥2 violations, got %d", len(vs))
	}
}
