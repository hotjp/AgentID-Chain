package domain

import "fmt"

// AgentState Agent 状态枚举。
type AgentState string

// 状态常量（与 docs §3 状态机图对齐）。
const (
	StateRegistered   AgentState = "registered"   // 已注册，待激活
	StateActive       AgentState = "active"       // 正常可用
	StateBanned       AgentState = "banned"       // 已封禁
	StateUnregistered AgentState = "unregistered" // 已注销（终态）
)

// IsValid 校验状态值。
func (s AgentState) IsValid() bool {
	switch s {
	case StateRegistered, StateActive, StateBanned, StateUnregistered:
		return true
	}
	return false
}

// IsTerminal 终态判断（不可再变更）。
func (s AgentState) IsTerminal() bool {
	return s == StateUnregistered
}

// String 返回状态可读名。
func (s AgentState) String() string { return string(s) }

// StateTransition 状态机迁移表。
//
// 行 = 当前状态；列 = 目标状态；值 = 是否允许。
var StateTransition = map[AgentState]map[AgentState]bool{
	StateRegistered: {
		StateActive:       true,
		StateBanned:       true,
		StateUnregistered: true,
	},
	StateActive: {
		StateBanned:       true,
		StateUnregistered: true,
	},
	StateBanned: {
		StateActive:       true, // Unban
		StateUnregistered: true,
	},
	StateUnregistered: {}, // 终态
}

// CanTransition 判断 from → to 是否合法。
func CanTransition(from, to AgentState) bool {
	if !from.IsValid() || !to.IsValid() {
		return false
	}
	return StateTransition[from][to]
}

// ValidateTransition 显式错误版（用于 L3/L4 主动校验）。
func ValidateTransition(from, to AgentState) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, from, to)
	}
	return nil
}
