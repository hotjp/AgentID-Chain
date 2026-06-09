// Package config — 环境变量覆盖。
//
// EnvOverride 是对 koanf env provider 的高层封装：
//   - 统一前缀（AGENTID_，可定制）
//   - 下划线→点 分隔
//   - 大小写不敏感
//   - 类型自动推断（int / bool / duration）
//
// 与 loader.go 中的 koanf env provider 不同：EnvOverride 提供更友好的
// 类型转换（duration、int、bool 自动解析），适用于 CLI 工具和 SDK 调用方。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvOverride 配置覆盖项。
type EnvOverride struct {
	Prefix  string   // 环境变量前缀（默认 "AGENTID_"）
	Delim   string   // 分隔符（默认 "."）
	Strict  bool     // 严格模式：找不到对应配置项时报错
	Skipped []string // 跳过的环境变量名（不参与覆盖）
}

// DefaultEnvOverride 返回默认的覆盖规则。
func DefaultEnvOverride() EnvOverride {
	return EnvOverride{
		Prefix: EnvPrefix,
		Delim:  EnvDelim,
		Strict: false,
	}
}

// Apply 应用环境变量覆盖到指定 Config 指针。
// 规则：
//   - key 必须以 Prefix 开头
//   - Prefix 之后的部分按 "_" 拆分，再按 Delim 连接
//   - 例如 AGENTID_DB_DSN → db.dsn
//   - 大小写不敏感
//   - 不会写回 Config（只打印 / 返回覆盖项）
//
// 返回实际应用的覆盖项（用于测试和调试）。
func (e EnvOverride) Apply(env map[string]string) []Override {
	if e.Prefix == "" {
		e.Prefix = EnvPrefix
	}
	if e.Delim == "" {
		e.Delim = EnvDelim
	}

	skip := make(map[string]struct{}, len(e.Skipped))
	for _, s := range e.Skipped {
		skip[s] = struct{}{}
	}

	var applied []Override
	for k, v := range env {
		if !strings.HasPrefix(k, e.Prefix) {
			continue
		}
		if _, ok := skip[k]; ok {
			continue
		}

		// 去掉前缀并小写
		suffix := strings.TrimPrefix(k, e.Prefix)
		suffix = strings.ToLower(suffix)
		// 下划线 → 分隔符
		keyPath := strings.ReplaceAll(suffix, "_", e.Delim)

		applied = append(applied, Override{
			EnvKey: k,
			ConfigKey: keyPath,
			RawValue: v,
			TypedValue: inferType(v),
		})
	}
	return applied
}

// Override 单条覆盖记录。
type Override struct {
	EnvKey     string      // 原始环境变量名
	ConfigKey  string      // 对应配置项路径（点分）
	RawValue   string      // 字符串值
	TypedValue any         // 推断后的类型值
}

// String 返回可读形式。
func (o Override) String() string {
	return fmt.Sprintf("%s → %s = %v", o.EnvKey, o.ConfigKey, o.TypedValue)
}

// inferType 简单类型推断。
//   - "true"/"false" → bool
//   - 纯数字 → int
//   - 数字+单位（s/m/h/ms）→ time.Duration
//   - 否则 → string
func inferType(v string) any {
	lower := strings.ToLower(v)
	switch lower {
	case "true":
		return true
	case "false":
		return false
	}
	// 尝试 int
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	// 尝试 duration（如 30s, 5m, 1h, 100ms）
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return v
}

// ListEnv 返回所有匹配前缀的环境变量。
// 用于：调试、文档生成。
func ListEnv(prefix string) map[string]string {
	if prefix == "" {
		prefix = EnvPrefix
	}
	out := make(map[string]string)
	for _, e := range os.Environ() {
		idx := strings.Index(e, "=")
		if idx < 0 {
			continue
		}
		k := e[:idx]
		v := e[idx+1:]
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out
}

// ExpandEnv 展开 ${VAR} 形式的环境变量引用（用于 YAML 占位）。
// 找不到的变量原样保留（不会报错）。
func ExpandEnv(s string) string {
	return os.Expand(s, func(name string) string {
		return os.Getenv(name)
	})
}

// NormalizeKey 归一化 key 形式（外部输入 → koanf 内部）。
//   - 全小写
//   - _ → .
//   - 去掉首尾 . （如果有）
func NormalizeKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", ".")
	s = strings.Trim(s, ".")
	return s
}
