package domain

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// =============================================================================
// Property 1: 新构造的 Agent 必须通过所有默认不变量
//   任意合法 Level / 任意合法 PublicKey / 任意 now，新 Agent 不变量全过。
// =============================================================================

func TestProperty_NewAgentSatisfiesInvariants(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		uuid := "01234567-89ab-cdef-0123-456789abcdef"
		level := rapid.SampledFrom([]LevelType{LevelTest, LevelBasic, LevelAdvanced, LevelPro}).Draw(t, "level")
		now := time.Now()
		agent, err := NewAgent(UUID(uuid), "test-owner", level, "pubkey-data", now)
		if err != nil {
			t.Fatalf("NewAgent: %v", err)
		}
		if v := CheckInvariantsAll(agent, now, DefaultInvariants(nil)); len(v) != 0 {
			t.Fatalf("new agent violated invariants: %+v", v)
		}
	})
}

// =============================================================================
// Property 2: PermissionWithinLevelInvariant — 位 ∈ max → 满足；位 ∉ max → 违反
//   构造一个 agent，在 level.max 内随机设一些位 → 满足。
//   构造一个 agent，强制在某 max 外位设 1 → 违反。
// =============================================================================

func TestProperty_PermissionWithinLevel(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()
		agent, _ := NewAgent(UUID("01234567-89ab-cdef-0123-456789abcdef"),
			"test-owner", LevelBasic, "pk", now)
		agent.Activate(now)

		// 在 level 允许范围内随机设位
		max := agent.Level.DefaultMaxPermissions()
		bits := rapid.SliceOf(rapid.IntRange(0, 15)).Draw(t, "bits")
		var perms uint64
		for _, b := range bits {
			perms |= uint64(1) << b
		}
		perms &= max // mask
		agent.Permissions = perms

		inv := PermissionWithinLevelInvariant{}
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("masked perms should pass, got %v (perms=%#x max=%#x)", v, perms, max)
		}

		// 强制在 max 之外设位（位 50），必须违反
		agent.Permissions = perms | (uint64(1) << 50)
		if v := inv.Check(agent, now); v == nil {
			t.Fatalf("bit-50 should violate perm_within_level")
		}
	})
}

// =============================================================================
// Property 3: StatusLegalInvariant — 状态值合法性
//   合法状态值 → 不违反；非法值（空 / "garbage"） → 违反。
// =============================================================================

func TestProperty_StatusLegal(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()
		agent, _ := NewAgent(UUID("01234567-89ab-cdef-0123-456789abcdef"),
			"test-owner", LevelBasic, "pk", now)

		// 合法状态序列
		legalStates := []AgentState{StateRegistered, StateActive, StateBanned, StateUnregistered}
		for _, s := range legalStates {
			agent.State = s
			inv := StatusLegalInvariant{}
			if v := inv.Check(agent, now); v != nil {
				t.Fatalf("legal state %s should pass, got %v", s, v)
			}
		}

		// 非法状态
		agent.State = AgentState("garbage")
		inv := StatusLegalInvariant{}
		if v := inv.Check(agent, now); v == nil {
			t.Fatalf("garbage state should violate status_legal")
		}
	})
}

// =============================================================================
// Property 4: ExpiresAtAfterRegisteredAtInvariant
//   ExpiresAt >= RegisteredAt → 满足；ExpiresAt < RegisteredAt → 违反。
//   ExpiresAt == nil → 满足。
// =============================================================================

func TestProperty_ExpiresAtAfterRegisteredAt(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		registeredAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		now := time.Now()
		agent, _ := NewAgent(UUID("01234567-89ab-cdef-0123-456789abcdef"),
			"test-owner", LevelBasic, "pk", registeredAt)

		inv := ExpiresAtAfterRegisteredAtInvariant{}

		// nil → pass
		agent.ExpiresAt = nil
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("nil ExpiresAt should pass: %v", v)
		}

		// ExpiresAt > RegisteredAt → pass
		agent.ExpiresAt = ptrTime(registeredAt.Add(24 * time.Hour))
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("future ExpiresAt should pass: %v", v)
		}

		// ExpiresAt == RegisteredAt → pass（边界）
		agent.ExpiresAt = ptrTime(registeredAt)
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("equal ExpiresAt should pass: %v", v)
		}

		// ExpiresAt < RegisteredAt → 违反
		agent.ExpiresAt = ptrTime(registeredAt.Add(-time.Hour))
		if v := inv.Check(agent, now); v == nil {
			t.Fatalf("past ExpiresAt should violate")
		}
	})
}

// =============================================================================
// Property 5: TemporaryPermissionExpireInvariant
//   临时权限位到期前：仍设 → pass（inv 只在位仍存在但已到期时违反）
//   临时权限位到期后：位已清 → pass；位未清 → 违反
// =============================================================================

func TestProperty_TempPermExpire(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()
		agent, _ := NewAgent(UUID("01234567-89ab-cdef-0123-456789abcdef"),
			"test-owner", LevelBasic, "pk", now)
		agent.Activate(now)

		inv := TemporaryPermissionExpireInvariant{}

		// 没临时权限 → pass
		agent.TempPermissions = nil
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("no temp perms should pass: %v", v)
		}

		// 临时权限未到期，位仍设 → pass
		future := now.Add(time.Hour)
		agent.TempPermissions = map[uint]time.Time{0: future}
		agent.Permissions = 1 << 0
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("unexpired temp perm should pass: %v", v)
		}

		// 临时权限已到期，位已清 → pass
		past := now.Add(-time.Hour)
		agent.TempPermissions = map[uint]time.Time{0: past}
		agent.Permissions = 0
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("cleared expired temp perm should pass: %v", v)
		}

		// 临时权限已到期，位未清 → 违反
		agent.Permissions = 1 << 0
		if v := inv.Check(agent, now); v == nil {
			t.Fatalf("uncleared expired temp perm should violate")
		}
	})
}

// =============================================================================
// Property 6: NoSelfUpgradeInvariant
//   LastUpgradeBy == AgentSelfDID → 违反；其他情况 → 满足。
// =============================================================================

func TestProperty_NoSelfUpgrade(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()
		agent, _ := NewAgent(UUID("01234567-89ab-cdef-0123-456789abcdef"),
			"test-owner", LevelBasic, "pk", now)
		agent.Activate(now)

		selfDID := "did:agentid:agent:" + string(agent.UUID)
		inv := NoSelfUpgradeInvariant{AgentSelfDID: func(*Agent) string { return selfDID }}

		// LastUpgradeBy == selfDID → 违反
		agent.LastUpgradeBy = selfDID
		if v := inv.Check(agent, now); v == nil {
			t.Fatalf("self-upgrade should violate")
		}

		// LastUpgradeBy == "" → 满足
		agent.LastUpgradeBy = ""
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("empty LastUpgradeBy should pass: %v", v)
		}

		// LastUpgradeBy == 其他 DID → 满足
		agent.LastUpgradeBy = "did:agentid:user:admin"
		if v := inv.Check(agent, now); v != nil {
			t.Fatalf("external upgrade should pass: %v", v)
		}

		// selfDID func == nil → 跳过（不强制）
		agent.LastUpgradeBy = selfDID
		skip := NoSelfUpgradeInvariant{AgentSelfDID: nil}
		if v := skip.Check(agent, now); v != nil {
			t.Fatalf("nil selfDID should skip, got %v", v)
		}
	})
}

// =============================================================================
// Property 7: 完整不变量在合法序列中保持
//   构造 agent → 激活 → 在 level 范围内 grant 随机位 → 所有不变量通过。
// =============================================================================

func TestProperty_AllInvariantsHoldUnderValidOps(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()
		agent, _ := NewAgent(UUID("01234567-89ab-cdef-0123-456789abcdef"),
			"test-owner", LevelBasic, "pk", now)
		agent.Activate(now)

		// 在 level 允许范围内随机 grant
		bits := rapid.SliceOf(rapid.IntRange(0, 15)).Draw(t, "bits")
		for _, b := range bits {
			_ = agent.Grant(uint(b))
		}

		// 跑全部默认不变量
		v := CheckInvariantsAll(agent, now, DefaultInvariants(nil))
		if len(v) != 0 {
			t.Fatalf("valid op sequence violated invariants: %+v (perms=%#x)",
				v, agent.Permissions)
		}
	})
}

// =============================================================================
// 工具函数
// =============================================================================

// ptrTime 返回 *time.Time 指针。
func ptrTime(t time.Time) *time.Time {
	return &t
}
