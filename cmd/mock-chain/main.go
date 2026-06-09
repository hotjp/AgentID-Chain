// mock-chain 模拟 EVM JSON-RPC 链节点（开发/测试用）
//
// 用途：本地开发与 CI 环境中替代真链（FISCO/Polygon/BSC）。
// 暴露 EVM JSON-RPC 风格接口 + 健康检查端点。
//
// 当前 P2.8 骨架：HTTP server + /healthz + 简单 JSON-RPC 桩
// P3.8 阶段接 internal/plugins/chain/mock/ 实现真实子集（eth_call/eth_blockNumber/...）
//
// 与 docker/compose/*.yml 集成：
//   - 在 hybrid compose 中作为 gateway 的链上后端
//   - 端口 8545（EVM 标准 JSON-RPC 端口）
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// 编译期注入
var (
	//nolint:gochecknoglobals // build-time injected
	Version string
)

// JSON-RPC 2.0 通用类型
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mockState 全局状态（无锁；P2.8 占位）
type mockState struct {
	blockNumber atomic.Uint64
	_           atomic.Uint64 // 占位: txCounter (P3.8 启用)
}

func main() {
	addr := flag.String("addr", ":8545", "监听地址")
	logLevel := flag.String("log-level", "info", "日志级别 (debug|info|warn|error)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	_ = logger // P3.8 阶段接 chain adapter 时使用

	state := &mockState{}
	state.blockNumber.Store(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/readyz", healthzHandler)
	mux.HandleFunc("/", jsonRPCHandler(state))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 优雅关闭
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		ver := Version
		if ver == "" {
			ver = "dev"
		}
		fmt.Fprintf(os.Stderr, "mock-chain %s listening on %s (log-level=%s)\n", ver, *addr, *logLevel)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	fmt.Fprintln(os.Stderr, "mock-chain shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// healthzHandler 健康检查
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"mock-chain"}`))
}

// jsonRPCHandler 极简 JSON-RPC 路由（P2.8 占位）
func jsonRPCHandler(state *mockState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONRPCError(w, nil, -32700, "parse error")
			return
		}
		resp := handleRPCMethod(r.Context(), &req, state)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func handleRPCMethod(_ context.Context, req *jsonRPCRequest, state *mockState) jsonRPCResponse {
	switch req.Method {
	case "eth_blockNumber":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  fmt.Sprintf("0x%x", state.blockNumber.Load()),
		}
	case "eth_chainId":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "0x539", // 1337 (Anvil/Hardhat default)
		}
	case "net_version":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "1337",
		}
	case "eth_accounts":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  []string{},
		}
	case "web3_clientVersion":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  fmt.Sprintf("mock-chain/%s", versionOr("dev")),
		}
	default:
		// P2.8 占位：未实现方法返回 -32601
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32601,
				Message: "method not implemented: " + req.Method,
			},
		}
	}
}

func writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC 错误也用 200
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: msg},
	})
}

func versionOr(def string) string {
	if Version == "" {
		return def
	}
	return Version
}
