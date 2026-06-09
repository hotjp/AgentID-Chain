// Package domain 核心实体定义。
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrInvalidUUID UUID 格式不合法。
var ErrInvalidUUID = errors.New("domain: invalid UUID")

// ErrInvalidOwner Owner 标识不合法。
var ErrInvalidOwner = errors.New("domain: invalid owner")

// ErrInvalidLevel Level 越界（不在 [0, MaxLevel] 范围）。
var ErrInvalidLevel = errors.New("domain: invalid level")

// ErrInvalidTransition 状态机不允许的转换。
var ErrInvalidTransition = errors.New("domain: invalid state transition")

// ErrPermissionExceedsLevel 权限位超出 Level 允许的最大值。
var ErrPermissionExceedsLevel = errors.New("domain: permission exceeds level")

// =============================================================================
// UUID 验证
// =============================================================================

// uuidPattern 接受 v4 (xxxxxx) 或 v7 (xxxxxxxx) hex 字符串，长度 32-36（带/不带连字符）。
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$|^[0-9a-fA-F]{32}$`)

// UUID 强类型别名（防止与普通 string 混用）。
type UUID string

// String 序列化回原始字符串。
func (u UUID) String() string { return string(u) }

// IsZero 判断是否为空 UUID。
func (u UUID) IsZero() bool { return strings.TrimSpace(string(u)) == "" }

// Validate 校验 UUID 格式（v4 / v7）。
func (u UUID) Validate() error {
	if u.IsZero() {
		return ErrInvalidUUID
	}
	if !uuidPattern.MatchString(string(u)) {
		return fmt.Errorf("%w: %s", ErrInvalidUUID, string(u))
	}
	return nil
}

// Short 取前 8 字符作为可读缩写（用于日志/UI）。
func (u UUID) Short() string {
	s := strings.ReplaceAll(string(u), "-", "")
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

// Fingerprint 基于 UUID 计算确定性指纹（用于审计追溯，不暴露原值）。
func (u UUID) Fingerprint() string {
	sum := sha256.Sum256([]byte(u))
	return hex.EncodeToString(sum[:8])
}

// =============================================================================
// Level 等级
// =============================================================================

// LevelType 等级类型（0-7，参考 docs §2.2 模块职责表）。
type LevelType uint8

// 等级常量。
const (
	LevelTest      LevelType = 0 // 测试 Agent
	LevelBasic     LevelType = 1 // 普通 Agent
	LevelAdvanced  LevelType = 2 // 高级 Agent
	LevelPro       LevelType = 3 // 专业 Agent
	LevelReserved4 LevelType = 4 // 预留
	LevelReserved5 LevelType = 5 // 预留
	LevelReserved6 LevelType = 6 // 预留
	LevelReserved7 LevelType = 7 // 系统/平台保留
)

// MaxLevel 最大 Level 值。
const MaxLevel = LevelReserved7

// IsValid 校验 Level 是否在合法区间。
func (l LevelType) IsValid() bool { return l <= MaxLevel }

// String 返回等级的可读名。
func (l LevelType) String() string {
	switch l {
	case LevelTest:
		return "test"
	case LevelBasic:
		return "basic"
	case LevelAdvanced:
		return "advanced"
	case LevelPro:
		return "pro"
	case LevelReserved4:
		return "reserved4"
	case LevelReserved5:
		return "reserved5"
	case LevelReserved6:
		return "reserved6"
	case LevelReserved7:
		return "platform"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(l))
	}
}

// DefaultMaxPermissions 返回 Level 允许的最大权限位。
// v2.0.1 简化：每级 16 位，0-3 共 64 位。
func (l LevelType) DefaultMaxPermissions() uint64 {
	if l > MaxLevel {
		return 0
	}
	return uint64(1)<<uint(l*16+16) - 1
}

// =============================================================================
// Owner 所有者标识
// =============================================================================

// ownerPattern: 小写字母/数字/_/-, 长度 3-64。
var ownerPattern = regexp.MustCompile(`^[a-z0-9_-]{3,64}$`)

// Owner 强类型别名。
type Owner string

// String 序列化。
func (o Owner) String() string { return string(o) }

// Validate 校验 Owner 格式。
func (o Owner) Validate() error {
	if !ownerPattern.MatchString(string(o)) {
		return fmt.Errorf("%w: %s", ErrInvalidOwner, string(o))
	}
	return nil
}

// =============================================================================
// Agent 实体
// =============================================================================

// Agent 领域实体。
//
// 设计原则：
//   - 不可变优先：通过 New* 构造；变更通过状态机/事件
//   - 零依赖：仅含值类型与时间戳
//   - 状态由 State 字段显式表达，不靠零值隐式
type Agent struct {
	// 身份
	UUID  UUID
	Owner Owner
	Level LevelType

	// 状态
	State     AgentState
	BanReason string

	// 凭证
	PublicKey string // Ed25519 公钥（base64 / hex）
	TxHash    string // 链上注册时填写，链下模式可空
	Signature string // 注册签名（base64）

	// 权限
	Permissions uint64

	// 时间
	RegisteredAt time.Time
	UpdatedAt    time.Time
	BannedAt     *time.Time // 指针，封禁时填写；解封置 nil
	ExpiresAt    *time.Time // 可选过期时间
}

// NewAgent 构造一个新 Agent（不校验完整状态机，仅做基础校验）。
func NewAgent(uuid UUID, owner Owner, level LevelType, publicKey string, now time.Time) (*Agent, error) {
	if err := uuid.Validate(); err != nil {
		return nil, err
	}
	if err := owner.Validate(); err != nil {
		return nil, err
	}
	if !level.IsValid() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(level))
	}
	if publicKey == "" {
		return nil, errors.New("domain: empty public key")
	}
	return &Agent{
		UUID:         uuid,
		Owner:        owner,
		Level:        level,
		State:        StateRegistered,
		PublicKey:    publicKey,
		Permissions:  0,
		RegisteredAt: now,
		UpdatedAt:    now,
	}, nil
}

// Activate 将 Agent 从 Registered 推进到 Active（"激活"事件）。
func (a *Agent) Activate(now time.Time) error {
	if a.State != StateRegistered {
		return fmt.Errorf("%w: %s → active", ErrInvalidTransition, a.State)
	}
	a.State = StateActive
	a.UpdatedAt = now
	return nil
}

// Ban 封禁 Agent（任意非 Banned/Unregistered 状态都可进入 Banned）。
func (a *Agent) Ban(reason string, now time.Time) error {
	if a.State == StateBanned || a.State == StateUnregistered {
		return fmt.Errorf("%w: %s → banned", ErrInvalidTransition, a.State)
	}
	if reason == "" {
		return errors.New("domain: empty ban reason")
	}
	a.State = StateBanned
	a.BanReason = reason
	a.BannedAt = &now
	a.UpdatedAt = now
	return nil
}

// Unban 解封（仅从 Banned 可恢复）。
func (a *Agent) Unban(now time.Time) error {
	if a.State != StateBanned {
		return fmt.Errorf("%w: %s → active", ErrInvalidTransition, a.State)
	}
	a.State = StateActive
	a.BanReason = ""
	a.BannedAt = nil
	a.UpdatedAt = now
	return nil
}

// Upgrade 升级 Level（仅允许单步升级，不允许跳级）。
func (a *Agent) Upgrade(newLevel LevelType, now time.Time) error {
	if !newLevel.IsValid() {
		return fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(newLevel))
	}
	if newLevel <= a.Level {
		return fmt.Errorf("%w: %d → %d", ErrInvalidLevel, uint8(a.Level), uint8(newLevel))
	}
	if newLevel != a.Level+1 {
		return fmt.Errorf("%w: must upgrade by exactly 1 level", ErrInvalidLevel)
	}
	a.Level = newLevel
	a.UpdatedAt = now
	return nil
}

// Grant 授予权限位（受 Level 上限约束）。
func (a *Agent) Grant(bit uint) error {
	if uint64(bit) > 63 {
		return errors.New("domain: bit out of range [0,63]")
	}
	if uint64(1)<<bit&a.Level.DefaultMaxPermissions() == 0 {
		return fmt.Errorf("%w: bit %d exceeds level %s", ErrPermissionExceedsLevel, bit, a.Level)
	}
	a.Permissions |= uint64(1) << bit
	return nil
}

// Revoke 撤销权限位。
func (a *Agent) Revoke(bit uint) error {
	if uint64(bit) > 63 {
		return errors.New("domain: bit out of range [0,63]")
	}
	a.Permissions &^= uint64(1) << bit
	return nil
}

// HasPermission 查询是否拥有某权限位。
func (a *Agent) HasPermission(bit uint) bool {
	if uint64(bit) > 63 {
		return false
	}
	return a.Permissions&(uint64(1)<<bit) != 0
}

// IsActive 业务可用性判断（Active 且未过期）。
func (a *Agent) IsActive(now time.Time) bool {
	if a.State != StateActive {
		return false
	}
	if a.ExpiresAt != nil && now.After(*a.ExpiresAt) {
		return false
	}
	return true
}
