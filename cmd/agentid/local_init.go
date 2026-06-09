// Package main local init 子命令实现。
//
// 用途：在本地文件系统上初始化 AgentID-Chain 的 CLI 工作目录（默认 ~/.agentid/），
// 生成默认 config.yaml 和数据目录（chain-data / outbox / cache）。
//
// 用法：
//
//	agentid local init
//	agentid local init --home /custom/path
//	agentid local init --force
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var localInitFlagVals = struct {
	home  string
	force bool
}{}

var localInitCmdImpl = &cobra.Command{
	Use:   "init",
	Short: "初始化本地工作目录（~/.agentid/）",
	Long: `初始化 AgentID-Chain 本地工作目录。

默认在 $HOME/.agentid/ 下创建：
  · config.yaml         CLI 配置文件
  · data/               本地链适配器状态目录
  · outbox/             事件 outbox 落盘目录
  · cache/              临时缓存目录

Examples:
  agentid local init
  agentid local init --home /opt/agentid-data
  agentid local init --force   # 覆盖已存在的 config.yaml`,
	RunE: runLocalInit,
}

func init() {
	localInitCmdImpl.Flags().StringVar(&localInitFlagVals.home, "home", "", "工作目录（默认 $HOME/.agentid）")
	localInitCmdImpl.Flags().BoolVar(&localInitFlagVals.force, "force", false, "覆盖已存在的 config.yaml")
}

// localParentCmd 见 local.go（父命令 local）。
var localParentCmd = &cobra.Command{
	Use:   "local",
	Short: "本地模式相关子命令",
}

func init() {
	localParentCmd.AddCommand(localInitCmdImpl)
}

func runLocalInit(_ *cobra.Command, _ []string) error {
	home, err := resolveLocalHome(localInitFlagVals.home)
	if err != nil {
		return err
	}

	dirs := []string{
		home,
		filepath.Join(home, "data"),
		filepath.Join(home, "outbox"),
		filepath.Join(home, "cache"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	cfgPath := filepath.Join(home, "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil && !localInitFlagVals.force {
		fmt.Fprintf(os.Stderr, "init: %s already exists (use --force to overwrite)\n", cfgPath)
	} else {
		if err := writeDefaultConfig(cfgPath); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "init: wrote %s\n", cfgPath)
	}

	// 写 init 标记
	marker := filepath.Join(home, "data", ".initialized")
	_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)), 0o644)

	// 输出 JSON 摘要
	summary := map[string]any{
		"ok":   true,
		"home": home,
		"paths": map[string]string{
			"config": cfgPath,
			"data":   filepath.Join(home, "data"),
			"outbox": filepath.Join(home, "outbox"),
			"cache":  filepath.Join(home, "cache"),
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(summary)
}

// resolveLocalHome 解析工作目录。
func resolveLocalHome(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve $HOME: %w", err)
	}
	return filepath.Join(home, ".agentid"), nil
}

// writeDefaultConfig 写默认配置。
func writeDefaultConfig(path string) error {
	body := `# AgentID-Chain CLI 配置（agentid local init 生成）
mode: local              # local | remote
backend: mock            # local | mock | onchain | hybrid（仅 mode=local 生效）
gateway: http://localhost:8080
api_key: ""              # 仅 mode=remote 生效
output: json             # json | table | yaml
timeout_seconds: 30
`
	return os.WriteFile(path, []byte(body), 0o600)
}
