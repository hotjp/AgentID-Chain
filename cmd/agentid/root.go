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
	rootCmd.PersistentFlags().StringP("config", "c", "", "配置文件路径（可选；缺省走 env + 默认值）")
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "日志级别 (debug|info|warn|error)")
	rootCmd.PersistentFlags().String("role", "gateway", "运行角色 (gateway|auth-center|tag-sense|mcp|migration|cli)")

	// 注册子命令
	registerSubcommands()
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

// ---------- 真实子命令 ----------

// serveCmd 见 serve.go。
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "起 gateway 服务（HTTP/gRPC/MCP）",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runServe(nil, nil)
	},
}

// upgradeCmd 见 upgrade.go。
var upgradeCmd = upgradeCmdImpl

// banCmd 见 ban.go。
var banCmd = banCmdImpl

// unbanCmd 见 unban.go。
var unbanCmd = unbanCmdImpl

// unregisterCmd 见 unregister.go。
var unregisterCmd = unregisterCmdImpl

// batchCmd 见 batch_register.go / batch.go。
var batchCmd = batchCmdImpl

// migrateCmd 见 migrate.go。
var migrateCmd = migrateCmdImpl

// infoCmd 见 info.go。
var infoCmd = infoCmdImpl

// registerCmd 见 register.go。
var registerCmd = registerCmdImpl

// auditCmd 见 audit.go。
var auditCmd = auditCmdImpl

// localCmd 见 local_init.go（父命令 local，承载 init 等子命令）。
var localCmd = localParentCmd

// configCmd 见 config_cmd.go。
var configCmd = configCmdImpl

// promptCmd 见 prompt.go。
var promptCmd = promptCmdImpl

// aapCmd 见 aap_handshake.go。
var aapCmd = aapCmdImpl

// mcpCmd 见 mcp.go。
var mcpCmd = mcpCmdImpl

// registerSubcommands 把全部子命令挂到 rootCmd（避免 init() 跨文件顺序问题）。
func registerSubcommands() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(banCmd)
	rootCmd.AddCommand(unbanCmd)
	rootCmd.AddCommand(unregisterCmd)
	rootCmd.AddCommand(batchCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(localCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(promptCmd)
	rootCmd.AddCommand(aapCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(versionCmd)
}
