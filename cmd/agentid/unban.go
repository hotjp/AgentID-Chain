// Package main unban 子命令实现。
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var unbanFlagVals = struct {
	uuid    string
	format  string
	config  string
	gateway string
	apiKey  string
	mode    string
	timeout int
}{}

var unbanCmdImpl = &cobra.Command{
	Use:   "unban",
	Short: "解封 Agent",
	Long: `解封 Agent（state banned -> active；幂等）。

Examples:
  agentid unban --uuid <uuid>`,
	RunE: runUnban,
}

func init() {
	unbanCmdImpl.Flags().StringVar(&unbanFlagVals.uuid, "uuid", "", "Agent UUID")
	unbanCmdImpl.Flags().StringVar(&unbanFlagVals.format, "format", "", "输出格式")
	unbanCmdImpl.Flags().StringVar(&unbanFlagVals.config, "config", "", "CLI 配置文件路径")
	unbanCmdImpl.Flags().StringVar(&unbanFlagVals.gateway, "gateway", "", "gateway 地址")
	unbanCmdImpl.Flags().StringVar(&unbanFlagVals.apiKey, "api-key", "", "gateway API Key")
	unbanCmdImpl.Flags().StringVar(&unbanFlagVals.mode, "mode", "", "客户端模式")
	unbanCmdImpl.Flags().IntVar(&unbanFlagVals.timeout, "timeout", 0, "HTTP 超时（秒）")
}

func runUnban(_ *cobra.Command, _ []string) error {
	if unbanFlagVals.uuid == "" {
		return fmt.Errorf("--uuid is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: unbanFlagVals.config,
		Gateway:    unbanFlagVals.gateway,
		APIKey:     unbanFlagVals.apiKey,
		Mode:       unbanFlagVals.mode,
		Timeout:    unbanFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	if err := client.UnbanAgent(ctx, unbanFlagVals.uuid); err != nil {
		return err
	}
	return printOK(unbanFlagVals.uuid, "unbanned")
}
