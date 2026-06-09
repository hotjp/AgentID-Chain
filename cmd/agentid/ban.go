// Package main ban/unban 子命令实现。
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cli"
	"github.com/spf13/cobra"
)

var banFlagVals = struct {
	uuid    string
	reason  string
	format  string
	config  string
	gateway string
	apiKey  string
	mode    string
	timeout int
}{}

var banCmdImpl = &cobra.Command{
	Use:   "ban",
	Short: "封禁 Agent",
	Long: `封禁 Agent（state -> banned；幂等）。

Examples:
  agentid ban --uuid <uuid> --reason "policy violation"`,
	RunE: runBan,
}

func init() {
	banCmdImpl.Flags().StringVar(&banFlagVals.uuid, "uuid", "", "Agent UUID")
	banCmdImpl.Flags().StringVar(&banFlagVals.reason, "reason", "policy", "封禁原因")
	banCmdImpl.Flags().StringVar(&banFlagVals.format, "format", "", "输出格式")
	banCmdImpl.Flags().StringVar(&banFlagVals.config, "config", "", "CLI 配置文件路径")
	banCmdImpl.Flags().StringVar(&banFlagVals.gateway, "gateway", "", "gateway 地址")
	banCmdImpl.Flags().StringVar(&banFlagVals.apiKey, "api-key", "", "gateway API Key")
	banCmdImpl.Flags().StringVar(&banFlagVals.mode, "mode", "", "客户端模式")
	banCmdImpl.Flags().IntVar(&banFlagVals.timeout, "timeout", 0, "HTTP 超时（秒）")
}

func runBan(_ *cobra.Command, _ []string) error {
	if banFlagVals.uuid == "" {
		return fmt.Errorf("--uuid is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: banFlagVals.config,
		Gateway:    banFlagVals.gateway,
		APIKey:     banFlagVals.apiKey,
		Mode:       banFlagVals.mode,
		Timeout:    banFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	if err := client.BanAgent(ctx, banFlagVals.uuid, banFlagVals.reason); err != nil {
		return err
	}
	return printOK(banFlagVals.uuid, "banned")
}

// printOK 通用 ok 响应输出。
func printOK(uuid, action string) error {
	fmt.Fprintf(prettyWriter(), `{"ok":true,"uuid":%q,"action":%q}`+"\n", uuid, action)
	return nil
}

// 编译期检查
var _ = cli.ModeLocal

