package a2a

import (
	"testing"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

// =============================================================================
// TrustLevel 类型
// =============================================================================

func TestTrustLevel_IsValid(t *testing.T) {
	for _, tl := range []TrustLevel{TrustNone, TrustBasic, TrustFull} {
		if !tl.IsValid() {
			t.Errorf("%s should be valid", tl)
		}
	}
	if TrustLevel("invalid").IsValid() {
		t.Error("invalid should be invalid")
	}
}

func TestTrustLevel_String(t *testing.T) {
	cases := map[TrustLevel]string{
		TrustNone: "none", TrustBasic: "basic", TrustFull: "full",
	}
	for tl, s := range cases {
		if tl.String() != s {
			t.Errorf("%v.String() = %q, want %q", tl, tl.String(), s)
		}
	}
}

func TestTrustLevel_Score(t *testing.T) {
	cases := map[TrustLevel]int{
		TrustNone: 0, TrustBasic: 50, TrustFull: 100,
		TrustLevel("invalid"): 0,
	}
	for tl, want := range cases {
		if got := tl.Score(); got != want {
			t.Errorf("%v.Score() = %d, want %d", tl, got, want)
		}
	}
}

func TestParseTrustLevel(t *testing.T) {
	cases := []struct {
		in   string
		want TrustLevel
		ok   bool
	}{
		{"none", TrustNone, true},
		{"NONE", TrustNone, true},
		{"  basic  ", TrustBasic, true},
		{"Full", TrustFull, true},
		{"invalid", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, err := ParseTrustLevel(c.in)
		if c.ok {
			if err != nil {
				t.Errorf("ParseTrustLevel(%q) err = %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseTrustLevel(%q) = %v, want %v", c.in, got, c.want)
			}
		} else {
			if err == nil {
				t.Errorf("ParseTrustLevel(%q) expected error", c.in)
			}
		}
	}
}

// =============================================================================
// PermA2AInterop
// =============================================================================

func TestHasA2AInterop(t *testing.T) {
	if HasA2AInterop(0) {
		t.Error("zero perms should not have A2A")
	}
	if !HasA2AInterop(uint64(1) << PermA2AInterop) {
		t.Error("bit 8 should have A2A")
	}
	if HasA2AInterop(uint64(1) << 7) {
		t.Error("bit 7 should not match A2A (bit 8)")
	}
}

// =============================================================================
// AgentSnapshot
// =============================================================================

func TestFromAgent_Nil(t *testing.T) {
	got := FromAgent(nil)
	if got.UUID != "" {
		t.Errorf("UUID = %q", got.UUID)
	}
}

func TestFromAgent_OK(t *testing.T) {
	a := &domain.Agent{
		UUID:        "uuid-001",
		State:      domain.StateActive,
		Level:       domain.LevelAdvanced,
		Permissions: uint64(1) << PermA2AInterop,
	}
	s := FromAgent(a)
	if s.UUID != "uuid-001" {
		t.Errorf("UUID = %q", s.UUID)
	}
	if s.State != domain.StateActive {
		t.Errorf("State = %s", s.State)
	}
	if s.Level != domain.LevelAdvanced {
		t.Errorf("Level = %s", s.Level)
	}
	if !HasA2AInterop(s.Permissions) {
		t.Error("perm bit lost")
	}
}

// =============================================================================
// ComputeTrustLevel — 规则覆盖
// =============================================================================

func snap(state domain.AgentState, level domain.LevelType, perms uint64) AgentSnapshot {
	return AgentSnapshot{UUID: "u", State: state, Level: level, Permissions: perms}
}

func TestComputeTrustLevel_Rule1_BannedOrRevoked(t *testing.T) {
	good := snap(domain.StateActive, domain.LevelAdvanced, 0)
	cases := []domain.AgentState{
		domain.StateBanned,
		domain.StateUnregistered,
		domain.StateRegistered,
	}
	for _, st := range cases {
		bad := snap(st, domain.LevelAdvanced, 0)
		if got := ComputeTrustLevel(good, bad); got != TrustNone {
			t.Errorf("good vs %s: got %v, want none", st, got)
		}
		if got := ComputeTrustLevel(bad, good); got != TrustNone {
			t.Errorf("%s vs good: got %v, want none", st, got)
		}
	}
}

func TestComputeTrustLevel_Rule1_BothBanned(t *testing.T) {
	a := snap(domain.StateBanned, domain.LevelAdvanced, 0)
	b := snap(domain.StateBanned, domain.LevelAdvanced, 0)
	if got := ComputeTrustLevel(a, b); got != TrustNone {
		t.Errorf("both banned: got %v", got)
	}
}

func TestComputeTrustLevel_Rule2_TestAgent(t *testing.T) {
	test := snap(domain.StateActive, domain.LevelTest, 0)
	pro := snap(domain.StateActive, domain.LevelPro,
		uint64(1)<<PermA2AInterop)
	if got := ComputeTrustLevel(test, pro); got != TrustBasic {
		t.Errorf("test vs pro: got %v", got)
	}
	if got := ComputeTrustLevel(pro, test); got != TrustBasic {
		t.Errorf("pro vs test: got %v", got)
	}
}

func TestComputeTrustLevel_Rule3_LevelDiff(t *testing.T) {
	// LevelBasic (1) vs LevelReserved4 (4) -> diff 3 -> basic
	a := snap(domain.StateActive, domain.LevelBasic,
		uint64(1)<<PermA2AInterop)
	b := snap(domain.StateActive, domain.LevelReserved4,
		uint64(1)<<PermA2AInterop)
	if got := ComputeTrustLevel(a, b); got != TrustBasic {
		t.Errorf("diff 3: got %v", got)
	}
}

func TestComputeTrustLevel_Rule3_LevelDiff_Exactly2(t *testing.T) {
	// LevelBasic (1) vs LevelPro (3) -> diff 2 -> NOT degraded by rule 3
	// 但 rule 4 检查 level>=advanced, basic 不满足 → 兜底 basic
	a := snap(domain.StateActive, domain.LevelBasic,
		uint64(1)<<PermA2AInterop)
	b := snap(domain.StateActive, domain.LevelPro,
		uint64(1)<<PermA2AInterop)
	if got := ComputeTrustLevel(a, b); got != TrustBasic {
		t.Errorf("diff 2: got %v, want basic (fallback because basic level <advanced)", got)
	}
}

func TestComputeTrustLevel_Rule4_Full(t *testing.T) {
	a := snap(domain.StateActive, domain.LevelAdvanced,
		uint64(1)<<PermA2AInterop)
	b := snap(domain.StateActive, domain.LevelPro,
		uint64(1)<<PermA2AInterop)
	if got := ComputeTrustLevel(a, b); got != TrustFull {
		t.Errorf("both advanced+A2A: got %v, want full", got)
	}
}

func TestComputeTrustLevel_Rule4_Symmetric(t *testing.T) {
	a := snap(domain.StateActive, domain.LevelAdvanced,
		uint64(1)<<PermA2AInterop)
	b := snap(domain.StateActive, domain.LevelAdvanced,
		uint64(1)<<PermA2AInterop)
	if ComputeTrustLevel(a, b) != ComputeTrustLevel(b, a) {
		t.Error("not symmetric")
	}
}

func TestComputeTrustLevel_Rule5_FallbackBasic(t *testing.T) {
	// 双方 advanced 但都没有 A2A 权限 → basic
	a := snap(domain.StateActive, domain.LevelAdvanced, 0)
	b := snap(domain.StateActive, domain.LevelAdvanced, 0)
	if got := ComputeTrustLevel(a, b); got != TrustBasic {
		t.Errorf("no A2A perm: got %v", got)
	}
}

func TestComputeTrustLevel_Rule5_OnlyOneHasA2A(t *testing.T) {
	a := snap(domain.StateActive, domain.LevelAdvanced,
		uint64(1)<<PermA2AInterop)
	b := snap(domain.StateActive, domain.LevelAdvanced, 0)
	if got := ComputeTrustLevel(a, b); got != TrustBasic {
		t.Errorf("only one has A2A: got %v", got)
	}
}

func TestComputeTrustLevel_Rule4_BothPlatform(t *testing.T) {
	a := snap(domain.StateActive, domain.LevelReserved7,
		uint64(1)<<PermA2AInterop)
	b := snap(domain.StateActive, domain.LevelReserved7,
		uint64(1)<<PermA2AInterop)
	if got := ComputeTrustLevel(a, b); got != TrustFull {
		t.Errorf("both platform+A2A: got %v", got)
	}
}

// =============================================================================
// ComputeTrustLevelFromAgents
// =============================================================================

func TestComputeTrustLevelFromAgents_Nil(t *testing.T) {
	a := &domain.Agent{State:      domain.StateActive, Level: domain.LevelAdvanced}
	if ComputeTrustLevelFromAgents(nil, a) != TrustNone {
		t.Error("nil a should be none")
	}
	if ComputeTrustLevelFromAgents(a, nil) != TrustNone {
		t.Error("nil b should be none")
	}
}

func TestComputeTrustLevelFromAgents_OK(t *testing.T) {
	a := &domain.Agent{
		State:      domain.StateActive,
		Level:       domain.LevelAdvanced,
		Permissions: uint64(1) << PermA2AInterop,
	}
	b := &domain.Agent{
		State:      domain.StateActive,
		Level:       domain.LevelPro,
		Permissions: uint64(1) << PermA2AInterop,
	}
	if got := ComputeTrustLevelFromAgents(a, b); got != TrustFull {
		t.Errorf("got %v, want full", got)
	}
}
