// Package main config 子命令实现。
//
// 用途：查看与编辑 CLI 配置文件（~/.agentid/config.yaml）。
//
// 子命令：
//
//	agentid config show             打印当前生效配置
//	agentid config path             打印配置文件路径
//	agentid config set <key> <val>  设置某个字段
//	agentid config reset            恢复默认配置
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agentid-chain/agentid-chain/internal/cli"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configFlagVals = struct {
	configPath string
}{}

var configCmdImpl = &cobra.Command{
	Use:   "config",
	Short: "查看 / 编辑 CLI 配置（~/.agentid/config.yaml）",
	Long: `查看与编辑 CLI 配置文件。

Examples:
  agentid config show
  agentid config path
  agentid config set mode remote
  agentid config set gateway http://localhost:8080
  agentid config set timeout_seconds 60
  agentid config reset`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "打印当前生效配置",
	RunE:  runConfigShow,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "打印配置文件路径",
	RunE:  runConfigPath,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置某个字段",
	Long: `支持的字段：mode / gateway / api_key / backend / output / timeout_seconds
示例：
  agentid config set mode remote
  agentid config set timeout_seconds 60`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "恢复默认配置（备份旧文件为 .bak）",
	RunE:  runConfigReset,
}

func init() {
	configCmdImpl.PersistentFlags().StringVar(&configFlagVals.configPath, "config", "", "配置文件路径")

	configCmdImpl.AddCommand(configShowCmd)
	configCmdImpl.AddCommand(configPathCmd)
	configCmdImpl.AddCommand(configSetCmd)
	configCmdImpl.AddCommand(configResetCmd)
}

func runConfigShow(_ *cobra.Command, _ []string) error {
	cfg, err := cli.LoadConfig(configFlagVals.configPath)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	fmt.Fprint(os.Stdout, string(data))
	return nil
}

func runConfigPath(_ *cobra.Command, _ []string) error {
	path := configFlagVals.configPath
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".agentid", "config.yaml")
	}
	fmt.Fprintln(os.Stdout, path)
	return nil
}

func runConfigSet(_ *cobra.Command, args []string) error {
	path, err := resolveConfigPath(configFlagVals.configPath)
	if err != nil {
		return err
	}
	if err := ensureConfigFile(path); err != nil {
		return err
	}
	cfg, err := cli.LoadConfig(path)
	if err != nil {
		return err
	}
	if err := applyConfigSet(cfg, args[0], args[1]); err != nil {
		return err
	}
	return writeConfigYAML(path, cfg)
}

func runConfigReset(_ *cobra.Command, _ []string) error {
	path, err := resolveConfigPath(configFlagVals.configPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.Rename(path, path+".bak")
	}
	return writeConfigYAML(path, cli.DefaultConfig())
}

// applyConfigSet 修改 Config 字段。
func applyConfigSet(cfg *cli.Config, key, val string) error {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "mode":
		v := strings.ToLower(val)
		if v != "local" && v != "remote" {
			return fmt.Errorf("mode must be local|remote, got %q", val)
		}
		cfg.Mode = cli.Mode(v)
	case "gateway":
		cfg.Gateway = val
	case "api_key", "api-key", "apikey":
		cfg.APIKey = val
	case "backend":
		v := strings.ToLower(val)
		if v != "local" && v != "mock" && v != "onchain" && v != "hybrid" {
			return fmt.Errorf("backend must be local|mock|onchain|hybrid, got %q", val)
		}
		cfg.Backend = v
	case "output":
		v := strings.ToLower(val)
		if v != "json" && v != "table" && v != "yaml" {
			return fmt.Errorf("output must be json|table|yaml, got %q", val)
		}
		cfg.Output = v
	case "timeout_seconds", "timeout-seconds", "timeout":
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("timeout_seconds must be positive int, got %q", val)
		}
		cfg.TimeoutSeconds = n
	default:
		return fmt.Errorf("unknown key %q (allowed: mode, gateway, api_key, backend, output, timeout_seconds)", key)
	}
	cfg.ApplyDefaults()
	return nil
}

// resolveConfigPath 解析 --config 或默认路径。
func resolveConfigPath(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve $HOME: %w", err)
	}
	return filepath.Join(home, ".agentid", "config.yaml"), nil
}

// ensureConfigFile 若不存在则创建默认配置。
func ensureConfigFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return writeConfigYAML(path, cli.DefaultConfig())
}

// writeConfigYAML 写 YAML 文件（权限 0600）。
func writeConfigYAML(path string, cfg *cli.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}
