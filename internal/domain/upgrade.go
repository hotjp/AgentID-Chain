// Package domain UpgradeAgent 业务规则。
//
// 业务流程：
//  1. 校验不变量（不可自升级 — 由 NoSelfUpgradeInvariant 兜底）
//  2. 校验等级合法性 + 单步升级
//  3. 调用 Agent.Upgrade 修改实体
//  4. permission 合并：保留旧权限 ∪ 新增位
//  5. 记录 LastUpgradeBy
//  6. 构造升级事件
//  7. 业务方负责把 Agent + Event 持久化（事务内 + outbox.Collect）
//
// 业务规则：
//   - 单步升级：newLevel = oldLevel + 1
//   - 不允许跳级（newLevel - oldLevel != 1 → ErrInvalidLevel）
//   - 不允许降级（newLevel < oldLevel → ErrInvalidLevel）
//   - 不允许自升级（operator_did == agent_self_did → ErrSelfUpgrade）
//   - permission 合并：保留原权限位，新增 newLevel.DefaultMaxPermissions() 中允许的位
package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrSelfUpgrade 不允许自升级。
var ErrSelfUpgrade = errors.New("domain: self-upgrade not allowed")

// ErrUpgradeInvalidState 当前状态不允许升级（如已 banned / unregistered）。
var ErrUpgradeInvalidState = errors.New("domain: upgrade invalid for current state")

// =============================================================================
// UpgradeAgent 业务规则
// =============================================================================

// UpgradeInput 升级入参。
type UpgradeInput struct {
	Agent        *Agent    // 待升级的 Agent 实体
	NewLevel     LevelType // 目标等级
	NewPerms     uint64    // 目标权限位（合并到 Agent.Permissions）
	OperatorDID  string    // 操作者 DID
	AgentSelfDID string    // Agent 自身 DID（用于自升级校验）
	Now          time.Time // 业务时间
	Reason       string    // 升级原因
}

// UpgradeOutput 升级出参。
type UpgradeOutput struct {
	Agent *Agent                // 修改后的 Agent（修改入参 Agent 本身）
	Event *AgentUpgradedEventV1 // 升级事件
}

// UpgradeAgent 升级业务规则。
//
// 行为：
//  1. 校验：状态合法 / 非自升级 / 单步升级
//  2. 合并 permissions
//  3. 修改 Agent.Level / Permissions / LastUpgradeBy / UpdatedAt
//  4. 构造升级事件
//
// 调用方负责持久化 Agent + 收集事件到 outbox。
func UpgradeAgent(in UpgradeInput) (*UpgradeOutput, error) {
	if in.Agent == nil {
		return nil, errors.New("domain: nil agent")
	}
	if in.OperatorDID == "" {
		return nil, errors.New("domain: empty operator_did")
	}
	if in.Now.IsZero() {
		return nil, errors.New("domain: now is zero")
	}

	// 1. 状态校验：仅 registered / active 可升级
	if in.Agent.State != StateActive && in.Agent.State != StateRegistered {
		return nil, fmt.Errorf("%w: state=%s", ErrUpgradeInvalidState, in.Agent.State)
	}

	// 2. 自升级校验
	if in.AgentSelfDID != "" && in.OperatorDID == in.AgentSelfDID {
		return nil, fmt.Errorf("%w: did=%s", ErrSelfUpgrade, in.OperatorDID)
	}

	// 3. 单步升级 + 等级合法
	if !in.NewLevel.IsValid() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(in.NewLevel))
	}
	if in.NewLevel <= in.Agent.Level {
		return nil, fmt.Errorf("%w: %s → %s", ErrInvalidLevel, in.Agent.Level, in.NewLevel)
	}
	if in.NewLevel != in.Agent.Level+1 {
		return nil, fmt.Errorf("%w: must upgrade by exactly 1 level", ErrInvalidLevel)
	}

	// 4. permission 合并：保留原权限位 ∪ 新增位（受 newLevel 上限约束）
	merged := in.Agent.Permissions | in.NewPerms
	maxPerms := in.NewLevel.DefaultMaxPermissions()
	if merged & ^maxPerms != 0 {
		return nil, fmt.Errorf("%w: merged perms exceed new level max", ErrPermissionExceedsLevel)
	}

	// 5. 修改实体
	oldLevel := in.Agent.Level
	oldPerms := in.Agent.Permissions
	if err := in.Agent.Upgrade(in.NewLevel, in.Now); err != nil {
		return nil, fmt.Errorf("domain: agent upgrade: %w", err)
	}
	in.Agent.Permissions = merged
	in.Agent.LastUpgradeBy = in.OperatorDID
	in.Agent.UpdatedAt = in.Now

	// 6. 不变量再校验（防御）
	if err := CheckInvariants(in.Agent, in.Now); err != nil {
		return nil, fmt.Errorf("%w: post-upgrade invariants failed: %w", ErrInvariantFailed, err)
	}

	// 7. 构造事件
	evt, err := NewAgentUpgradedEventV1(
		string(in.Agent.UUID)+":upgrade:"+in.Now.Format("20060102T150405.000"),
		in.Agent.UUID,
		oldLevel,
		in.NewLevel,
		oldPerms,
		merged,
		in.Reason,
		in.OperatorDID,
		in.Now,
	)
	if err != nil {
		return nil, fmt.Errorf("domain: build upgraded event: %w", err)
	}

	return &UpgradeOutput{
		Agent: in.Agent,
		Event: evt,
	}, nil
}

// CollectUpgradeOutbox 便捷包装：写入 outbox。
func CollectUpgradeOutbox(ctx context.Context, w OutboxWriter, out *UpgradeOutput) error {
	if out == nil || out.Event == nil {
		return errors.New("domain: nil upgrade output")
	}
	return Collect(ctx, w, out.Event)
}
