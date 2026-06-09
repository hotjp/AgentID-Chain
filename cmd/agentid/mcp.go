// Package main mcp 子命令实现 — 启动 MCP 服务器（stdio 传输）。
//
// 用法：
//
//	agentid mcp --api-key <key>               # 启动 MCP 服务器（stdio）
//	agentid mcp --api-key <key> --backend mock  # 指定本地后端类型
//
// MCP 服务器把 AgentID-Chain 的 8 个工具暴露给 LLM：
//   - agentid_register / agentid_get_info / agentid_upgrade
//   - agentid_check_permission / agentid_audit_logs
//   - agentid_batch_register / agentid_ban / agentid_unban
//
// 传输层：stdin/stdout JSON-RPC 2.0（与 Anthropic / OpenAI / Cursor 客户端协议兼容）。
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/agentid-chain/agentid-chain/core/backend"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
	"github.com/agentid-chain/agentid-chain/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpFlagVals = struct {
	apiKey string
	be     string
}{}

var mcpCmdImpl = &cobra.Command{
	Use:   "mcp",
	Short: "启动 MCP 服务器（stdio 传输）",
	Long: `启动 AgentID-Chain 的 MCP（Model Context Protocol）服务器。

Examples:
  agentid mcp --api-key my-key
  agentid mcp --api-key my-key --backend mock`,
	RunE: runMCP,
}

func init() {
	mcpCmdImpl.Flags().StringVar(&mcpFlagVals.apiKey, "api-key", "", "API Key（initialize 时校验）")
	mcpCmdImpl.Flags().StringVar(&mcpFlagVals.be, "backend", "mock", "本地后端类型 (local|mock|onchain|hybrid)")
}

func runMCP(_ *cobra.Command, _ []string) error {
	be, err := buildLocalBackendForMCP(mcpFlagVals.be)
	if err != nil {
		return err
	}
	defer be.Close(context.Background())

	srv := mcp.NewServer(mcp.ServerInfo{
		Name:    "agentid-chain",
		Version: "2.0.1",
	}, mcpFlagVals.apiKey)
	mcp.RegisterAgentIDTools(srv, be)

	// 写到 stderr，避免污染 stdout 的 JSON-RPC 帧
	fmt.Fprintln(os.Stderr, "agentid mcp: serving on stdio (api-key auth)")
	return srv.Serve(context.Background())
}

func buildLocalBackendForMCP(typ string) (backend.IdentityBackend, error) {
	switch typ {
	case "local", "mock":
		return backend.NewBackend(backend.Config{Type: backend.TypeMock})
	case "onchain", "hybrid":
		return backend.NewBackend(backend.Config{
			Type:         backend.TypeOnchain,
			ChainAdapter: mock.NewMockAdapter(),
		})
	default:
		return nil, fmt.Errorf("unknown backend %q", typ)
	}
}
