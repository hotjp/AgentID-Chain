// Package cli 提供 agentid CLI 的共享客户端与配置加载。
//
// 设计：
//   - 双模式：Local（直连 core/backend 内存 / 配置）；Remote（HTTP 调 gateway）
//   - 模式由 ~/.agentid/config.yaml 中 mode 字段 + --gateway CLI flag 决定
//   - Output 由 --output 决定（json | table | yaml）
//
// 用法（典型 wire）：
//
//	cfg, _ := cli.LoadConfig("")            // 默认读 ~/.agentid/config.yaml
//	client, _ := cli.NewClient(cfg)         // 根据 cfg.Mode 路由
//	resp, err := client.RegisterAgent(ctx, &cli.RegisterRequest{...})
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentid-chain/agentid-chain/core/backend"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
	"gopkg.in/yaml.v3"
)

// =============================================================================
// Config（CLI 层：~/.agentid/config.yaml）
// =============================================================================

// Mode 客户端模式。
type Mode string

const (
	ModeLocal  Mode = "local"  // 直连本地后端（默认；适合开发 / 调试）
	ModeRemote Mode = "remote" // HTTP 调远端 gateway
)

// Config CLI 配置。
type Config struct {
	// Mode 客户端模式（local / remote）。
	Mode Mode `yaml:"mode"`
	// Gateway 远端 gateway 地址（Mode=remote 时生效；默认 http://localhost:8080）。
	Gateway string `yaml:"gateway"`
	// APIKey 可选：与 gateway 通信时的 API Key（X-API-Key 头）。
	APIKey string `yaml:"api_key"`
	// Backend 后端类型（Mode=local 时生效：local / mock / onchain / hybrid）。
	Backend string `yaml:"backend"`
	// Output 全局默认输出格式（json / table / yaml）。
	Output string `yaml:"output"`
	// Timeout HTTP 超时（秒；默认 30s）。
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

// LoadConfig 加载配置（路径为空时读 ~/.agentid/config.yaml；不存在则返回默认值）。
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = defaultConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Mode:           ModeLocal,
		Gateway:        "http://localhost:8080",
		Backend:        "mock",
		Output:         "json",
		TimeoutSeconds: 30,
	}
}

func (c *Config) applyDefaults() {
	if c.Mode == "" {
		c.Mode = ModeLocal
	}
	if c.Gateway == "" {
		c.Gateway = "http://localhost:8080"
	}
	if c.Backend == "" {
		c.Backend = "mock"
	}
	if c.Output == "" {
		c.Output = "json"
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 30
	}
}

// ApplyDefaults 公开版（包外调用）。
func (c *Config) ApplyDefaults() { c.applyDefaults() }

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".agentid", "config.yaml")
}

// EnsureConfigFile 若 ~/.agentid/config.yaml 不存在则创建（含默认内容）。
func EnsureConfigFile() (string, error) {
	path := defaultConfigPath()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal default config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// =============================================================================
// Client（双模式）
// =============================================================================

// Client 是 CLI 与后端的统一接口。
type Client struct {
	cfg    *Config
	local  backend.IdentityBackend   // Mode=local 时使用
	http   *http.Client              // Mode=remote 时使用
	tok    *aapToken                 // AAP token 缓存（remote 模式使用）
}

// NewClient 构造客户端。
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.applyDefaults()

	c := &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
		tok: &aapToken{},
	}
	if cfg.Mode == ModeLocal {
		be, err := buildLocalBackend(cfg.Backend)
		if err != nil {
			return nil, fmt.Errorf("build local backend: %w", err)
		}
		c.local = be
	}
	return c, nil
}

// Mode 客户端模式。
func (c *Client) Mode() Mode { return c.cfg.Mode }

// Config 暴露配置（只读）。
func (c *Client) Config() *Config { return c.cfg }

// Close 关闭。
func (c *Client) Close(ctx context.Context) error {
	if c.local != nil {
		return c.local.Close(ctx)
	}
	return nil
}

// buildLocalBackend 构造本地后端。
func buildLocalBackend(typ string) (backend.IdentityBackend, error) {
	switch strings.ToLower(typ) {
	case "local":
		return backend.NewBackend(backend.Config{Type: backend.TypeLocal})
	case "mock":
		return backend.NewBackend(backend.Config{Type: backend.TypeMock})
	case "onchain":
		// 本地 onchain 默认用 mock 适配器（开发者友好）
		return backend.NewBackend(backend.Config{
			Type:         backend.TypeOnchain,
			ChainAdapter: mock.NewMockAdapter(),
		})
	case "hybrid":
		return backend.NewBackend(backend.Config{
			Type:         backend.TypeHybrid,
			ChainAdapter: mock.NewMockAdapter(),
		})
	default:
		return nil, fmt.Errorf("unknown backend %q", typ)
	}
}

// =============================================================================
// API DTO（与 gateway HTTP API 对齐 / 与 core/backend 解耦）
// =============================================================================

// RegisterRequest 注册请求。
type RegisterRequest struct {
	Name       string            `json:"name,omitempty"`
	Owner      string            `json:"owner"`
	Level      uint8             `json:"level"`
	Permission uint64            `json:"permission"`
	PublicKey  string            `json:"public_key"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// AgentCredential 注册凭证。
type AgentCredential struct {
	UUID       string    `json:"uuid"`
	Owner      string    `json:"owner"`
	Level      uint8     `json:"level"`
	State      string    `json:"state"`
	Permission uint64    `json:"permission"`
	PublicKey  string    `json:"public_key"`
	TxHash     string    `json:"tx_hash,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AgentInfo 查询结果。
type AgentInfo struct {
	UUID         string    `json:"uuid"`
	Owner        string    `json:"owner"`
	Level        uint8     `json:"level"`
	State        string    `json:"state"`
	Permission   uint64    `json:"permission"`
	PublicKey    string    `json:"public_key"`
	TxHash       string    `json:"tx_hash,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ChangeLog 审计日志。
type ChangeLog struct {
	UUID       string    `json:"uuid"`
	Action     string    `json:"action"`
	Actor      string    `json:"actor,omitempty"`
	OldValue   string    `json:"old_value,omitempty"`
	NewValue   string    `json:"new_value,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	TxHash     string    `json:"tx_hash,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ErrorResponse 错误响应。
type ErrorResponse struct {
	Error string `json:"error"`
}

// =============================================================================
// API 方法（统一接口；local / remote 自动路由）
// =============================================================================

// RegisterAgent 注册。
func (c *Client) RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
	if c.cfg.Mode == ModeLocal {
		return c.registerLocal(ctx, req)
	}
	var out AgentCredential
	if err := c.doJSON(ctx, http.MethodPost, "/api/v2/agents", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgentInfo 查询。
func (c *Client) GetAgentInfo(ctx context.Context, uuid string) (*AgentInfo, error) {
	if c.cfg.Mode == ModeLocal {
		return c.getInfoLocal(ctx, uuid)
	}
	var out AgentInfo
	if err := c.doJSON(ctx, http.MethodGet, "/api/v2/agents/"+uuid, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAgentLevel 升级。
func (c *Client) UpdateAgentLevel(ctx context.Context, uuid string, targetLevel uint8, reason string) error {
	if c.cfg.Mode == ModeLocal {
		return c.local.UpdateAgentLevel(ctx, uuid, targetLevel, reason)
	}
	body := map[string]any{"level": targetLevel, "reason": reason}
	var out map[string]any
	return c.doJSON(ctx, http.MethodPost, "/api/v2/agents/"+uuid+"/upgrade", body, &out)
}

// BanAgent 封禁。
func (c *Client) BanAgent(ctx context.Context, uuid string, reason string) error {
	if c.cfg.Mode == ModeLocal {
		return c.local.BanAgent(ctx, uuid, reason)
	}
	body := map[string]any{"reason": reason}
	var out map[string]any
	return c.doJSON(ctx, http.MethodPost, "/api/v2/agents/"+uuid+"/ban", body, &out)
}

// UnbanAgent 解封。
func (c *Client) UnbanAgent(ctx context.Context, uuid string) error {
	if c.cfg.Mode == ModeLocal {
		return c.local.UnbanAgent(ctx, uuid)
	}
	var out map[string]any
	return c.doJSON(ctx, http.MethodPost, "/api/v2/agents/"+uuid+"/unban", nil, &out)
}

// UnregisterAgent 注销。
func (c *Client) UnregisterAgent(ctx context.Context, uuid string) error {
	if c.cfg.Mode == ModeLocal {
		return c.local.UnregisterAgent(ctx, uuid)
	}
	var out map[string]any
	return c.doJSON(ctx, http.MethodPost, "/api/v2/agents/"+uuid+"/unregister", nil, &out)
}

// GetChangeLogs 审计日志。
func (c *Client) GetChangeLogs(ctx context.Context, uuid string) ([]ChangeLog, error) {
	if c.cfg.Mode == ModeLocal {
		logs, err := c.local.GetChangeLogs(ctx, uuid)
		if err != nil {
			return nil, err
		}
		out := make([]ChangeLog, len(logs))
		for i, l := range logs {
			out[i] = ChangeLog{
				UUID: l.UUID, Action: l.Action, Actor: l.Actor,
				OldValue: l.OldValue, NewValue: l.NewValue, Reason: l.Reason,
				TxHash: l.TxHash, OccurredAt: l.OccurredAt,
			}
		}
		return out, nil
	}
	var out []ChangeLog
	if err := c.doJSON(ctx, http.MethodGet, "/api/v2/agents/"+uuid+"/logs", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BatchGetAgentInfo 批量查询。
func (c *Client) BatchGetAgentInfo(ctx context.Context, uuids []string) (map[string]*AgentInfo, error) {
	if c.cfg.Mode == ModeLocal {
		infos, err := c.local.BatchGetAgentInfo(ctx, uuids)
		if err != nil {
			return nil, err
		}
		out := make(map[string]*AgentInfo, len(infos))
		for k, v := range infos {
			out[k] = infoToDTO(v)
		}
		return out, nil
	}
	body := map[string]any{"uuids": uuids}
	var out map[string]*AgentInfo
	if err := c.doJSON(ctx, http.MethodPost, "/api/v2/agents/batch", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// HealthCheck 健康检查。
func (c *Client) HealthCheck(ctx context.Context) error {
	if c.cfg.Mode == ModeLocal {
		// 本地：mock chain 的 HealthCheck
		if c.local != nil {
			_ = c.local // 后端不暴露 HealthCheck
		}
		return nil
	}
	return c.doJSON(ctx, http.MethodGet, "/healthz", nil, nil)
}

// =============================================================================
// Local 路径
// =============================================================================

func (c *Client) registerLocal(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
	cred, err := c.local.RegisterAgent(ctx, &backend.RegisterRequest{
		Owner:      req.Owner,
		Level:      req.Level,
		Permission: req.Permission,
		PublicKey:  req.PublicKey,
		Metadata:   req.Metadata,
	})
	if err != nil {
		return nil, err
	}
	return credToDTO(cred), nil
}

func (c *Client) getInfoLocal(ctx context.Context, uuid string) (*AgentInfo, error) {
	info, err := c.local.GetAgentInfo(ctx, uuid)
	if err != nil {
		return nil, err
	}
	return infoToDTO(info), nil
}

// =============================================================================
// HTTP helpers
// =============================================================================

// doJSON 通用 JSON HTTP 调用。
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.cfg.Gateway, "/")+path, reqBody)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		if out != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("decode %s: %w", string(respBody), err)
			}
		}
		return nil
	}

	// 错误响应
	var errResp ErrorResponse
	_ = json.Unmarshal(respBody, &errResp)
	msg := errResp.Error
	if msg == "" {
		msg = string(respBody)
	}
	return fmt.Errorf("gateway %d: %s", resp.StatusCode, msg)
}

// =============================================================================
// DTO 转换
// =============================================================================

func credToDTO(cred *backend.AgentCredential) *AgentCredential {
	if cred == nil {
		return nil
	}
	return &AgentCredential{
		UUID:       cred.UUID,
		Owner:      cred.Owner,
		Level:      cred.Level,
		State:      cred.State,
		Permission: cred.Permission,
		PublicKey:  cred.PublicKey,
		TxHash:     cred.TxHash,
		CreatedAt:  cred.CreatedAt,
		UpdatedAt:  cred.UpdatedAt,
	}
}

func infoToDTO(info *backend.AgentInfo) *AgentInfo {
	if info == nil {
		return nil
	}
	return &AgentInfo{
		UUID:         info.UUID,
		Owner:        info.Owner,
		Level:        info.Level,
		State:        info.State,
		Permission:   info.Permission,
		PublicKey:    info.PublicKey,
		TxHash:       info.TxHash,
		RegisteredAt: info.RegisteredAt,
		UpdatedAt:    info.UpdatedAt,
	}
}

// 编译期检查
var _ chain_adapter.ChainType = chain_adapter.ChainTypeMock // 防止 chain_adapter 被剔除
