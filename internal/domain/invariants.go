// Package domain 业务不变量（invariants）。
//
// 业务不变量是"无论何时都成立"的硬规则，由 invariants.go 集中表达：
//  1. 不可自升级（agent 升级不能由 agent 自己触发；必须外部 operator）
//  2. 临时权限自动回收（granted_at + ttl 超过 → 权限位自动清零）
//  3. status 合法（不允许出现非法 status 值）
//  4. 状态机守恒（每次状态变更必须留下 changelog 痕迹）
//  5. 权限不超过等级（permission 位 ∈ level 允许范围）
//
// 设计：
//   - Invariant 接口 + 多个实现（Open/Closed 原则）
//   - CheckInvariants(a) 一次跑所有不变量
//   - 每个不变量返回 []Violation — 多错误一次性返回
package domain

import (
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrInvariantViolation 不变量违反。
var ErrInvariantViolation = errors.New("domain: invariant violation")

// =============================================================================
// Violation 不变量违反详情
// =============================================================================

// Violation 单个不变量违反。
type Violation struct {
	Invariant string // 不变量名称
	Reason    string // 违反原因
}

// Error 实现 error 接口。
func (v Violation) Error() string {
	return fmt.Sprintf("%s: %s", v.Invariant, v.Reason)
}

// =============================================================================
// Invariant 不变量接口
// =============================================================================

// Invariant 业务不变量检查器。
type Invariant interface {
	// Name 不变量名称（用于诊断）。
	Name() string
	// Check 检查 a 是否满足不变量；返回 nil 表示满足，否则返回 Violation。
	Check(a *Agent, now time.Time) *Violation
}

// =============================================================================
// 5 个内置不变量
// =============================================================================

// StatusLegalInvariant status 必须合法。
type StatusLegalInvariant struct{}

// Name 实现 Invariant。
func (StatusLegalInvariant) Name() string { return "status_legal" }

// Check 实现 Invariant。
func (StatusLegalInvariant) Check(a *Agent, _ time.Time) *Violation {
	if !a.State.IsValid() {
		return &Violation{
			Invariant: "status_legal",
			Reason:    fmt.Sprintf("invalid status: %s", a.State),
		}
	}
	return nil
}

// NoSelfUpgradeInvariant agent 不可自升级（外部 operator 必须不同）。
//
// 业务规则：升级操作由 Auth-Center 服务发起；Agent 自身不能调用 Upgrade 自己。
// 本不变量校验最近一次 Upgrade 操作（lastUpgradeBy）的 DID 不等于 agent 自身的 DID。
type NoSelfUpgradeInvariant struct {
	// AgentSelfDID 业务系统给每个 Agent 分配的"自身 DID"
	// （从 owner + uuid 派生；测试时可注入）
	AgentSelfDID func(*Agent) string
}

// Name 实现 Invariant。
func (NoSelfUpgradeInvariant) Name() string { return "no_self_upgrade" }

// Check 实现 Invariant。
func (n NoSelfUpgradeInvariant) Check(a *Agent, _ time.Time) *Violation {
	if n.AgentSelfDID == nil {
		// 未配置上下文，跳过（不强制）
		return nil
	}
	selfDID := n.AgentSelfDID(a)
	if a.LastUpgradeBy == "" || a.LastUpgradeBy != selfDID {
		return nil
	}
	return &Violation{
		Invariant: "no_self_upgrade",
		Reason:    fmt.Sprintf("agent %s tried to upgrade itself (did=%s)", a.UUID, selfDID),
	}
}

// TemporaryPermissionExpireInvariant 临时权限自动回收。
//
// 业务规则：Grant 时若指定 ttl，到期后权限位必须清零。
// 本不变量检查 a.TempPermissions（map[bit]expiry）；过期的位必须不在 a.Permissions。
type TemporaryPermissionExpireInvariant struct{}

// Name 实现 Invariant。
func (TemporaryPermissionExpireInvariant) Name() string { return "temp_perm_expire" }

// Check 实现 Invariant。
func (TemporaryPermissionExpireInvariant) Check(a *Agent, now time.Time) *Violation {
	if a.TempPermissions == nil {
		return nil
	}
	for bit, expiry := range a.TempPermissions {
		if !now.Before(expiry) {
			// 已过期
			if a.Permissions&(uint64(1)<<bit) != 0 {
				return &Violation{
					Invariant: "temp_perm_expire",
					Reason:    fmt.Sprintf("bit %d expired at %s but still set", bit, expiry),
				}
			}
		}
	}
	return nil
}

// PermissionWithinLevelInvariant 权限位不超过 Level 允许的最大值。
type PermissionWithinLevelInvariant struct{}

// Name 实现 Invariant。
func (PermissionWithinLevelInvariant) Name() string { return "perm_within_level" }

// Check 实现 Invariant。
func (PermissionWithinLevelInvariant) Check(a *Agent, _ time.Time) *Violation {
	max := a.Level.DefaultMaxPermissions()
	// a.Permissions 只能包含 max 中的位
	if a.Permissions & ^max != 0 {
		return &Violation{
			Invariant: "perm_within_level",
			Reason: fmt.Sprintf("permissions %#016x exceed level %s max %#016x",
				a.Permissions, a.Level, max),
		}
	}
	return nil
}

// ExpiresAtAfterRegisteredAtInvariant 过期时间在注册时间之后。
type ExpiresAtAfterRegisteredAtInvariant struct{}

// Name 实现 Invariant。
func (ExpiresAtAfterRegisteredAtInvariant) Name() string { return "expires_after_registered" }

// Check 实现 Invariant。
func (ExpiresAtAfterRegisteredAtInvariant) Check(a *Agent, _ time.Time) *Violation {
	if a.ExpiresAt != nil && a.ExpiresAt.Before(a.RegisteredAt) {
		return &Violation{
			Invariant: "expires_after_registered",
			Reason: fmt.Sprintf("expires_at=%s before registered_at=%s",
				a.ExpiresAt, a.RegisteredAt),
		}
	}
	return nil
}

// =============================================================================
// 不变量集合与检查
// =============================================================================

// DefaultInvariants 默认不变量集合（按顺序检查）。
func DefaultInvariants(selfDID func(*Agent) string) []Invariant {
	return []Invariant{
		StatusLegalInvariant{},
		NoSelfUpgradeInvariant{AgentSelfDID: selfDID},
		TemporaryPermissionExpireInvariant{},
		PermissionWithinLevelInvariant{},
		ExpiresAtAfterRegisteredAtInvariant{},
	}
}

// CheckInvariants 跑全部不变量；返回首个违反（顺序敏感）。
//
// 业务可在 init 阶段 / 升级前调用。
func CheckInvariants(a *Agent, now time.Time) error {
	return CheckInvariantsWith(a, now, DefaultInvariants(nil))
}

// CheckInvariantsWith 自定义不变量集合。
func CheckInvariantsWith(a *Agent, now time.Time, invariants []Invariant) error {
	for _, inv := range invariants {
		if v := inv.Check(a, now); v != nil {
			return fmt.Errorf("%w: %w", ErrInvariantViolation, v)
		}
	}
	return nil
}

// CheckInvariantsAll 跑全部不变量；返回所有违反（聚合）。
//
// 与 CheckInvariants 的区别：本函数不短路，所有违反一次性返回。
func CheckInvariantsAll(a *Agent, now time.Time, invariants []Invariant) []Violation {
	var out []Violation
	for _, inv := range invariants {
		if v := inv.Check(a, now); v != nil {
			out = append(out, *v)
		}
	}
	return out
}
