// Package main aap 子命令实现（CLI 侧 AAP 握手）。
//
// 用途：让 CLI 主动完成 AAP 三段式握手（challenge → sign → proof），拿到 access_token。
//
// 用法：
//
//	agentid aap handshake --key <path>             # 触发握手，把 token 缓存到 Client
//	agentid aap handshake --key <path> --print     # 同时打印 token + 过期时间
//	agentid aap token --print                      # 打印当前缓存的 token
//	agentid aap clear                              # 清空 token
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cli"
	"github.com/spf13/cobra"
)

var aapFlagVals = struct {
	key     string
	keyData string
	print   bool
	timeout int
	config  string
	gateway string
	apiKey  string
	mode    string
}{}

var aapCmdImpl = &cobra.Command{
	Use:   "aap",
	Short: "AAP 协议握手（CLI 侧）",
	Long: `CLI 侧 AAP 握手命令（与 server 端 aap.Generator / aap.ProofSigner 对齐）。

子命令：
  agentid aap handshake --key <path>          触发 challenge → sign → proof
  agentid aap token                           打印当前缓存的 token
  agentid aap clear                           清空 token

握手需要 Ed25519 私钥（PKCS#8 PEM 或 raw 64B），通过 --key 指定文件路径，
或 --key-data 指定 base64 编码的 raw 64B 私钥。

Examples:
  agentid aap handshake --key ./agent.key --print
  agentid aap token`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var aapHandshakeCmd = &cobra.Command{
	Use:   "handshake",
	Short: "执行 AAP 三段式握手（challenge → sign → proof）",
	Long: `执行 AAP 握手并把 access_token 缓存到 Client。

需要：
  --gateway    远端 gateway 地址（默认从 config 读）
  --key        Ed25519 私钥文件路径（PEM PKCS#8 或 raw 64B）

可选：
  --print      打印 token 和 expires_in
  --timeout    HTTP 超时（秒；默认 30）`,
	RunE: runAAPHandshake,
}

var aapTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "打印当前缓存的 token",
	RunE:  runAAPToken,
}

var aapClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "清空 token 缓存",
	RunE:  runAAPClear,
}

func init() {
	f := aapCmdImpl.PersistentFlags()
	f.StringVar(&aapFlagVals.key, "key", "", "Ed25519 私钥文件路径（PEM PKCS#8 或 raw 64B）")
	f.StringVar(&aapFlagVals.keyData, "key-data", "", "Ed25519 私钥（base64 编码的 raw 64B）")
	f.BoolVar(&aapFlagVals.print, "print", false, "打印 token 和 expires_in")
	f.IntVar(&aapFlagVals.timeout, "timeout", 30, "HTTP 超时（秒）")
	f.StringVar(&aapFlagVals.config, "config", "", "CLI 配置文件路径")
	f.StringVar(&aapFlagVals.gateway, "gateway", "", "gateway 地址")
	f.StringVar(&aapFlagVals.apiKey, "api-key", "", "gateway API Key")
	f.StringVar(&aapFlagVals.mode, "mode", "", "客户端模式")

	aapCmdImpl.AddCommand(aapHandshakeCmd)
	aapCmdImpl.AddCommand(aapTokenCmd)
	aapCmdImpl.AddCommand(aapClearCmd)
}

func runAAPHandshake(_ *cobra.Command, _ []string) error {
	if aapFlagVals.key == "" && aapFlagVals.keyData == "" {
		return fmt.Errorf("--key or --key-data is required")
	}
	if aapFlagVals.key != "" && aapFlagVals.keyData != "" {
		return fmt.Errorf("--key and --key-data are mutually exclusive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(aapFlagVals.timeout)*time.Second)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: aapFlagVals.config,
		Gateway:    aapFlagVals.gateway,
		APIKey:     aapFlagVals.apiKey,
		Mode:       aapFlagVals.mode,
		Timeout:    aapFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	opts := cli.HandshakeOptions{
		PrivateKeyPath: aapFlagVals.key,
	}
	if aapFlagVals.keyData != "" {
		raw, err := base64.StdEncoding.DecodeString(aapFlagVals.keyData)
		if err != nil {
			return fmt.Errorf("decode --key-data: %w", err)
		}
		opts.PrivateKey = raw
	}

	tok, err := client.AAPHandshake(ctx, opts)
	if err != nil {
		return err
	}
	if aapFlagVals.print {
		fmt.Fprintf(os.Stdout, "{\"access_token\":%q,\"expires_in\":%d,\"token_type\":%q}\n",
			tok.AccessToken, tok.ExpiresIn, tok.TokenType)
	} else {
		fmt.Fprintf(os.Stdout, "aap: handshake ok (expires_in=%ds, token cached)\n", tok.ExpiresIn)
	}
	return nil
}

func runAAPToken(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: aapFlagVals.config,
		Gateway:    aapFlagVals.gateway,
		APIKey:     aapFlagVals.apiKey,
		Mode:       aapFlagVals.mode,
		Timeout:    aapFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	tok := client.Token()
	if tok == "" {
		fmt.Fprintln(os.Stdout, "(no token cached — run `agentid aap handshake` first)")
		return nil
	}
	fmt.Fprintln(os.Stdout, tok)
	return nil
}

func runAAPClear(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: aapFlagVals.config,
		Gateway:    aapFlagVals.gateway,
		APIKey:     aapFlagVals.apiKey,
		Mode:       aapFlagVals.mode,
		Timeout:    aapFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)
	client.ClearToken()
	fmt.Fprintln(os.Stdout, "aap: token cleared")
	return nil
}
