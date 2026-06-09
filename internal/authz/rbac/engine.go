// Package rbac 是 L3 鉴权层的 RBAC（Role-Based Access Control）引擎。
//
// 设计要点：
//   - 纯位掩码校验，无 IO
//   - 等级 → 最大权限位掩码 通过 LevelTemplate 注入
//   - 默认值由 domain.LevelType.DefaultMaxPermissions 计算
//   - Fail Fast：任何超出等级允许范围的位都拒绝
//
// 业务规则（v2.0.1 §3.3.3）：
//   - 每个 Level 拥有最大权限位掩码（uint64）
//   - Agent 的 Permissions ⊆ level.DefaultMaxPermissions
//   - 检查单个 bit：Check(bit, level) → level 默认 max 包含该 bit？
//   - 检查整个位掩码：CheckMask(perms, level) → (perms & ^max) == 0？
//
// 与 L2 domain 的关系：
//   - LevelType 枚举在 domain
//   - DefaultMaxPermissions 是 domain 的事实来源
//   - rbac 只做"掩码校验"；业务判定交给上层
package rbac

import (
	"errors"
	"fmt"
	"sync"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrInvalidLevel 等级值不合法（不在 [0, MaxLevel] 范围）。
var ErrInvalidLevel = errors.New("rbac: invalid level")

// ErrTemplateNotSet 等级模板未配置（Engine 未注入）。
var ErrTemplateNotSet = errors.New("rbac: level template not set")

// ErrBitOutOfRange 位索引超出 [0, 63] 范围。
var ErrBitOutOfRange = errors.New("rbac: permission bit out of range")

// =============================================================================
// LevelTemplate 等级 → 最大权限位掩码
// =============================================================================

// LevelTemplate 等级模板：定义每个 Level 允许的最大权限位掩码。
//
// 数据驱动：可从 YAML / DB 加载；Engine 接收后可热替换。
// 默认情况下，模板应与 domain.LevelType.DefaultMaxPermissions 保持一致。
type LevelTemplate struct {
	mu     sync.RWMutex
	maxMap map[domain.LevelType]uint64
}

// NewLevelTemplate 从给定的映射构造模板。
//
// 内部会做完整性校验：
//   - level 必须在 [LevelTest, MaxLevel] 范围内
//   - max 必须是连续的（从 bit 0 起连续 1；不强制但建议）
func NewLevelTemplate(m map[domain.LevelType]uint64) (*LevelTemplate, error) {
	t := &LevelTemplate{maxMap: make(map[domain.LevelType]uint64, len(m))}
	for lvl, max := range m {
		if !lvl.IsValid() {
			return nil, fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(lvl))
		}
		t.maxMap[lvl] = max
	}
	return t, nil
}

// NewDefaultLevelTemplate 返回默认模板（与 domain.DefaultMaxPermissions 对齐）。
//
// 默认规则（v2.0.1）：
//   - LevelTest:      0x000000000000FFFF  (bits [0, 16))
//   - LevelBasic:     0x00000000FFFFFFFF  (bits [0, 32))
//   - LevelAdvanced:  0x0000FFFFFFFFFFFF  (bits [0, 48))
//   - LevelPro:       0xFFFFFFFFFFFFFFFF  (bits [0, 64))
//   - LevelReserved4-7: 全部填满（platform 用）
func NewDefaultLevelTemplate() *LevelTemplate {
	m := map[domain.LevelType]uint64{}
	for lvl := domain.LevelTest; lvl <= domain.MaxLevel; lvl++ {
		m[lvl] = lvl.DefaultMaxPermissions()
	}
	return &LevelTemplate{maxMap: m}
}

// Max 查表：返回 level 允许的最大权限位掩码。
//
// 未配置时回退到 domain.LevelType.DefaultMaxPermissions（兜底）。
func (t *LevelTemplate) Max(level domain.LevelType) uint64 {
	if t == nil {
		return level.DefaultMaxPermissions()
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if max, ok := t.maxMap[level]; ok {
		return max
	}
	// 兜底：使用 domain 默认值
	return level.DefaultMaxPermissions()
}

// Set 热替换某个等级的最大权限位掩码（写时加锁）。
func (t *LevelTemplate) Set(level domain.LevelType, max uint64) error {
	if !level.IsValid() {
		return fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(level))
	}
	if t == nil {
		return ErrTemplateNotSet
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maxMap[level] = max
	return nil
}

// Snapshot 返回模板的不可变副本（用于审计/调试）。
func (t *LevelTemplate) Snapshot() map[domain.LevelType]uint64 {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[domain.LevelType]uint64, len(t.maxMap))
	for k, v := range t.maxMap {
		out[k] = v
	}
	return out
}

// =============================================================================
// Engine RBAC 引擎
// =============================================================================

// Engine RBAC 决策引擎。
//
// 决策类型：
//   - Check(bit, level)         : 检查单个 bit 是否在 level 允许范围
//   - CheckMask(perms, level)   : 检查整个位掩码是否 ⊆ level.max
//   - AllowedBits(level)        : 返回 level 允许的所有 bit 集合
//
// 线程安全：Engine 持有 LevelTemplate 引用（template 自身已加锁）。
type Engine struct {
	template *LevelTemplate
}

// NewEngine 构造 RBAC 引擎。
//
// template 可为 nil（nil-safe：使用 domain 默认 max）。
func NewEngine(template *LevelTemplate) *Engine {
	return &Engine{template: template}
}

// Template 返回当前模板引用（用于热更新 / 调试）。
func (e *Engine) Template() *LevelTemplate {
	return e.template
}

// Check 检查单个 bit 是否在 level 允许范围。
//
// bit 范围：[0, 63]；超过范围返回 ErrBitOutOfRange。
// 未配置 level 时回退到 domain 默认 max。
func (e *Engine) Check(bit uint, level domain.LevelType) (bool, error) {
	if uint64(bit) > 63 {
		return false, fmt.Errorf("%w: %d", ErrBitOutOfRange, bit)
	}
	if !level.IsValid() {
		return false, fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(level))
	}
	max := e.maxFor(level)
	return (uint64(1)<<bit)&max != 0, nil
}

// MustCheck 与 Check 行为相同，错误时 panic（仅用于配置驱动的硬编码）。
func (e *Engine) MustCheck(bit uint, level domain.LevelType) bool {
	ok, err := e.Check(bit, level)
	if err != nil {
		panic(err)
	}
	return ok
}

// CheckMask 检查整个权限位掩码是否完全在 level.max 范围内。
//
// 规则：(perms & ^max) == 0 → 允许；否则拒绝。
// 用于"Agent 当前权限是否在等级允许范围内"的判定。
func (e *Engine) CheckMask(perms uint64, level domain.LevelType) (bool, error) {
	if !level.IsValid() {
		return false, fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(level))
	}
	max := e.maxFor(level)
	return perms&^max == 0, nil
}

// AllowedBits 返回 level 允许的所有 bit 位置集合（倒序：[63, 0]）。
//
// 用途：UI 展示"该等级可以授予哪些权限"。
func (e *Engine) AllowedBits(level domain.LevelType) ([]uint, error) {
	if !level.IsValid() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidLevel, uint8(level))
	}
	max := e.maxFor(level)
	out := make([]uint, 0, 64)
	for i := uint(63); ; i-- {
		if max&(uint64(1)<<i) != 0 {
			out = append(out, i)
		}
		if i == 0 {
			break
		}
	}
	return out, nil
}

// MaxPermissions 返回 level 的最大权限位掩码。
func (e *Engine) MaxPermissions(level domain.LevelType) uint64 {
	return e.maxFor(level)
}

// HasAny 检查 perms 中是否存在 level 允许的位。
func (e *Engine) HasAny(perms uint64, level domain.LevelType) bool {
	max := e.maxFor(level)
	return perms&max != 0
}

// HasAll 检查 perms 中是否所有位都在 level 允许范围（即 perms ⊆ max）。
func (e *Engine) HasAll(perms uint64, level domain.LevelType) bool {
	max := e.maxFor(level)
	return perms&^max == 0
}

// =============================================================================
// 私有方法
// =============================================================================

// maxFor 查表：取 level 的 max。
func (e *Engine) maxFor(level domain.LevelType) uint64 {
	if e == nil || e.template == nil {
		return level.DefaultMaxPermissions()
	}
	return e.template.Max(level)
}
