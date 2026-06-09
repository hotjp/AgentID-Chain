// Package config 加载逻辑（YAML + env + flag）。
package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// koanf 实例（包级唯一）
var k = koanf.New(".")

const (
	// EnvPrefix 环境变量前缀
	EnvPrefix = "AGENTID_"
	// EnvDelim 分隔符（AGENTID_DB_DSN → db.dsn）
	EnvDelim = "."
)

// Load 按优先级加载配置：defaults → YAML → env。
// path 为空时跳过 YAML。
func Load(path string) (*Config, error) {
	// 1. defaults
	if err := k.Load(structs.Provider(New(), "koanf"), nil); err != nil {
		return nil, fmt.Errorf("load defaults: %w", err)
	}

	// 2. YAML（可选）
	if path != "" {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("load YAML %s: %w", path, err)
		}
	}

	// 3. env（AGENTID_ 前缀）
	if err := k.Load(env.Provider(".", env.Opt{
		Prefix: EnvPrefix,
		TransformFunc: func(k, v string) (string, any) {
			// AGENTID_DB_DSN → db.dsn
			k = strings.ToLower(strings.TrimPrefix(k, EnvPrefix))
			k = strings.ReplaceAll(k, "_", EnvDelim)
			return k, v
		},
	}), nil); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
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
	return k.Set(key, val)
}

// Snapshot 返回当前 koanf 序列化为 map（用于 debug 打印）。
func Snapshot() map[string]any {
	return k.All()
}
