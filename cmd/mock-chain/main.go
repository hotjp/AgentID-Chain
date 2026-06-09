// mock-chain 模拟 EVM 链节点 + 链适配器 HTTP 端点（开发/测试用）
//
// 用途：本地开发与 CI 环境中替代真链（FISCO/Polygon/BSC）。
// 暴露：
//   - EVM JSON-RPC 桩（eth_blockNumber / eth_chainId / ...）
//   - 自定义 chain adapter REST 端点（与 core/chain_adapter.BaseChainAdapter 一一对应）
//   - /healthz / /readyz / /metrics
//
// P8.9：HTTP 暴露 BaseChainAdapter（P8.10 OnchainBackend 通过 HTTP 调用）
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

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
)

// 编译期注入
var (
	//nolint:gochecknoglobals // build-time injected
	Version string
)

// =============================================================================
// JSON-RPC 通用类型（EVM 风格）
// =============================================================================

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// =============================================================================
// 自定义 HTTP API（BaseChainAdapter 桥接）
// =============================================================================

// registerReq HTTP 请求体。
type httpRegisterReq struct {
	UUID       string            `json:"uuid"`
	Owner      string            `json:"owner"`
	Level      uint8             `json:"level"`
	Permission uint64            `json:"permission"`
	PublicKey  string            `json:"public_key"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type httpUpdateLevelReq struct {
	UUID     string `json:"uuid"`
	NewLevel uint8  `json:"new_level"`
	Reason   string `json:"reason,omitempty"`
}

type httpBanReq struct {
	UUID   string `json:"uuid"`
	Reason string `json:"reason,omitempty"`
}

type httpReceiptResp struct {
	TxHash      string `json:"tx_hash"`
	BlockNumber uint64 `json:"block_number"`
	GasUsed     uint64 `json:"gas_used"`
	ConfirmedAt string `json:"confirmed_at"`
}

type httpStateResp struct {
	UUID       string `json:"uuid"`
	Owner      string `json:"owner"`
	Level      uint8  `json:"level"`
	State      string `json:"state"`
	Permission uint64 `json:"permission"`
	PublicKey  string `json:"public_key"`
	TxHash     string `json:"tx_hash"`
	UpdatedAt  string `json:"updated_at"`
}

type httpErrorResp struct {
	Error string `json:"error"`
}

// =============================================================================
// 状态
// =============================================================================

// mockState 全局状态。
type mockState struct {
	adapter    *mock.MockAdapter
	blockStart atomic.Uint64
}

// =============================================================================
// 入口
// =============================================================================

func main() {
	addr := flag.String("addr", ":8545", "监听地址")
	logLevel := flag.String("log-level", "info", "日志级别 (debug|info|warn|error)")
	chainID := flag.Uint64("chain-id", 1337, "链 ID")
	startBlock := flag.Uint64("start-block", 1, "起始 block number")
	flag.Parse()

	level := slog.LevelInfo
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	adapter := mock.NewWithConfig(*chainID, *startBlock)
	state := &mockState{adapter: adapter}
	state.blockStart.Store(*startBlock)

	mux := http.NewServeMux()
	// 健康检查
	mux.HandleFunc("/healthz", healthzHandler(adapter))
	mux.HandleFunc("/readyz", healthzHandler(adapter))
	// BaseChainAdapter HTTP 端点
	mux.HandleFunc("/api/v1/agents/register", registerHandler(adapter, logger))
	mux.HandleFunc("/api/v1/agents/update-level", updateLevelHandler(adapter, logger))
	mux.HandleFunc("/api/v1/agents/ban", banHandler(adapter, logger))
	mux.HandleFunc("/api/v1/agents/unban", unbanHandler(adapter, logger))
	mux.HandleFunc("/api/v1/agents/revoke", revokeHandler(adapter, logger))
	mux.HandleFunc("/api/v1/agents/state", getStateHandler(adapter, logger))
	// EVM JSON-RPC 桩
	mux.HandleFunc("/", jsonRPCHandler(state))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           loggingMiddleware(mux, logger),
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
		fmt.Fprintf(os.Stderr, "mock-chain %s listening on %s (chain-id=%d, log-level=%s)\n",
			ver, *addr, *chainID, *logLevel)
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
	_ = adapter.Close()
}

// =============================================================================
// Health
// =============================================================================

func healthzHandler(adapter *mock.MockAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := adapter.HealthCheck(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(httpErrorResp{Error: err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"service": "mock-chain",
			"chain":   string(adapter.ChainType()),
		})
	}
}

// =============================================================================
// Chain Adapter HTTP handlers
// =============================================================================

func registerHandler(adapter chain_adapter.BaseChainAdapter, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req httpRegisterReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		receipt, err := adapter.RegisterAgent(r.Context(), &chain_adapter.RegisterRequest{
			UUID:       req.UUID,
			Owner:      req.Owner,
			Level:      req.Level,
			Permission: req.Permission,
			PublicKey:  req.PublicKey,
			Metadata:   req.Metadata,
		})
		if err != nil {
			writeChainErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, receiptToResp(receipt))
		logger.Debug("register ok", "uuid", req.UUID, "tx", receipt.TxHash)
	}
}

func updateLevelHandler(adapter chain_adapter.BaseChainAdapter, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req httpUpdateLevelReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		receipt, err := adapter.UpdateLevel(r.Context(), &chain_adapter.UpdateLevelRequest{
			UUID:     req.UUID,
			NewLevel: req.NewLevel,
			Reason:   req.Reason,
		})
		if err != nil {
			writeChainErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, receiptToResp(receipt))
		logger.Debug("update-level ok", "uuid", req.UUID, "tx", receipt.TxHash)
	}
}

func banHandler(adapter chain_adapter.BaseChainAdapter, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req httpBanReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		receipt, err := adapter.BanAgent(r.Context(), &chain_adapter.BanRequest{
			UUID:   req.UUID,
			Reason: req.Reason,
		})
		if err != nil {
			writeChainErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, receiptToResp(receipt))
		logger.Debug("ban ok", "uuid", req.UUID, "tx", receipt.TxHash)
	}
}

func unbanHandler(adapter chain_adapter.BaseChainAdapter, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		uuid := r.URL.Query().Get("uuid")
		if uuid == "" {
			var body httpBanReq
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				uuid = body.UUID
			}
		}
		if uuid == "" {
			writeJSONErr(w, http.StatusBadRequest, "uuid required")
			return
		}
		receipt, err := adapter.UnbanAgent(r.Context(), uuid)
		if err != nil {
			writeChainErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, receiptToResp(receipt))
		logger.Debug("unban ok", "uuid", uuid, "tx", receipt.TxHash)
	}
}

func revokeHandler(adapter chain_adapter.BaseChainAdapter, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		uuid := r.URL.Query().Get("uuid")
		if uuid == "" {
			var body httpBanReq
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				uuid = body.UUID
			}
		}
		if uuid == "" {
			writeJSONErr(w, http.StatusBadRequest, "uuid required")
			return
		}
		receipt, err := adapter.RevokeAgent(r.Context(), uuid)
		if err != nil {
			writeChainErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, receiptToResp(receipt))
		logger.Debug("revoke ok", "uuid", uuid, "tx", receipt.TxHash)
	}
}

func getStateHandler(adapter chain_adapter.BaseChainAdapter, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		uuid := r.URL.Query().Get("uuid")
		if uuid == "" {
			writeJSONErr(w, http.StatusBadRequest, "uuid required")
			return
		}
		agent, err := adapter.GetAgentState(r.Context(), uuid)
		if err != nil {
			writeChainErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, agentToResp(agent))
		logger.Debug("get-state ok", "uuid", uuid)
	}
}

// =============================================================================
// JSON-RPC（EVM 桩）
// =============================================================================

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
			Result:  fmt.Sprintf("0x%x", state.adapter.BlockNumber()),
		}
	case "eth_chainId":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  fmt.Sprintf("0x%x", state.adapter.ChainID()),
		}
	case "net_version":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  fmt.Sprintf("%d", state.adapter.ChainID()),
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

// =============================================================================
// 工具
// =============================================================================

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, httpErrorResp{Error: msg})
}

func writeChainErr(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case *chain_adapter.ErrAgentNotFoundOnchain:
		writeJSON(w, http.StatusNotFound, httpErrorResp{Error: e.Error()})
	case *chain_adapter.ErrChainUnavailable:
		writeJSON(w, http.StatusServiceUnavailable, httpErrorResp{Error: e.Error()})
	case *chain_adapter.ErrTxFailed:
		writeJSON(w, http.StatusBadRequest, httpErrorResp{Error: e.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, httpErrorResp{Error: err.Error()})
	}
}

func writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: msg},
	})
}

func receiptToResp(r *chain_adapter.Receipt) httpReceiptResp {
	return httpReceiptResp{
		TxHash:      r.TxHash,
		BlockNumber: r.BlockNumber,
		GasUsed:     r.GasUsed,
		ConfirmedAt: r.ConfirmedAt.UTC().Format(time.RFC3339Nano),
	}
}

func agentToResp(a *chain_adapter.AgentOnchain) httpStateResp {
	return httpStateResp{
		UUID:       a.UUID,
		Owner:      a.Owner,
		Level:      a.Level,
		State:      string(a.State),
		Permission: a.Permission,
		PublicKey:  a.PublicKey,
		TxHash:     a.TxHash,
		UpdatedAt:  a.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func versionOr(def string) string {
	if Version == "" {
		return def
	}
	return Version
}

// loggingMiddleware 简单请求日志（method/path/status/duration）。
func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		logger.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
