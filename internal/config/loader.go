// Package config — koanf 加载器（defaults → YAML → env）。
//
// 加载优先级（与 docs/AgentID-Chain-技术文档-v2.0.1.md §5.3 对齐）：
//  1. 内置 defaults（New()）
//  2. YAML 文件（--config / -c 指定；可选）
//  3. 环境变量（AGENTID_ 前缀，区分大小写映射，例 AGENTID_DB_DSN → db.dsn）
//  4. 命令行 flag（最高优先级；通过 Override() 设置）
//
// 所有路径必须以本包内 Koanf 实例操作为准，禁止外部持有 Koanf 实例。
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// koanf 实例（包级唯一，单例模式）
var (
	instance *koanf.Koanf
	once     sync.Once
)

// koanfInstance 懒加载单例 koanf 实例。
func koanfInstance() *koanf.Koanf {
	once.Do(func() {
		instance = koanf.New(".")
	})
	return instance
}

// resetKoanf 重置单例（仅测试使用）。
func resetKoanf() {
	instance = nil
	once = sync.Once{}
}

// EnvPrefix 环境变量前缀。
const EnvPrefix = "AGENTID_"

// EnvDelim 分隔符（AGENTID_DB_DSN → db.dsn）。
const EnvDelim = "."

// LoadResult 加载结果（携带 source 调试信息）。
type LoadResult struct {
	Cfg          *Config
	LoadedFrom   string   // 实际加载的 YAML 路径（空 = 未加载）
	EnvCount     int      // 应用的环境变量数
	DefaultsUsed []string // 使用了默认值的 key（只读）
}

// MustLoad 加载配置（加载失败直接 panic；用于 main 启动）。
// path 为空字符串时跳过 YAML。
func MustLoad(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		panic(fmt.Errorf("config: must load failed: %w", err))
	}
	return cfg
}

// Load 按优先级加载配置：defaults → YAML → env → flag。
// path 为空时跳过 YAML。
func Load(path string) (*Config, error) {
	return LoadWith(path, nil)
}

// LoadWith 与 Load 类似，但接受额外的 flagOverrides（key=value 形式）。
// flagOverrides 中的每个 key 会通过 koanf.Set 设置（最高优先级）。
func LoadWith(path string, flagOverrides []string) (*Config, error) {
	// 每次 Load 都重置 koanf 实例（避免污染）
	resetKoanf()
	k := koanfInstance()

	// 1. defaults
	if err := k.Load(structs.Provider(New(), "koanf"), nil); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}

	// 2. YAML（可选）
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("config: load YAML %s: %w", path, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("config: stat YAML %s: %w", path, err)
		}
		// 文件不存在时静默忽略（让 defaults / env 生效）
	}

	// 3. env（AGENTID_ 前缀）
	envCount := 0
	envProvider := env.Provider(".", env.Opt{
		Prefix: EnvPrefix,
		TransformFunc: func(k, v string) (string, any) {
			k = strings.ToLower(strings.TrimPrefix(k, EnvPrefix))
			k = strings.ReplaceAll(k, "_", EnvDelim)
			envCount++
			return k, v
		},
	})
	if err := k.Load(envProvider, nil); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	// 4. flag overrides（最高优先级）
	for _, override := range flagOverrides {
		if err := LoadString(override); err != nil {
			return nil, fmt.Errorf("config: apply override %q: %w", override, err)
		}
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &cfg, nil
}

// LoadString 解析 `key=value` 形式（用于命令行 flag 覆盖；最高优先级）。
func LoadString(s string) error {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid override: %s (want key=value)", s)
	}
	key := strings.ToLower(parts[0])
	val := parts[1]
	return koanfInstance().Set(key, val)
}

// Snapshot 返回当前 koanf 序列化为 map（用于 debug 打印）。
func Snapshot() map[string]any {
	return koanfInstance().All()
}

// LoadFromEnv 只从环境变量加载（跳过 defaults / YAML / flag）。
// 用于：单元测试、动态配置热更新。
func LoadFromEnv() (*Config, error) {
	resetKoanf()
	k := koanfInstance()

	// 仅 defaults + env
	if err := k.Load(structs.Provider(New(), "koanf"), nil); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}
	envProvider := env.Provider(".", env.Opt{
		Prefix: EnvPrefix,
		TransformFunc: func(k, v string) (string, any) {
			k = strings.ToLower(strings.TrimPrefix(k, EnvPrefix))
			k = strings.ReplaceAll(k, "_", EnvDelim)
			return k, v
		},
	})
	if err := k.Load(envProvider, nil); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	return &cfg, nil
}
