// Package a2a TrustLevel 计算（v2.0.1 §4.3.5）。
//
// TrustLevel 描述两个 agent 之间的互信程度。
// 在 A2A token 颁发时由网关计算并写入 claims.trust_level。
//
// 计算规则（v2.0.1 §4.3.5）：
//  1. 任一方非 active → TrustNone
//  2. 任一方 LevelTest（=0） → TrustBasic
//  3. 两方 Level 差 > 2 → TrustBasic（避免高 / 低权限互信）
//  4. 双方都拥有 PermA2AInterop 且 level≥advanced(2) → TrustFull
//  5. 兜底 → TrustBasic
package a2a

import (
	"fmt"
	"strings"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

// =============================================================================
// TrustLevel 类型
// =============================================================================

// TrustLevel A2A 互信等级（字符串枚举）。
type TrustLevel string

// 互信等级常量。
const (
	// TrustNone 不互信（任一方被封禁 / 撤销）。
	TrustNone TrustLevel = "none"
	// TrustBasic 仅身份确认（默认互信）。
	TrustBasic TrustLevel = "basic"
	// TrustFull 完全互信（双方均 advanced+ 且开通 A2A 权限）。
	TrustFull TrustLevel = "full"
)

// IsValid 校验 TrustLevel 是否合法。
func (t TrustLevel) IsValid() bool {
	switch t {
	case TrustNone, TrustBasic, TrustFull:
		return true
	}
	return false
}

// String 序列化。
func (t TrustLevel) String() string { return string(t) }

// Score 数值化（用于 claims.trust_level 整数化场景）。
//
//	none  → 0
//	basic → 50
//	full  → 100
func (t TrustLevel) Score() int {
	switch t {
	case TrustNone:
		return 0
	case TrustBasic:
		return 50
	case TrustFull:
		return 100
	}
	return 0
}

// ParseTrustLevel 字符串 → TrustLevel。
func ParseTrustLevel(s string) (TrustLevel, error) {
	t := TrustLevel(strings.ToLower(strings.TrimSpace(s)))
	if !t.IsValid() {
		return "", fmt.Errorf("a2a: invalid trust level %q", s)
	}
	return t, nil
}

// =============================================================================
// PermA2AInterop 权限位定义
// =============================================================================

// PermA2AInterop A2A 互操作权限位。
//
// 该位定义在 basic level 范围内（bit 8），所有 level≥basic 的 agent
// 都可选择性开启 / 关闭 A2A 互操作能力。
const PermA2AInterop uint = 8

// HasA2AInterop 检查 agent 是否开通了 A2A 互操作权限。
func HasA2AInterop(perms uint64) bool {
	return perms&(uint64(1)<<PermA2AInterop) != 0
}

// =============================================================================
// AgentSnapshot：computeTrustLevel 入参
// =============================================================================

// AgentSnapshot 计算 TrustLevel 所需的最小 agent 视图。
//
// 用 snapshot 替代直接传 *domain.Agent，避免长链路依赖；
// 也便于 mock 测试。
type AgentSnapshot struct {
	// UUID agent 唯一 ID（仅用于错误信息 / 日志）
	UUID string
	// State 当前状态（registered / active / banned / unregistered）
	State domain.AgentState
	// Level 等级
	Level domain.LevelType
	// Permissions 权限位掩码
	Permissions uint64
}

// FromAgent 从 *domain.Agent 派生 snapshot。
func FromAgent(a *domain.Agent) AgentSnapshot {
	if a == nil {
		return AgentSnapshot{}
	}
	return AgentSnapshot{
		UUID:        string(a.UUID),
		State:       a.State,
		Level:       a.Level,
		Permissions: a.Permissions,
	}
}

// =============================================================================
// 核心：ComputeTrustLevel
// =============================================================================

// ComputeTrustLevel 计算双方互信等级（v2.0.1 §4.3.5 规则）。
//
// 顺序敏感的规则链（前面命中即返回）：
//  1. 任一方非 active → TrustNone
//  2. 任一方 LevelTest → TrustBasic
//  3. 等级差 > 2 → TrustBasic
//  4. 双方 PermA2AInterop && level≥advanced → TrustFull
//  5. 兜底 → TrustBasic
func ComputeTrustLevel(a, b AgentSnapshot) TrustLevel {
	// 规则 1：状态检查（必须均为 active）
	if a.State != domain.StateActive || b.State != domain.StateActive {
		return TrustNone
	}

	// 规则 2：测试 agent 参与 → basic
	if a.Level == domain.LevelTest || b.Level == domain.LevelTest {
		return TrustBasic
	}

	// 规则 3：等级差 > 2
	diff := int(a.Level) - int(b.Level)
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		return TrustBasic
	}

	// 规则 4：双方 A2A 互操作权限 + level≥advanced
	if HasA2AInterop(a.Permissions) && HasA2AInterop(b.Permissions) &&
		a.Level >= domain.LevelAdvanced && b.Level >= domain.LevelAdvanced {
		return TrustFull
	}

	// 规则 5：兜底
	return TrustBasic
}

// ComputeTrustLevelFromAgents 接收 *domain.Agent 的便捷版本。
func ComputeTrustLevelFromAgents(a, b *domain.Agent) TrustLevel {
	if a == nil || b == nil {
		return TrustNone
	}
	return ComputeTrustLevel(FromAgent(a), FromAgent(b))
}
