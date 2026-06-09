// Package backend: 身份后端统一抽象（IdentityBackend）。
//
// 设计：所有身份操作（注册/查询/升级/封禁/注销/审计）都通过此接口下发，
// 业务层（service/）完全无感知存储后端类型。三个实现：
//
//   - LocalBackend    ：纯本地（PG + Redis + 审计）
//   - OnchainBackend  ：链上（FISCO / Polygon / BSC / mock）
//   - HybridBackend   ：写上链 + 查本地（开放生态 + 高性能读）
//
// 用法（典型 wire）：
//
//	cfg := backend.Config{Type: backend.TypeLocal, Postgres: ...}
//	be, err := backend.New(cfg)
//	cred, err := be.RegisterAgent(ctx, &backend.RegisterRequest{...})
package backend

import (
	"context"
	"time"
)

// =============================================================================
// 后端类型
// =============================================================================

// BackendType 后端类型枚举。
type BackendType string

const (
	TypeLocal   BackendType = "local"   // 纯本地（PG + Redis）
	TypeOnchain BackendType = "onchain" // 纯链上（ChainAdapter）
	TypeHybrid  BackendType = "hybrid"  // 写上链 + 查本地
	TypeMock    BackendType = "mock"    // 内存 mock（仅测试）
)

// =============================================================================
// 通用错误
// =============================================================================

// ErrBackendUnavailable 后端不可用（DSN 空 / 链节点未连 / etc）。
var ErrBackendUnavailable = errorsNew("backend: unavailable")

// ErrAgentNotFound agent 不存在。
var ErrAgentNotFound = errorsNew("backend: agent not found")

// ErrAgentExists agent 已存在（重复注册）。
var ErrAgentExists = errorsNew("backend: agent already exists")

// ErrInvalidState 状态机非法转换。
var ErrInvalidState = errorsNew("backend: invalid state transition")

// =============================================================================
// 入参 / 出参
// =============================================================================

// RegisterRequest 注册请求。
type RegisterRequest struct {
	Owner      string
	Level      uint8
	Permission uint64
	PublicKey  string
	Metadata   map[string]string
}

// AgentCredential 注册凭证。
type AgentCredential struct {
	UUID       string
	Owner      string
	Level      uint8
	State      string
	Permission uint64
	PublicKey  string
	TxHash     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AgentInfo 完整信息（查询返回）。
type AgentInfo struct {
	UUID         string
	Owner        string
	Level        uint8
	State        string
	Permission   uint64
	PublicKey    string
	TxHash       string
	RegisteredAt time.Time
	UpdatedAt    time.Time
}

// ChangeLog 变更日志（审计）。
type ChangeLog struct {
	UUID       string
	Action     string
	Actor      string
	OldValue   string
	NewValue   string
	Reason     string
	TxHash     string
	OccurredAt time.Time
}

// =============================================================================
// IdentityBackend 统一接口
// =============================================================================

// IdentityBackend 身份后端统一接口。
//
// 所有方法 ctx 第一参数；链上/链下实现保持接口级兼容。
type IdentityBackend interface {
	// RegisterAgent 注册 agent，返回凭证。
	RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error)
	// GetAgentInfo 查询 agent 完整信息。
	GetAgentInfo(ctx context.Context, uuid string) (*AgentInfo, error)
	// UpdateAgentLevel 升级 agent 等级（+1 only）。
	UpdateAgentLevel(ctx context.Context, uuid string, newLevel uint8, reason string) error
	// BanAgent 封禁 agent。
	BanAgent(ctx context.Context, uuid string, reason string) error
	// UnbanAgent 解封 agent。
	UnbanAgent(ctx context.Context, uuid string) error
	// UnregisterAgent 注销 agent（永久）。
	UnregisterAgent(ctx context.Context, uuid string) error
	// GetChangeLogs 查询变更日志。
	GetChangeLogs(ctx context.Context, uuid string) ([]ChangeLog, error)
	// BatchGetAgentInfo 批量查询（A2A 互认）。
	BatchGetAgentInfo(ctx context.Context, uuids []string) (map[string]*AgentInfo, error)
	// BackendType 后端类型标识。
	BackendType() BackendType
	// Close 关闭后端（关闭连接池等）。
	Close(ctx context.Context) error
}

// =============================================================================
// 错误构造（避免循环导入 errors 包）
// =============================================================================

func errorsNew(s string) error {
	return &backendError{msg: s}
}

type backendError struct{ msg string }

func (e *backendError) Error() string { return e.msg }

// Is 实现 errors.Is（仅按字符串匹配）。
func (e *backendError) Is(target error) bool {
	t, ok := target.(*backendError)
	if !ok {
		return false
	}
	return e.msg == t.msg
}
