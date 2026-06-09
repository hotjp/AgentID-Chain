// Package main register 子命令实现。
//
// 用法：
//
//	agentid register \
//	  --owner did:agentid:alice \
//	  --level 1 \
//	  --permission 255 \
//	  --public-key <base64-ed25519> \
//	  --output credentials.json \
//	  --format json
//
// 流程：
//  1. 解析 flag
//  2. 加载 CLI 配置（~/.agentid/config.yaml + --config）
//  3. 构造 Client（local / remote）
//  4. 调 RegisterAgent
//  5. 按 format 输出
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cli"
	"github.com/agentid-chain/agentid-chain/internal/cli/output"
	"github.com/spf13/cobra"
)

// registerFlags register 子命令的所有 flag。
type registerFlags struct {
	owner      string
	level      uint8
	permission uint64
	publicKey  string
	output     string
	format     string
	config     string
	gateway    string
	apiKey     string
	mode       string
	timeout    int
}

var registerFlagVals = &registerFlags{}

// registerCmd cobra 子命令。
var registerCmdImpl = &cobra.Command{
	Use:   "register",
	Short: "注册新 Agent",
	Long: `注册新 Agent 并返回凭证（UUID + TxHash）。

Examples:
  # 本地（mock 后端）注册
  agentid register --owner did:agentid:alice --level 1 --public-key <pk>

  # 调远端 gateway 注册
  agentid register --gateway http://gw:8080 --owner did:agentid:alice --level 2 \
    --public-key <pk> --output credentials.json`,
	RunE: runRegister,
}

func init() {
	registerCmdImpl.Flags().StringVar(&registerFlagVals.owner, "owner", "", "Owner DID（如 did:agentid:alice）")
	registerCmdImpl.Flags().Uint8Var(&registerFlagVals.level, "level", 1, "初始 Level（1+）")
	registerCmdImpl.Flags().Uint64Var(&registerFlagVals.permission, "permission", 0xFF, "权限位掩码（uint64）")
	registerCmdImpl.Flags().StringVar(&registerFlagVals.publicKey, "public-key", "", "Ed25519 公钥（base64）")
	registerCmdImpl.Flags().StringVar(&registerFlagVals.output, "output", "", "凭证输出文件（默认 stdout）")
	registerCmdImpl.Flags().StringVar(&registerFlagVals.format, "format", "", "输出格式 (json|table|yaml；默认从 config 读)")
	registerCmdImpl.Flags().StringVar(&registerFlagVals.config, "config", "", "CLI 配置文件路径（默认 ~/.agentid/config.yaml）")
	registerCmdImpl.Flags().StringVar(&registerFlagVals.gateway, "gateway", "", "gateway 地址（覆盖 config）")
	registerCmdImpl.Flags().StringVar(&registerFlagVals.apiKey, "api-key", "", "gateway API Key（覆盖 config）")
	registerCmdImpl.Flags().StringVar(&registerFlagVals.mode, "mode", "", "客户端模式 (local|remote；覆盖 config)")
	registerCmdImpl.Flags().IntVar(&registerFlagVals.timeout, "timeout", 0, "HTTP 超时（秒；覆盖 config）")
}

func runRegister(cmd *cobra.Command, _ []string) error {
	if err := validateRegisterFlags(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: registerFlagVals.config,
		Gateway:    registerFlagVals.gateway,
		APIKey:     registerFlagVals.apiKey,
		Mode:       registerFlagVals.mode,
		Timeout:    registerFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	cred, err := client.RegisterAgent(ctx, &cli.RegisterRequest{
		Owner:      registerFlagVals.owner,
		Level:      registerFlagVals.level,
		Permission: registerFlagVals.permission,
		PublicKey:  registerFlagVals.publicKey,
	})
	if err != nil {
		return err
	}

	// 输出
	format := output.ParseFormat(chooseFormat(client, registerFlagVals.format))
	return writeCredential(cred, registerFlagVals.output, format)
}

func validateRegisterFlags() error {
	if registerFlagVals.owner == "" {
		return fmt.Errorf("--owner is required")
	}
	if registerFlagVals.level == 0 {
		return fmt.Errorf("--level must be >= 1")
	}
	if registerFlagVals.publicKey == "" {
		return fmt.Errorf("--public-key is required")
	}
	return nil
}

// writeCredential 写凭证（文件 / stdout）。
func writeCredential(cred *cli.AgentCredential, path string, format output.Format) error {
	if path == "" || path == "-" {
		return output.Print(os.Stdout, format, cred)
	}
	// 确保目录存在
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := output.Print(f, format, cred); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "credential written to %s\n", path)
	return nil
}
