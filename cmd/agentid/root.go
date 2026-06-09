// Package main 是 AgentID-Chain 的 CLI 入口。
//
// 使用 cobra 框架构建子命令树：
//   - serve    起 gateway / auth-center / tag-sense 服务（按 --role）
//   - register 注册新 Agent
//   - info     查询 Agent 信息
//   - upgrade  升级 Agent Level
//   - ban      封禁 Agent
//   - unban    解封 Agent
//   - unregister 注销 Agent
//   - batch    批量操作
//   - migrate  数据库迁移（仅 --role=migration）
//   - version  打印版本
//
// 当前文件是 P2.6 骨架：root + 占位子命令；具体实现由 P5 阶段补全。
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

// 版本变量（由 Makefile ldflags 注入；参考 Makefile LDFLAGS）
var (
	//nolint:gochecknoglobals // build-time injected
	Version string
	//nolint:gochecknoglobals // build-time injected
	Commit string
	//nolint:gochecknoglobals // build-time injected
	BuildDate string
)

// rootCmd cobra 根命令。
var rootCmd = &cobra.Command{
	Use:   "agentid",
	Short: "AgentID-Chain CLI — AI Agent 分布式身份与权限网关",
	Long: `agentid 是 AgentID-Chain 的统一入口。

支持四种范式（docs §4）：
  · CLI   : 本工具（cobra）
  · MCP   : Model Context Protocol server（--role=mcp）
  · A2A   : Agent-to-Agent Token 服务
  · Prompt: NL 解析（experimental）

文档基线：docs/AgentID-Chain-技术文档-v2.0.1.md
`,
	SilenceUsage:  true,
	SilenceErrors: false,
	// 默认无参数时执行：打印 help
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

// Execute 入口函数（main.go 调用）。
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// 全局 flags
	rootCmd.PersistentFlags().StringP("config", "c", "config.yaml", "配置文件路径")
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "日志级别 (debug|info|warn|error)")
	rootCmd.PersistentFlags().String("role", "gateway", "运行角色 (gateway|auth-center|tag-sense|mcp|migration|cli)")

	// 注册子命令（占位实现，P5 阶段补全）
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(banCmd)
	rootCmd.AddCommand(unbanCmd)
	rootCmd.AddCommand(unregisterCmd)
	rootCmd.AddCommand(batchCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

// versionCmd 打印版本信息。
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "打印版本信息",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("agentid %s\n", versionString())
	},
}

func versionString() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	c := Commit
	if c == "" {
		c = "none"
	}
	d := BuildDate
	if d == "" {
		d = "unknown"
	}
	return fmt.Sprintf("%s (commit %s, built %s, %s/%s)",
		v, c, d, runtime.GOOS, runtime.GOARCH)
}

// ---------- 子命令占位（各命令的 RunE 仅打印 not-implemented；P5 阶段替换） ----------

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "起 gateway 服务（HTTP/gRPC/MCP）",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "serve: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "注册新 Agent",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "register: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "查询 Agent 信息",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "info: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "升级 Agent Level",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "upgrade: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var banCmd = &cobra.Command{
	Use:   "ban",
	Short: "封禁 Agent",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "ban: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var unbanCmd = &cobra.Command{
	Use:   "unban",
	Short: "解封 Agent",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "unban: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var unregisterCmd = &cobra.Command{
	Use:   "unregister",
	Short: "注销 Agent",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "unregister: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "批量操作（CSV 输入）",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "batch: not implemented yet (see LRA P5)")
		os.Exit(1)
		return nil
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "数据库迁移（仅 --role=migration）",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr, "migrate: not implemented yet (see LRA P3.1)")
		os.Exit(1)
		return nil
	},
}
