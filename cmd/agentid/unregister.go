// Package main unregister 子命令实现。
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var unregisterFlagVals = struct {
	uuid    string
	format  string
	config  string
	gateway string
	apiKey  string
	mode    string
	timeout int
}{}

var unregisterCmdImpl = &cobra.Command{
	Use:   "unregister",
	Short: "注销 Agent（永久）",
	Long: `注销 Agent（state -> unregistered；不可恢复；幂等）。

Examples:
  agentid unregister --uuid <uuid>`,
	RunE: runUnregister,
}

func init() {
	unregisterCmdImpl.Flags().StringVar(&unregisterFlagVals.uuid, "uuid", "", "Agent UUID")
	unregisterCmdImpl.Flags().StringVar(&unregisterFlagVals.format, "format", "", "输出格式")
	unregisterCmdImpl.Flags().StringVar(&unregisterFlagVals.config, "config", "", "CLI 配置文件路径")
	unregisterCmdImpl.Flags().StringVar(&unregisterFlagVals.gateway, "gateway", "", "gateway 地址")
	unregisterCmdImpl.Flags().StringVar(&unregisterFlagVals.apiKey, "api-key", "", "gateway API Key")
	unregisterCmdImpl.Flags().StringVar(&unregisterFlagVals.mode, "mode", "", "客户端模式")
	unregisterCmdImpl.Flags().IntVar(&unregisterFlagVals.timeout, "timeout", 0, "HTTP 超时（秒）")
}

func runUnregister(_ *cobra.Command, _ []string) error {
	if unregisterFlagVals.uuid == "" {
		return fmt.Errorf("--uuid is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: unregisterFlagVals.config,
		Gateway:    unregisterFlagVals.gateway,
		APIKey:     unregisterFlagVals.apiKey,
		Mode:       unregisterFlagVals.mode,
		Timeout:    unregisterFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	if err := client.UnregisterAgent(ctx, unregisterFlagVals.uuid); err != nil {
		return err
	}
	return printOK(unregisterFlagVals.uuid, "unregistered")
}
