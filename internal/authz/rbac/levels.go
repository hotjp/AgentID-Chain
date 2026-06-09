// Package rbac 等级模板加载器（levels.go）。
//
// 业务场景：
//   - 等级 → 最大权限位掩码 的映射需要持久化（运营可调）
//   - 启动时从 config/agent_level.yaml 加载到内存
//   - 提供热更新（Reload）以便线上调整不需重启
//
// 数据流：
//
//	YAML 文件 → LevelsConfig（YAML schema）→ map[Level]uint64 → LevelTemplate（线程安全）
//
// 与 engine.go 的关系：
//   - engine 持有 *LevelTemplate 引用
//   - Loader 负责构造/更新该 *LevelTemplate
//   - Reload 走"读 YAML → 解析 → 原子替换"三步，外部无锁感知
package rbac

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
	"gopkg.in/yaml.v3"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrEmptyPath 配置文件路径为空。
var ErrEmptyPath = errors.New("rbac: empty config path")

// ErrLevelsConfigInvalid 等级模板配置非法。
var ErrLevelsConfigInvalid = errors.New("rbac: levels config invalid")

// =============================================================================
// LevelsConfig YAML schema
// =============================================================================

// LevelsConfig 等级模板配置（与 config/agent_level.yaml 对齐）。
//
// 字段说明：
//   - LevelMaxPermissions: 等级名（如 "basic"）→ 16 进制权限位掩码
//   - Version: schema 版本号（便于未来升级）
//   - UpdatedAt: 最后更新时间（审计）
type LevelsConfig struct {
	// Version schema 版本（当前 1）
	Version string `yaml:"version"`
	// LevelMaxPermissions 等级名 → 最大权限位掩码（hex 字符串）
	LevelMaxPermissions map[string]string `yaml:"level_max_permissions"`
	// UpdatedAt 最后更新时间
	UpdatedAt string `yaml:"updated_at"`
}

// =============================================================================
// Loader 等级模板加载器
// =============================================================================

// Loader 等级模板加载器。
//
// 线程安全：所有内部状态读写加锁。
// 加载策略：
//   - NewLoader(path) 时同步加载一次（启动期）
//   - Reload() 可热更新（运营 / SIGHUP 触发）
type Loader struct {
	mu       sync.RWMutex
	path     string
	template *LevelTemplate
	loadedAt time.Time
}

// NewLoader 构造加载器并同步加载一次。
//
// path 为 "" → 使用默认模板（不读文件）。
// 加载失败 → 返回错误（启动期应 fail-fast）。
func NewLoader(path string) (*Loader, error) {
	l := &Loader{path: path}
	if path == "" {
		l.template = NewDefaultLevelTemplate()
		l.loadedAt = time.Now()
		return l, nil
	}
	if err := l.Reload(); err != nil {
		return nil, err
	}
	return l, nil
}

// Template 返回当前等级模板（读时加锁）。
//
// Engine 构造时持有此引用；热更新由 Reload 完成。
func (l *Loader) Template() *LevelTemplate {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.template
}

// Path 返回当前配置文件路径。
func (l *Loader) Path() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.path
}

// LoadedAt 返回最后一次成功加载的时间。
func (l *Loader) LoadedAt() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.loadedAt
}

// Reload 重新从磁盘加载并原子替换 template。
//
// 失败时保留旧 template（不破坏服务可用性）。
func (l *Loader) Reload() error {
	if l.path == "" {
		// 配置文件未设置：保持默认
		l.mu.Lock()
		l.template = NewDefaultLevelTemplate()
		l.loadedAt = time.Now()
		l.mu.Unlock()
		return nil
	}

	cfg, err := LoadLevelsFromFile(l.path)
	if err != nil {
		return fmt.Errorf("rbac: reload %s: %w", l.path, err)
	}

	tpl, err := BuildTemplate(cfg)
	if err != nil {
		return fmt.Errorf("rbac: build template: %w", err)
	}

	l.mu.Lock()
	l.template = tpl
	l.loadedAt = time.Now()
	l.mu.Unlock()
	return nil
}

// SetPath 设置新的配置文件路径；下一次 Reload 时生效。
//
// 不立即加载（业务可在切换路径后显式调用 Reload）。
func (l *Loader) SetPath(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.path = path
}

// =============================================================================
// 文件加载与解析
// =============================================================================

// LoadLevelsFromFile 从 YAML 文件加载等级模板配置。
//
// 文件不存在 → 返回 os.ErrNotExist（外层可决定是否走默认）。
func LoadLevelsFromFile(path string) (*LevelsConfig, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return ParseLevels(data)
}

// ParseLevels 解析 YAML 数据为 LevelsConfig。
//
// 解析失败 → 返回 error。
func ParseLevels(data []byte) (*LevelsConfig, error) {
	var cfg LevelsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	if cfg.LevelMaxPermissions == nil {
		return &cfg, fmt.Errorf("%w: level_max_permissions is required", ErrLevelsConfigInvalid)
	}
	return &cfg, nil
}

// BuildTemplate 从 LevelsConfig 构造 LevelTemplate。
//
// 转换规则：
//   - "0xFFFF" / "ffff" / "0Xffff" 形式都接受（大小写、0x 前缀都支持）
//   - 未配置的等级 → 使用 domain.LevelType.DefaultMaxPermissions() 兜底
//   - 配置中包含非法 level 名 → 返回 ErrInvalidLevel
func BuildTemplate(cfg *LevelsConfig) (*LevelTemplate, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: nil config", ErrLevelsConfigInvalid)
	}
	m := make(map[domain.LevelType]uint64, len(cfg.LevelMaxPermissions))
	for name, hexStr := range cfg.LevelMaxPermissions {
		lvl, ok := parseLevelName(name)
		if !ok {
			return nil, fmt.Errorf("%w: unknown level %q", ErrInvalidLevel, name)
		}
		max, err := parseHexMask(hexStr)
		if err != nil {
			return nil, fmt.Errorf("rbac: level %q: %w", name, err)
		}
		m[lvl] = max
	}

	tpl, err := NewLevelTemplate(m)
	if err != nil {
		return nil, err
	}

	// 兜底：未配置的等级回退到 domain 默认（保持 Engine 行为一致）
	defaults := NewDefaultLevelTemplate()
	for lvl := domain.LevelTest; lvl <= domain.MaxLevel; lvl++ {
		if _, ok := m[lvl]; !ok {
			if err := tpl.Set(lvl, defaults.Max(lvl)); err != nil {
				return nil, err
			}
		}
	}
	return tpl, nil
}

// parseLevelName 把字符串等级名解析为 domain.LevelType。
//
// 接受 "test" / "basic" / "advanced" / "pro" / "reserved4".."reserved7" / "platform"
// 也接受数字形式 "0".."7" / "level0".."level7"。
func parseLevelName(s string) (domain.LevelType, bool) {
	switch s {
	case "test", "0", "level0":
		return domain.LevelTest, true
	case "basic", "1", "level1":
		return domain.LevelBasic, true
	case "advanced", "2", "level2":
		return domain.LevelAdvanced, true
	case "pro", "3", "level3":
		return domain.LevelPro, true
	case "reserved4", "4", "level4":
		return domain.LevelReserved4, true
	case "reserved5", "5", "level5":
		return domain.LevelReserved5, true
	case "reserved6", "6", "level6":
		return domain.LevelReserved6, true
	case "reserved7", "7", "level7", "platform":
		return domain.LevelReserved7, true
	}
	return 0, false
}

// parseHexMask 解析 16 进制字符串为 uint64。
//
// 接受形式：
//   - "0xFFFF" / "0XFFFF"
//   - "FFFF"
//   - 64 位以内任意 16 进制
func parseHexMask(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty hex string")
	}
	// 去除可选的 0x / 0X 前缀
	clean := s
	if len(clean) >= 2 && (clean[:2] == "0x" || clean[:2] == "0X") {
		clean = clean[2:]
	}
	if len(clean) == 0 {
		return 0, fmt.Errorf("empty hex after prefix")
	}
	if len(clean) > 16 {
		return 0, fmt.Errorf("hex too long: %d chars (max 16)", len(clean))
	}
	var v uint64
	for _, c := range clean {
		switch {
		case c >= '0' && c <= '9':
			v = v<<4 | uint64(c-'0')
		case c >= 'a' && c <= 'f':
			v = v<<4 | uint64(c-'a'+10)
		case c >= 'A' && c <= 'F':
			v = v<<4 | uint64(c-'A'+10)
		default:
			return 0, fmt.Errorf("invalid hex char: %q", c)
		}
	}
	return v, nil
}
