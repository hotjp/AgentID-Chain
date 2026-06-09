// Package main migrate 子命令实现（数据库迁移）。
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var migrateFlagVals = struct {
	dsn      string
	dryRun   bool
	rollback bool
	to       string
}{}

var migrateCmdImpl = &cobra.Command{
	Use:   "migrate",
	Short: "数据库迁移（仅 --role=migration）",
	Long: `运行数据库 schema 迁移。

Examples:
  agentid migrate --role migration --dsn <dsn>
  agentid migrate --role migration --dsn <dsn> --dry-run`,
	RunE: runMigrate,
}

func init() {
	migrateCmdImpl.Flags().StringVar(&migrateFlagVals.dsn, "dsn", "", "PostgreSQL DSN（覆盖 config）")
	migrateCmdImpl.Flags().BoolVar(&migrateFlagVals.dryRun, "dry-run", false, "只打印 SQL，不执行")
	migrateCmdImpl.Flags().BoolVar(&migrateFlagVals.rollback, "rollback", false, "回滚上一批")
	migrateCmdImpl.Flags().StringVar(&migrateFlagVals.to, "to", "", "目标版本（migration 名）")
}

func runMigrate(_ *cobra.Command, _ []string) error {
	// 此处为占位；实际迁移在 P3 L1 / P15 测试基础设施阶段提供。
	// 现阶段 migrate 工具位于 cmd/migration-tool/（独立的子命令）。
	fmt.Fprintln(os.Stderr, "migrate: see cmd/migration-tool/main.go (P3 L1) — agentid migrate will be wired in P15")
	return fmt.Errorf("not implemented in this build")
}
