// migration-tool 数据库迁移工具（独立二进制）
//
// 用途：在不启动业务服务的情况下执行数据库 schema 迁移。
// 与主 CLI 分离：
//   - 主 CLI (cmd/agentid) 启动业务服务时按需运行 migration
//   - 本工具用于 CI/CD、运维人员手工迁移、回滚等场景
//
// 迁移策略：
//   - 默认使用 ent ORM (P3.1 接入)
//   - 支持 up / down / status / create 子命令
//   - 多库支持：apigateway / authcenter / tagsense / agentid_audit
//
// 当前 P2.7 骨架：cobra 子命令 + 框架占位
// P3.1 阶段接 ent 时填充实际 migration 逻辑。
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// 版本变量（Makefile ldflags 注入）
var (
	//nolint:gochecknoglobals // build-time injected
	Version string
	//nolint:gochecknoglobals // build-time injected
	Commit string
)

// 目标库列表（与 scripts/init-databases.sql 对齐）
var dbs = []string{"apigateway", "authcenter", "tagsense", "agentid_audit"}

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "migration-tool",
	Short: "AgentID-Chain 数据库迁移工具",
	Long: `migration-tool 独立二进制，用于执行/回滚/查看 AgentID-Chain 各业务库的 schema 迁移。

支持的子库：
  · apigateway    API Gateway 业务库
  · authcenter    Auth Center 业务库
  · tagsense      Tag Sense 业务库
  · agentid_audit 审计库

文档：docs/AgentID-Chain-技术文档-v2.0.1.md §3.1`,
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "migrations.yaml", "迁移配置文件路径")
	rootCmd.PersistentFlags().String("dsn", "", "PostgreSQL DSN（覆盖配置文件）")
	rootCmd.PersistentFlags().Bool("dry-run", false, "只打印将执行的 SQL，不真正执行")
	rootCmd.PersistentFlags().StringSlice("dbs", dbs, "目标数据库列表")

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(versionCmd)
}

// upCmd 应用所有未执行的迁移
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "应用所有未执行的迁移（up）",
	RunE: func(cmd *cobra.Command, _ []string) error {
		targets, _ := cmd.Flags().GetStringSlice("dbs")
		_, _ = fmt.Fprintf(os.Stderr, "migration up: dbs=%s (not implemented yet — see LRA P3.1)\n", strings.Join(targets, ","))
		os.Exit(1)
		return nil
	},
}

// downCmd 回滚最近一次迁移
var downCmd = &cobra.Command{
	Use:   "down",
	Short: "回滚最近一次迁移（down）",
	RunE: func(cmd *cobra.Command, _ []string) error {
		targets, _ := cmd.Flags().GetStringSlice("dbs")
		_, _ = fmt.Fprintf(os.Stderr, "migration down: dbs=%s (not implemented yet — see LRA P3.1)\n", strings.Join(targets, ","))
		os.Exit(1)
		return nil
	},
}

// statusCmd 显示当前 schema 版本与待执行迁移
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "显示各库的 schema 版本",
	RunE: func(cmd *cobra.Command, _ []string) error {
		targets, _ := cmd.Flags().GetStringSlice("dbs")
		_, _ = fmt.Fprintf(os.Stderr, "migration status: dbs=%s (not implemented yet — see LRA P3.1)\n", strings.Join(targets, ","))
		os.Exit(1)
		return nil
	},
}

// createCmd 创建新的迁移文件
var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "创建新的迁移文件 (up.sql / down.sql)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprintf(os.Stderr, "migration create %s: not implemented yet — see LRA P3.1\n", args[0])
		os.Exit(1)
		return nil
	},
}

// versionCmd 打印版本
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "打印版本信息",
	Run: func(_ *cobra.Command, _ []string) {
		v := Version
		if v == "" {
			v = "dev"
		}
		c := Commit
		if c == "" {
			c = "none"
		}
		fmt.Printf("migration-tool %s (commit %s)\n", v, c)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
