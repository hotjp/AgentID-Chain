// Package storage 核心接口定义。
//
// StorageClient 是 L1 层的统一抽象，向上层（L2-L5）屏蔽底层存储细节。
// 所有身份、权限、审计数据的读写都必须通过此接口进行。
package storage

import (
	"context"
	"errors"
	"time"
)

// =============================================================================
// 错误定义（L1 错误码，与 docs/AgentID-Chain-技术文档-v2.0.1.md §11.2 对齐）
// =============================================================================

// ErrNotFound 资源不存在（UUID/Token/Key 等查询不到）。
var ErrNotFound = errors.New("storage: not found")

// ErrConflict 资源冲突（重复主键、唯一索引冲突）。
var ErrConflict = errors.New("storage: conflict")

// ErrUnavailable 底层存储不可用（连接断开、超时、只读副本等）。
var ErrUnavailable = errors.New("storage: unavailable")

// ErrTxFailed 事务执行失败。
var ErrTxFailed = errors.New("storage: transaction failed")

// =============================================================================
// 通用类型
// =============================================================================

// HealthStatus 健康检查状态。
type HealthStatus struct {
	Healthy   bool
	Latency   time.Duration
	Message   string
	CheckedAt time.Time
}

// =============================================================================
// StorageClient 统一接口
// =============================================================================

// Client 是 L1 层的统一抽象接口。
//
// 实现示例：
//   - internal/storage/postgres/client.go  ← PostgreSQL + ent
//   - internal/storage/redis/client.go     ← Redis 缓存层
//   - internal/storage/outbox/client.go    ← Outbox 转发
//
// 业务层（L2-L5）只能依赖此接口，禁止直接 import 具体实现。
//
//nolint:revive // stutter 允许：对外 public API 用包名前缀强调语义
type Client interface {
	// Identity 提供 Agent 身份的 CRUD + 状态变更。
	Identity() IdentityStore

	// Permission 提供 RBAC 权限位的读写。
	Permission() PermissionStore

	// Audit 提供操作审计日志的追加与查询。
	Audit() AuditStore

	// Nonce 提供 AAP / A2A Challenge 的 Nonce 防重放存储。
	Nonce() NonceStore

	// Revocation 提供 A2A Token 撤销列表的查询与维护。
	Revocation() RevocationStore

	// Cache 提供读穿 / 写穿缓存（可降级为 no-op）。
	Cache() CacheStore

	// HealthCheck 健康检查，用于 L5 暴露 /healthz。
	HealthCheck(ctx context.Context) HealthStatus

	// Close 关闭底层连接（优雅关闭）。
	Close(ctx context.Context) error
}

// =============================================================================
// 子接口定义（每个接口职责单一）
// =============================================================================

// IdentityStore Agent 身份存储。
type IdentityStore interface {
	// GetAgent 查询 Agent 实体（by UUID）。
	GetAgent(ctx context.Context, uuid string) (*AgentRecord, error)
	// PutAgent 写入/更新 Agent 实体。
	PutAgent(ctx context.Context, rec *AgentRecord) error
	// ListAgentsByOwner 列出某 owner 名下所有 Agent。
	ListAgentsByOwner(ctx context.Context, owner string) ([]*AgentRecord, error)
	// BatchGetAgents 批量查询（A2A 互认用）。
	BatchGetAgents(ctx context.Context, uuids []string) (map[string]*AgentRecord, error)
}

// PermissionStore RBAC 权限位存储。
type PermissionStore interface {
	// GetPermissions 读取 Agent 的位掩码权限。
	GetPermissions(ctx context.Context, uuid string) (uint64, error)
	// SetPermissions 覆盖写入位掩码权限。
	SetPermissions(ctx context.Context, uuid string, mask uint64) error
	// GrantPermission 增量授予一位权限。
	GrantPermission(ctx context.Context, uuid string, bit uint) error
	// RevokePermission 增量撤销一位权限。
	RevokePermission(ctx context.Context, uuid string, bit uint) error
}

// AuditStore 审计日志。
type AuditStore interface {
	// Append 追加一条审计记录。
	Append(ctx context.Context, entry *AuditEntry) error
	// Query 按 UUID + 时间范围查询。
	Query(ctx context.Context, uuid string, from, to time.Time) ([]*AuditEntry, error)
	// Count 统计数量（用于监控/合规导出）。
	Count(ctx context.Context, uuid string) (int64, error)
}

// NonceStore AAP / A2A Challenge Nonce 防重放。
type NonceStore interface {
	// Store 写入 nonce，TTL 内不允许重复。
	Store(ctx context.Context, nonce string, ttl time.Duration) error
	// Exists 查询 nonce 是否已存在（存在 = 重复）。
	Exists(ctx context.Context, nonce string) (bool, error)
	// Consume 校验并消费 nonce（一次性）。
	Consume(ctx context.Context, nonce string) error
}

// RevocationStore A2A Token 撤销列表。
type RevocationStore interface {
	// Revoke 撤销 token（写入黑名单 + 过期时间）。
	Revoke(ctx context.Context, jti string, expiresAt time.Time) error
	// IsRevoked 查询 token 是否已撤销。
	IsRevoked(ctx context.Context, jti string) (bool, error)
	// PurgeExpired 清理已过期条目。
	PurgeExpired(ctx context.Context) (int64, error)
}

// CacheStore 通用缓存。
type CacheStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	// Incr 原子自增（限流器用）。
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// =============================================================================
// 记录类型（L2 Domain 实体的存储表示；L2 实现里会转 Agent 实体）
// =============================================================================

// AgentRecord 持久化的 Agent 记录。
type AgentRecord struct {
	UUID         string
	Owner        string
	Level        uint8
	Permissions  uint64
	State        string // registered / active / banned / unregistered
	TxHash       string // 链上注册时的交易哈希（链下后端可为空）
	PublicKey    string // Ed25519 公钥
	RegisteredAt time.Time
	UpdatedAt    time.Time
}

// AuditEntry 一条审计记录。
type AuditEntry struct {
	ID        string
	UUID      string
	Actor     string
	Action    string
	Resource  string
	OldValue  string
	NewValue  string
	Result    string
	LatencyMs int64
	Timestamp time.Time
}
