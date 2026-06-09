// Package main info 子命令实现。
//
// 用法：
//
//	agentid info --uuid <uuid>
//	agentid info --uuid <uuid> --format table
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var infoFlagVals = struct {
	uuid    string
	format  string
	config  string
	gateway string
	apiKey  string
	mode    string
	timeout int
}{}

var infoCmdImpl = &cobra.Command{
	Use:   "info",
	Short: "查询 Agent 完整信息",
	Long: `查询 Agent 完整信息（UUID / Owner / Level / State / Permission / PublicKey / TxHash）。

Examples:
  agentid info --uuid 019eab13-e32e-7249-ab6d-bd63989530b5
  agentid info --uuid <uuid> --format table`,
	RunE: runInfo,
}

func init() {
	infoCmdImpl.Flags().StringVar(&infoFlagVals.uuid, "uuid", "", "Agent UUID")
	infoCmdImpl.Flags().StringVar(&infoFlagVals.format, "format", "", "输出格式 (json|table|yaml)")
	infoCmdImpl.Flags().StringVar(&infoFlagVals.config, "config", "", "CLI 配置文件路径")
	infoCmdImpl.Flags().StringVar(&infoFlagVals.gateway, "gateway", "", "gateway 地址")
	infoCmdImpl.Flags().StringVar(&infoFlagVals.apiKey, "api-key", "", "gateway API Key")
	infoCmdImpl.Flags().StringVar(&infoFlagVals.mode, "mode", "", "客户端模式")
	infoCmdImpl.Flags().IntVar(&infoFlagVals.timeout, "timeout", 0, "HTTP 超时（秒）")
}

func runInfo(_ *cobra.Command, _ []string) error {
	if infoFlagVals.uuid == "" {
		return fmt.Errorf("--uuid is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: infoFlagVals.config,
		Gateway:    infoFlagVals.gateway,
		APIKey:     infoFlagVals.apiKey,
		Mode:       infoFlagVals.mode,
		Timeout:    infoFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	info, err := client.GetAgentInfo(ctx, infoFlagVals.uuid)
	if err != nil {
		return err
	}
	return printAgentInfo(client, infoFlagVals.format, info)
}
