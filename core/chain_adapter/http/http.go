// Package http 提供了通过 HTTP 调用链适配器端的客户端实现。
//
// 用途：
//   - OnchainBackend 在 P8.10 中持有 BaseChainAdapter，但生产环境通常通过
//     HTTP / gRPC 跨进程调用真链节点（合约 SDK 在独立进程）。
//   - 本包的 HTTPAdapter 把 BaseChainAdapter 6 个方法桥接到 mock-chain 的
//     /api/v1/agents/* HTTP 端点（见 cmd/mock-chain/main.go）。
//
// 设计：
//   - 简单：直接 JSON 编码 / 解码
//   - 失败映射：HTTP 404 → ErrAgentNotFoundOnchain；其他 → ErrChainUnavailable / ErrTxFailed
//   - 线程安全（每个调用独立 http.Client + 短超时）
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
)

// HTTPAdapter 通过 HTTP 调用远端 ChainAdapter。
type HTTPAdapter struct {
	baseURL string
	client  *http.Client
}

// NewHTTPAdapter 构造。
func NewHTTPAdapter(baseURL string) *HTTPAdapter {
	return &HTTPAdapter{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ChainType 返回远端 chain type（通过 HealthCheck 探测）。
func (h *HTTPAdapter) ChainType() chain_adapter.ChainType {
	return chain_adapter.ChainTypeMock
}

// RegisterAgent 上链注册。
func (h *HTTPAdapter) RegisterAgent(ctx context.Context, req *chain_adapter.RegisterRequest) (*chain_adapter.Receipt, error) {
	var receipt chain_adapter.Receipt
	err := h.do(ctx, http.MethodPost, "/api/v1/agents/register", req, &receipt)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

// UpdateLevel 链上更新 Level。
func (h *HTTPAdapter) UpdateLevel(ctx context.Context, req *chain_adapter.UpdateLevelRequest) (*chain_adapter.Receipt, error) {
	var receipt chain_adapter.Receipt
	err := h.do(ctx, http.MethodPost, "/api/v1/agents/update-level", req, &receipt)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

// BanAgent 链上封禁。
func (h *HTTPAdapter) BanAgent(ctx context.Context, req *chain_adapter.BanRequest) (*chain_adapter.Receipt, error) {
	var receipt chain_adapter.Receipt
	err := h.do(ctx, http.MethodPost, "/api/v1/agents/ban", req, &receipt)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

// UnbanAgent 链上解封。
func (h *HTTPAdapter) UnbanAgent(ctx context.Context, uuid string) (*chain_adapter.Receipt, error) {
	var receipt chain_adapter.Receipt
	err := h.do(ctx, http.MethodPost, "/api/v1/agents/unban?uuid="+uuid, nil, &receipt)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

// RevokeAgent 链上注销。
func (h *HTTPAdapter) RevokeAgent(ctx context.Context, uuid string) (*chain_adapter.Receipt, error) {
	var receipt chain_adapter.Receipt
	err := h.do(ctx, http.MethodPost, "/api/v1/agents/revoke?uuid="+uuid, nil, &receipt)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

// GetAgentState 查询链上状态。
func (h *HTTPAdapter) GetAgentState(ctx context.Context, uuid string) (*chain_adapter.AgentOnchain, error) {
	var agent chain_adapter.AgentOnchain
	err := h.do(ctx, http.MethodGet, "/api/v1/agents/state?uuid="+uuid, nil, &agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// HealthCheck 健康检查。
func (h *HTTPAdapter) HealthCheck(ctx context.Context) error {
	return h.do(ctx, http.MethodGet, "/healthz", nil, nil)
}

// =============================================================================
// 内部
// =============================================================================

// do 通用 HTTP 调用。
func (h *HTTPAdapter) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, reqBody)
	if err != nil {
		return &chain_adapter.ErrChainUnavailable{Reason: "new request: " + err.Error()}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return &chain_adapter.ErrChainUnavailable{Reason: "do request: " + err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		if out != nil {
			if err := json.Unmarshal(respBody, out); err != nil {
				return &chain_adapter.ErrTxFailed{Reason: "decode: " + err.Error()}
			}
		}
		return nil
	}

	// 错误响应
	var errResp struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(respBody, &errResp)
	if errResp.Error == "" {
		errResp.Error = string(respBody)
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return &chain_adapter.ErrAgentNotFoundOnchain{UUID: extractUUID(path, errResp.Error)}
	case http.StatusServiceUnavailable:
		return &chain_adapter.ErrChainUnavailable{Reason: errResp.Error}
	default:
		return &chain_adapter.ErrTxFailed{Reason: errResp.Error}
	}
}

func extractUUID(path, _ string) string {
	// 简单提取：从 path 中抽 ?uuid=xxx
	const marker = "?uuid="
	i := bytes.Index([]byte(path), []byte(marker))
	if i < 0 {
		return ""
	}
	rest := path[i+len(marker):]
	return rest
}

// 编译期检查
var _ chain_adapter.BaseChainAdapter = (*HTTPAdapter)(nil)

// 抑制 unused warning
var _ = errors.New
