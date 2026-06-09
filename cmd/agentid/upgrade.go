// Package main upgrade 子命令实现。
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var upgradeFlagVals = struct {
	uuid        string
	targetLevel uint8
	reason      string
	format      string
	config      string
	gateway     string
	apiKey      string
	mode        string
	timeout     int
}{}

var upgradeCmdImpl = &cobra.Command{
	Use:   "upgrade",
	Short: "升级 Agent Level",
	Long: `升级 Agent 等级（newLevel > currentLevel）。

Examples:
  agentid upgrade --uuid <uuid> --target-level 2 --reason "policy review"`,
	RunE: runUpgrade,
}

func init() {
	upgradeCmdImpl.Flags().StringVar(&upgradeFlagVals.uuid, "uuid", "", "Agent UUID")
	upgradeCmdImpl.Flags().Uint8Var(&upgradeFlagVals.targetLevel, "target-level", 2, "目标 Level（必须 > 当前 Level）")
	upgradeCmdImpl.Flags().StringVar(&upgradeFlagVals.reason, "reason", "", "升级原因")
	upgradeCmdImpl.Flags().StringVar(&upgradeFlagVals.format, "format", "", "输出格式")
	upgradeCmdImpl.Flags().StringVar(&upgradeFlagVals.config, "config", "", "CLI 配置文件路径")
	upgradeCmdImpl.Flags().StringVar(&upgradeFlagVals.gateway, "gateway", "", "gateway 地址")
	upgradeCmdImpl.Flags().StringVar(&upgradeFlagVals.apiKey, "api-key", "", "gateway API Key")
	upgradeCmdImpl.Flags().StringVar(&upgradeFlagVals.mode, "mode", "", "客户端模式")
	upgradeCmdImpl.Flags().IntVar(&upgradeFlagVals.timeout, "timeout", 0, "HTTP 超时（秒）")
}

func runUpgrade(_ *cobra.Command, _ []string) error {
	if upgradeFlagVals.uuid == "" {
		return fmt.Errorf("--uuid is required")
	}
	if upgradeFlagVals.targetLevel == 0 {
		return fmt.Errorf("--target-level must be >= 1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: upgradeFlagVals.config,
		Gateway:    upgradeFlagVals.gateway,
		APIKey:     upgradeFlagVals.apiKey,
		Mode:       upgradeFlagVals.mode,
		Timeout:    upgradeFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	if err := client.UpdateAgentLevel(ctx, upgradeFlagVals.uuid, upgradeFlagVals.targetLevel, upgradeFlagVals.reason); err != nil {
		return err
	}
	fmt.Fprintf(prettyWriter(), `{"ok":true,"uuid":%q,"new_level":%d}`+"\n",
		upgradeFlagVals.uuid, upgradeFlagVals.targetLevel)
	return nil
}
