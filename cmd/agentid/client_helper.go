// Package main CLI 命令共享的 client 构造工具。
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/agentid-chain/agentid-chain/internal/cli"
	"github.com/agentid-chain/agentid-chain/internal/cli/output"
)

// buildClientOpts 客户端构造选项。
type buildClientOpts struct {
	// ConfigPath CLI 配置文件路径。
	ConfigPath string
	// Gateway 覆盖 config.gateway。
	Gateway string
	// APIKey 覆盖 config.api_key。
	APIKey string
	// Mode 覆盖 config.mode (local / remote)。
	Mode string
	// Timeout 覆盖 config.timeout_seconds。
	Timeout int
	// Backend 覆盖 config.backend（仅 Mode=local）。
	Backend string
}

// buildClient 构造 CLI 客户端（按优先级：flag > config > default）。
func buildClient(_ context.Context, opts buildClientOpts) (*cli.Client, error) {
	// 1. 读 config
	cfg, err := cli.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load cli config: %w", err)
	}
	// 旧版 LoadConfig 已经 ApplyDefaults()，再调一次防 flag 写入后空字段

	// 2. flag 覆盖
	if opts.Gateway != "" {
		cfg.Gateway = opts.Gateway
	}
	if opts.APIKey != "" {
		cfg.APIKey = opts.APIKey
	}
	if opts.Mode != "" {
		cfg.Mode = cli.Mode(opts.Mode)
	}
	if opts.Timeout > 0 {
		cfg.TimeoutSeconds = opts.Timeout
	}
	if opts.Backend != "" {
		cfg.Backend = opts.Backend
	}
	cfg.ApplyDefaults()

	// 3. 构造
	return cli.NewClient(cfg)
}

// chooseFormat 选 format（flag > config > default）。
func chooseFormat(c *cli.Client, flag string) string {
	if flag != "" {
		return flag
	}
	return c.Config().Output
}

// printAgentInfo 通用的 agent info 输出辅助。
func printAgentInfo(c *cli.Client, flag string, info *cli.AgentInfo) error {
	format := output.ParseFormat(chooseFormat(c, flag))
	return output.Print(os.Stdout, format, info)
}

// printLogs 通用的 logs 输出。
func printLogs(c *cli.Client, flag string, logs []cli.ChangeLog) error {
	format := output.ParseFormat(chooseFormat(c, flag))
	return output.PrintList(os.Stdout, format, logs, []string{
		"action", "actor", "old_value", "new_value", "reason", "occurred_at",
	})
}

// prettyWriter 返回 stdout 写入器（单一出口，便于未来 redirect / 测试）。
func prettyWriter() interface {
	Write(p []byte) (int, error)
} {
	return os.Stdout
}
