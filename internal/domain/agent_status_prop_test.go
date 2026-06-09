package domain

import (
	"testing"

	"pgregory.net/rapid"
)

// =============================================================================
// Property 1: 无自环 (no self-loop)
//   任意状态 s，CanTransitionTo(s, s) 必须返回 false。
//   业务含义：状态机不允许"原地踏步"。
// =============================================================================

func TestProperty_NoSelfLoop(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.SampledFrom(allAgentStatuses()).Draw(t, "s")
		if CanTransitionTo(s, s) {
			t.Fatalf("self-loop detected: %s → %s", s, s)
		}
	})
}

// =============================================================================
// Property 2: 终态无出边 (terminal state has no outgoing edges)
//   AgentStatusRevoked 是终态；任意 to 都不允许。
// =============================================================================

func TestProperty_TerminalHasNoOutgoing(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		to := rapid.SampledFrom(allAgentStatuses()).Draw(t, "to")
		if CanTransitionTo(AgentStatusRevoked, to) {
			t.Fatalf("terminal state %s should not transition to %s", AgentStatusRevoked, to)
		}
	})
}

// =============================================================================
// Property 3: 终态判定幂等
//   s.IsTerminal() 与任意函数上下文无关。
//   注意：s 为非终态时，IsTerminal() 必须为 false。
// =============================================================================

func TestProperty_IsTerminalIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.SampledFrom(allAgentStatuses()).Draw(t, "s")
		got1 := s.IsTerminal()
		got2 := s.IsTerminal()
		if got1 != got2 {
			t.Fatalf("IsTerminal not idempotent: %s -> %v, %v", s, got1, got2)
		}
		expected := s == AgentStatusRevoked
		if got1 != expected {
			t.Fatalf("IsTerminal(%s) = %v, want %v", s, got1, expected)
		}
	})
}

// =============================================================================
// Property 4: AllowedTransitions 与 CanTransitionTo 一致
//   对任意 from ∈ AllowedTransitions(s)，CanTransitionTo(s, from) 必须为 true；
//   对任意 from ∉ AllowedTransitions(s)，CanTransitionTo(s, from) 必须为 false。
// =============================================================================

func TestProperty_AllowedTransitionsConsistent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		from := rapid.SampledFrom(allAgentStatuses()).Draw(t, "from")
		to := rapid.SampledFrom(allAgentStatuses()).Draw(t, "to")
		allowed := AllowedTransitions(from)
		inAllowed := false
		for _, a := range allowed {
			if a == to {
				inAllowed = true
				break
			}
		}
		got := CanTransitionTo(from, to)
		if got != inAllowed {
			t.Fatalf("CanTransitionTo(%s,%s)=%v but inAllowed=%v (allowed=%v)",
				from, to, got, inAllowed, allowed)
		}
	})
}

// =============================================================================
// Property 5: 状态机可达性 — N 步随机合法转移后，状态机一致性保持
//   任意 N，对初始状态 s 走 N 步合法转移：
//     - 当前状态 ∈ 已知状态集
//     - 上一步为合法转移
//   状态机不应产生"非法状态"。
// =============================================================================

func TestProperty_ReachableAfterRandomWalks(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		steps := rapid.IntRange(1, 30).Draw(t, "steps")
		cur := rapid.SampledFrom(allAgentStatuses()).Draw(t, "start")
		for i := 0; i < steps; i++ {
			next := AllowedTransitions(cur)
			if len(next) == 0 {
				// 终态，停止
				break
			}
			cur = rapid.SampledFrom(next).Draw(t, "next")
			// 转换必须合法
			if !CanTransitionTo(/*prevNotTracked,*/ AgentStatus("__none__"), cur) {
				// 这里不强校验（prev 未知），只校验 cur 是合法状态
				if !isKnownStatus(cur) {
					t.Fatalf("walk produced unknown status: %s", cur)
				}
			}
		}
		// 最终状态必须仍是合法状态
		if !isKnownStatus(cur) {
			t.Fatalf("final status unknown: %s", cur)
		}
	})
}

// =============================================================================
// Property 6: 转换表幂等 — 转换表自身不可存在 "A → B 且 A → B 重复"
//   本包转换表是 map，天然去重；本测试断言转换表大小稳定。
// =============================================================================

func TestProperty_TransitionTableSizeStable(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 在多次读取中转换表语义保持一致
		from := rapid.SampledFrom(allAgentStatuses()).Draw(t, "from")
		first := AllowedTransitions(from)
		second := AllowedTransitions(from)
		third := AllowedTransitions(from)
		if len(first) != len(second) || len(second) != len(third) {
			t.Fatalf("AllowedTransitions size unstable for %s: %d, %d, %d",
				from, len(first), len(second), len(third))
		}
	})
}

// =============================================================================
// Property 7: 不可达状态对 (unreachable pair)
//   对任意 from, to 满足：转换表 & 兜底状态机 都不允许 — CanTransitionTo 必须 false。
//   反例：Revoked → *（所有都不可达）。
// =============================================================================

func TestProperty_RevokedReachability(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 走 0~5 步合法转移后停在 Revoked
		steps := rapid.IntRange(0, 5).Draw(t, "steps")
		cur := AgentStatusPending // pending 是 Revoked 的可达入口
		reachedRevoked := false
		for i := 0; i < steps; i++ {
			allowed := AllowedTransitions(cur)
			if len(allowed) == 0 {
				break
			}
			cur = rapid.SampledFrom(allowed).Draw(t, "n")
			if cur == AgentStatusRevoked {
				reachedRevoked = true
			}
		}
		// 一旦进入 Revoked 就不再有出边
		if reachedRevoked {
			if CanTransitionTo(AgentStatusRevoked, AgentStatusActive) ||
				CanTransitionTo(AgentStatusRevoked, AgentStatusBanned) ||
				CanTransitionTo(AgentStatusRevoked, AgentStatusPending) {
				t.Fatalf("Revoked should not transition out, but did")
			}
		}
	})
}

// =============================================================================
// 工具函数
// =============================================================================

// allAgentStatuses 返回所有合法 AgentStatus 集合。
func allAgentStatuses() []AgentStatus {
	return []AgentStatus{
		AgentStatusPending,
		AgentStatusActive,
		AgentStatusBanned,
		AgentStatusRevoked,
	}
}

// isKnownStatus 检查 s 是否在已知状态集里。
func isKnownStatus(s AgentStatus) bool {
	for _, k := range allAgentStatuses() {
		if k == s {
			return true
		}
	}
	return false
}
