// Package main audit 子命令实现。
//
// 用法：
//
//	agentid audit --uuid <uuid>
//	agentid audit --uuid <uuid> --format table --limit 20
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var auditFlagVals = struct {
	uuid    string
	limit   int
	format  string
	config  string
	gateway string
	apiKey  string
	mode    string
	timeout int
}{}

var auditCmdImpl = &cobra.Command{
	Use:   "audit",
	Short: "查询 Agent 审计日志（变更记录）",
	Long: `查询 Agent 的变更审计日志（register / upgrade / ban / unban / unregister）。

Examples:
  agentid audit --uuid <uuid>
  agentid audit --uuid <uuid> --format table --limit 10`,
	RunE: runAudit,
}

func init() {
	auditCmdImpl.Flags().StringVar(&auditFlagVals.uuid, "uuid", "", "Agent UUID")
	auditCmdImpl.Flags().IntVar(&auditFlagVals.limit, "limit", 50, "返回条数上限（1-500）")
	auditCmdImpl.Flags().StringVar(&auditFlagVals.format, "format", "", "输出格式 (json|table|yaml)")
	auditCmdImpl.Flags().StringVar(&auditFlagVals.config, "config", "", "CLI 配置文件路径")
	auditCmdImpl.Flags().StringVar(&auditFlagVals.gateway, "gateway", "", "gateway 地址")
	auditCmdImpl.Flags().StringVar(&auditFlagVals.apiKey, "api-key", "", "gateway API Key")
	auditCmdImpl.Flags().StringVar(&auditFlagVals.mode, "mode", "", "客户端模式")
	auditCmdImpl.Flags().IntVar(&auditFlagVals.timeout, "timeout", 0, "HTTP 超时（秒）")
}

func runAudit(_ *cobra.Command, _ []string) error {
	if auditFlagVals.uuid == "" {
		return fmt.Errorf("--uuid is required")
	}
	if auditFlagVals.limit < 1 || auditFlagVals.limit > 500 {
		return fmt.Errorf("--limit must be in [1, 500]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: auditFlagVals.config,
		Gateway:    auditFlagVals.gateway,
		APIKey:     auditFlagVals.apiKey,
		Mode:       auditFlagVals.mode,
		Timeout:    auditFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	logs, err := client.GetChangeLogs(ctx, auditFlagVals.uuid)
	if err != nil {
		return err
	}

	// limit 截断
	if len(logs) > auditFlagVals.limit {
		logs = logs[len(logs)-auditFlagVals.limit:]
	}
	return printLogs(client, auditFlagVals.format, logs)
}
