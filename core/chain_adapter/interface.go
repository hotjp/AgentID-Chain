// Package chain_adapter 链适配器统一抽象（BaseChainAdapter）。
//
// 设计：
//   - 所有链驱动（FISCO BCOS / Polygon / BSC / mock / ...）实现同一接口
//   - OnchainBackend / HybridBackend 通过本接口下发上链操作
//   - 入参 / 出参 / 错误以本包类型为准，避免对 internal/domain 的循环依赖
//
// 用法（典型 wire）：
//
//	adapter, _ := mock.NewMockAdapter()
//	be, _ := backend.NewOnchainBackend(adapter, cache)
//
// 实现：
//   - core/chain_adapter/mock/    （P8.9：开发 / 集成测试用）
//   - core/chain_adapter/fisco/   （后续）
//   - core/chain_adapter/polygon/ （后续）
//   - core/chain_adapter/bsc/     （后续）
package chain_adapter

import (
	"context"
	"time"
)

// =============================================================================
// 链类型 / 状态
// =============================================================================

// ChainType 链驱动标识。
type ChainType string

// 已知链驱动常量。
const (
	ChainTypeMock    ChainType = "mock"    // 本地 mock
	ChainTypeFISCO   ChainType = "fisco"   // FISCO BCOS
	ChainTypePolygon ChainType = "polygon" // Polygon
	ChainTypeBSC     ChainType = "bsc"     // BNB Smart Chain
)

// AgentState 链上 Agent 状态（用于对账）。
type AgentState string

// 链上状态常量。
const (
	StateActive    AgentState = "active"    // 正常
	StateBanned    AgentState = "banned"    // 封禁
	StateRevoked   AgentState = "revoked"   // 注销
	StateUnchanged AgentState = "unchanged" // 状态未变（UpdateLevel 等）
)

// =============================================================================
// 入参 / 出参
// =============================================================================

// RegisterRequest 上链注册请求。
type RegisterRequest struct {
	UUID        string            `json:"uuid"`
	Owner       string            `json:"owner"`
	Level       uint8             `json:"level"`
	Permission  uint64            `json:"permission"`
	PublicKey   string            `json:"public_key"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UpdateLevelRequest 链上等级变更请求。
type UpdateLevelRequest struct {
	UUID     string `json:"uuid"`
	NewLevel uint8  `json:"new_level"`
	Reason   string `json:"reason,omitempty"`
}

// BanRequest 链上封禁请求。
type BanRequest struct {
	UUID   string `json:"uuid"`
	Reason string `json:"reason,omitempty"`
}

// Receipt 上链回执（统一形态）。
type Receipt struct {
	TxHash      string    `json:"tx_hash"`
	BlockNumber uint64    `json:"block_number"`
	GasUsed     uint64    `json:"gas_used"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// AgentOnchain 链上 Agent 全量状态（对账用）。
type AgentOnchain struct {
	UUID       string     `json:"uuid"`
	Owner      string     `json:"owner"`
	Level      uint8      `json:"level"`
	State      AgentState `json:"state"`
	Permission uint64     `json:"permission"`
	PublicKey  string     `json:"public_key"`
	TxHash     string     `json:"tx_hash"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// =============================================================================
// 错误
// =============================================================================

// ErrChainUnavailable 链节点不可用。
type ErrChainUnavailable struct{ Reason string }

func (e *ErrChainUnavailable) Error() string { return "chain_adapter: unavailable: " + e.Reason }

// ErrAgentNotFoundOnchain 链上找不到 Agent。
type ErrAgentNotFoundOnchain struct{ UUID string }

func (e *ErrAgentNotFoundOnchain) Error() string { return "chain_adapter: agent not found onchain: " + e.UUID }

// ErrTxFailed 链上交易失败。
type ErrTxFailed struct{ Reason string }

func (e *ErrTxFailed) Error() string { return "chain_adapter: tx failed: " + e.Reason }

// =============================================================================
// BaseChainAdapter 接口
// =============================================================================

// BaseChainAdapter 多链适配器统一接口。
//
// 约束：
//   - 链上变更是同步的：Register/Update/Ban/Unban 必须在 ctx 范围内返回 Receipt
//   - HealthCheck 必须轻量级（HTTP ping / 节点 status）
//   - 状态查询（GetAgentState）须返回链上最新一次确认后的状态
type BaseChainAdapter interface {
	// ChainType 返回链驱动标识（"mock" / "fisco" / "polygon" / "bsc"）。
	ChainType() ChainType

	// RegisterAgent 上链注册 Agent。
	RegisterAgent(ctx context.Context, req *RegisterRequest) (*Receipt, error)

	// UpdateLevel 链上更新 Level。
	UpdateLevel(ctx context.Context, req *UpdateLevelRequest) (*Receipt, error)

	// BanAgent 链上封禁。
	BanAgent(ctx context.Context, req *BanRequest) (*Receipt, error)

	// UnbanAgent 链上解封。
	UnbanAgent(ctx context.Context, uuid string) (*Receipt, error)

	// RevokeAgent 链上注销。
	RevokeAgent(ctx context.Context, uuid string) (*Receipt, error)

	// GetAgentState 查询链上最新状态。
	GetAgentState(ctx context.Context, uuid string) (*AgentOnchain, error)

	// HealthCheck 健康检查。
	HealthCheck(ctx context.Context) error
}
