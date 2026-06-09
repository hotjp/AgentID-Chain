// Package domain ChangeLog 值对象。
//
// ChangeLog 记录 Agent 状态变更（注册 / 升级 / 封禁 / 解封 / 注销 / 权限变更）。
// 不可变；与 ent.AuditLog 1:1 映射（domain → L1 转换器负责）。
//
// 字段：
//   - UUID        变更记录 UUID
//   - AgentUUID   归属 Agent UUID
//   - Action      动作类型（"register" / "upgrade" / "ban" / "unban" / "unregister" / "grant" / "revoke"）
//   - OldValue    变更前的值（JSON 字符串；与 Action 配套）
//   - NewValue    变更后的值
//   - OperatorDID 操作者 DID
//   - Timestamp   发生时间
//   - Reason      业务原因（可空；如"违反策略 X"）
package domain

import (
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrInvalidAction 动作类型不在白名单。
var ErrInvalidAction = errors.New("domain: invalid action")

// 合法动作白名单（与 docs §3.4 状态机对齐）。
var validActions = map[string]struct{}{
	"register":   {},
	"upgrade":    {},
	"downgrade":  {},
	"ban":        {},
	"unban":      {},
	"unregister": {},
	"grant":      {},
	"revoke":     {},
	"activate":   {},
	"suspend":    {},
	"resume":     {},
	"renew":      {},
}

// =============================================================================
// ChangeLog 值对象
// =============================================================================

// ChangeLog Agent 状态变更记录。
type ChangeLog struct {
	UUID        UUID
	AgentUUID   UUID
	Action      string
	OldValue    string
	NewValue    string
	OperatorDID string
	Timestamp   time.Time
	Reason      string
}

// NewChangeLog 构造 ChangeLog。
//
// 校验：
//   - AgentUUID 非零
//   - Action 合法（白名单）
//   - OperatorDID 非空
//   - OldValue / NewValue / Reason 可空
func NewChangeLog(
	uuid UUID,
	agentUUID UUID,
	action string,
	oldValue, newValue, operatorDID, reason string,
	now time.Time,
) (*ChangeLog, error) {
	if uuid.IsZero() {
		return nil, fmt.Errorf("%w: uuid is empty", ErrInvalidUUID)
	}
	if agentUUID.IsZero() {
		return nil, fmt.Errorf("%w: agent uuid is empty", ErrInvalidUUID)
	}
	if _, ok := validActions[action]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidAction, action)
	}
	if operatorDID == "" {
		return nil, errors.New("domain: operator_did is empty")
	}
	return &ChangeLog{
		UUID:        uuid,
		AgentUUID:   agentUUID,
		Action:      action,
		OldValue:    oldValue,
		NewValue:    newValue,
		OperatorDID: operatorDID,
		Timestamp:   now,
		Reason:      reason,
	}, nil
}

// IsValidAction 静态校验动作是否合法（导出供其他包使用）。
func IsValidAction(action string) bool {
	_, ok := validActions[action]
	return ok
}

// Actions 返回合法动作列表（snapshot）。
func Actions() []string {
	out := make([]string, 0, len(validActions))
	for k := range validActions {
		out = append(out, k)
	}
	return out
}
