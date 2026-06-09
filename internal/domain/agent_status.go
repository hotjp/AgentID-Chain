// Package domain AgentStatus 状态机（声明式）。
//
// 与 state.go 的关系：
//   - state.go 定义 AgentState（registered/active/banned/unregistered）
//   - agent_status.go 在它之上提供"声明式状态机"：
//   - 状态转换表（map[from]map[to]bool）
//   - 守卫函数 CanTransitionTo(from, to)
//   - 角色化命名（"Active" / "Banned" / "Revoked"）
//
// 设计：
//   - 状态转换表是数据（不是代码）— 易于审查、易于扩展
//   - CanTransitionTo 等价于 state.go 的 CanTransition（向后兼容）
//   - 命名映射：Active=StateActive, Banned=StateBanned, Revoked=StateUnregistered
//
// 典型用法：
//
//	if !domain.CanTransitionTo(a.State, domain.StateBanned) {
//	    return ErrInvalidTransition
//	}
package domain

// AgentStatus "用户视角"状态命名（与业务术语对齐）。
//
// 内部等价：
//   - AgentStatusActive    ↔ StateActive
//   - AgentStatusBanned    ↔ StateBanned
//   - AgentStatusRevoked   ↔ StateUnregistered
//   - AgentStatusPending   ↔ StateRegistered
type AgentStatus string

// 业务状态常量。
const (
	AgentStatusPending AgentStatus = "pending" // 待激活（= StateRegistered）
	AgentStatusActive  AgentStatus = "active"  // 活跃
	AgentStatusBanned  AgentStatus = "banned"  // 封禁
	AgentStatusRevoked AgentStatus = "revoked" // 撤销（= StateUnregistered）
)

// toAgentState 业务状态 → 内部 AgentState。
func (s AgentStatus) toAgentState() AgentState {
	switch s {
	case AgentStatusPending:
		return StateRegistered
	case AgentStatusActive:
		return StateActive
	case AgentStatusBanned:
		return StateBanned
	case AgentStatusRevoked:
		return StateUnregistered
	}
	return ""
}

// 声明式状态转换表（from → to → allowed）。
//
// 数据驱动；新增转换只需修改此表。
var allowedStatusTransitions = map[AgentStatus]map[AgentStatus]bool{
	// 待激活 → 活跃 / 撤销
	AgentStatusPending: {
		AgentStatusActive:  true,
		AgentStatusRevoked: true,
		AgentStatusBanned:  true, // 允许注册即封禁（如黑名单）
	},

	// 活跃 → 封禁 / 撤销
	AgentStatusActive: {
		AgentStatusBanned:  true,
		AgentStatusRevoked: true,
	},

	// 封禁 → 活跃 / 撤销
	AgentStatusBanned: {
		AgentStatusActive:  true, // 解封
		AgentStatusRevoked: true, // 封禁后直接撤销
	},

	// 撤销：终态（不可再变）
	AgentStatusRevoked: {},
}

// CanTransitionTo 守卫函数：判断 from → to 是否合法。
//
// 内部走 state.go 的 CanTransition（保持单一事实源）。
func CanTransitionTo(from, to AgentStatus) bool {
	fromState := from.toAgentState()
	toState := to.toAgentState()
	if fromState == "" || toState == "" {
		return false
	}
	// 业务表优先（更严格）；状态机兜底
	if to, ok := allowedStatusTransitions[from]; ok {
		if allowed, has := to[AgentStatus(toState.toStatus())]; has {
			return allowed
		}
	}
	// 兜底：内部状态机的允许列表
	return CanTransition(fromState, toState)
}

// toStatus AgentState → AgentStatus（反向映射）。
func (s AgentState) toStatus() AgentStatus {
	switch s {
	case StateRegistered:
		return AgentStatusPending
	case StateActive:
		return AgentStatusActive
	case StateBanned:
		return AgentStatusBanned
	case StateUnregistered:
		return AgentStatusRevoked
	}
	return ""
}

// AllowedTransitions 返回 from 的所有合法目标状态。
//
// 用途：UI 显示"可以进行的下一步"列表。
func AllowedTransitions(from AgentStatus) []AgentStatus {
	to, ok := allowedStatusTransitions[from]
	if !ok {
		return nil
	}
	out := make([]AgentStatus, 0, len(to))
	for s := range to {
		out = append(out, s)
	}
	return out
}

// IsTerminal 业务终态判断。
func (s AgentStatus) IsTerminal() bool {
	return s == AgentStatusRevoked
}
