// Package service 插件接口定义。
//
// 所有外部能力（链、验证码、审计、密钥）以接口形式定义。
// 实现位于 internal/plugins/ 子包或外部 plugin 模块，运行时由 L4 Service 注入。
package service

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

// =============================================================================
// 链适配器接口（实现位于 internal/plugins/chain/）
// =============================================================================

// ChainType 链类型标识。
type ChainType string

// 链驱动标识常量。
const (
	ChainMock    ChainType = "mock"    // 本地 mock（开发/测试）
	ChainFISCO   ChainType = "fisco"   // FISCO BCOS
	ChainPolygon ChainType = "polygon" // Polygon
	ChainBSC     ChainType = "bsc"     // BNB Smart Chain
)

// RegisterRequest 链上注册请求。
type RegisterRequest struct {
	UUID        domain.UUID
	Owner       domain.Owner
	Level       domain.LevelType
	Permissions uint64
	PublicKey   ed25519.PublicKey
	Metadata    map[string]string
}

// RegisterReceipt 链上注册回执。
type RegisterReceipt struct {
	TxHash      string
	BlockNumber uint64
	GasUsed     uint64
	ConfirmedAt time.Time
}

// ChainAdapter 多链适配器统一接口。
//
// 各实现：
//   - internal/plugins/chain/mock/    (P3.8)
//   - internal/plugins/chain/fisco/   (后续)
//   - internal/plugins/chain/polygon/ (后续)
//   - internal/plugins/chain/bsc/     (后续)
type ChainAdapter interface {
	// ChainType 返回链驱动标识。
	ChainType() ChainType

	// RegisterAgent 上链注册 Agent。
	RegisterAgent(ctx context.Context, req *RegisterRequest) (*RegisterReceipt, error)

	// UpdateLevel 链上更新 Level。
	UpdateLevel(ctx context.Context, uuid domain.UUID, newLevel domain.LevelType, reason string) (*RegisterReceipt, error)

	// BanAgent 链上封禁。
	BanAgent(ctx context.Context, uuid domain.UUID, reason string) (*RegisterReceipt, error)

	// UnbanAgent 链上解封。
	UnbanAgent(ctx context.Context, uuid domain.UUID) (*RegisterReceipt, error)

	// GetAgentState 查询链上最新状态（用于 reconcile）。
	GetAgentState(ctx context.Context, uuid domain.UUID) (*ChainAgentState, error)

	// HealthCheck 健康检查。
	HealthCheck(ctx context.Context) error
}

// ChainAgentState 链上 Agent 状态（用于对账）。
type ChainAgentState struct {
	UUID      domain.UUID
	Level     domain.LevelType
	Banned    bool
	TxHash    string
	UpdatedAt time.Time
}

// =============================================================================
// 验证码引擎接口（实现位于 internal/captcha/）
// =============================================================================

// Challenge 挑战内容。
type Challenge struct {
	ID        string            // 挑战 ID
	Topic     string            // 主题（verification/identity/...）
	Format    string            // 格式（haiku/quatrain/...）
	Target    map[string]any    // 目标约束（ASCII sum / word count / ...）
	TTL       time.Duration     // 有效期
	IssuedAt  time.Time         // 发放时间
	ExpiresAt time.Time         // 过期时间
	Metadata  map[string]string // 元信息（nonce/level/...）
}

// ChallengeResponse 响应内容。
type ChallengeResponse struct {
	ChallengeID string
	Response    string
	SubmittedAt time.Time
}

// VerifyResult 验证结果。
type VerifyResult struct {
	Passed     bool
	Score      float64
	Reason     string
	VerifiedAt time.Time
}

// CaptchaEngine 验证码引擎。
//
// 实现：
//   - internal/captcha/aap/          AAP 协议（Challenge-Response + EdDSA）
//   - internal/captcha/moltcaptcha/  MoltCaptcha SMHL 反向 CAPTCHA
type CaptchaEngine interface {
	// EngineName 返回引擎名（"aap" / "moltcaptcha"）。
	EngineName() string

	// Generate 生成挑战。
	Generate(ctx context.Context, topic, format string, difficulty int) (*Challenge, error)

	// Verify 校验响应。
	Verify(ctx context.Context, challenge *Challenge, resp *ChallengeResponse) (*VerifyResult, error)
}

// =============================================================================
// 审计通知接口（实现位于 internal/plugins/audit/）
// =============================================================================

// AuditEvent 审计事件载荷。
type AuditEvent struct {
	ID        string
	UUID      domain.UUID
	Actor     string
	Action    string
	Resource  string
	OldValue  string
	NewValue  string
	Result    string
	Metadata  map[string]string
	Timestamp time.Time
}

// AuditNotifier 审计通知（同步写 + 异步转发）。
type AuditNotifier interface {
	// Notify 同步写审计（落 PG）；异步转发到 SIEM/Webhook。
	Notify(ctx context.Context, event *AuditEvent) error
	// Close 关闭底层通道。
	Close() error
}

// =============================================================================
// 速率限制器接口（实现位于 internal/authz/ratelimit/）
// =============================================================================

// LimitDecision 限流决策。
type LimitDecision struct {
	Allowed    bool
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
	Reason     string
}

// RateLimiter 限流器。
type RateLimiter interface {
	// Allow 检查是否允许。
	Allow(ctx context.Context, key string, n int) (*LimitDecision, error)
	// Reset 重置计数器（管理员用）。
	Reset(ctx context.Context, key string) error
}

// =============================================================================
// 密钥提供者接口（实现位于 internal/plugins/keystore/）
// =============================================================================

// KeyProvider 密钥提供者（从 KMS / Vault / 本地文件加载）。
type KeyProvider interface {
	// DomainKey 加载 AAP 域签名密钥。
	DomainKey(ctx context.Context) (ed25519.PrivateKey, error)
	// A2AKey 加载 A2A Token 签名密钥。
	A2AKey(ctx context.Context) (ed25519.PrivateKey, error)
	// ChainKey 加载链上操作私钥。
	ChainKey(ctx context.Context) (ed25519.PrivateKey, error)
	// Rotate 轮换指定 key（返回新 key id）。
	Rotate(ctx context.Context, keyType string) (string, error)
}

// =============================================================================
// 身份后端接口（实现位于 internal/core/backend/）
// =============================================================================

// IdentityProvider 身份后端（local PG / onchain / hybrid）的统一查询接口。
//
// 业务用法：
//   - L4 工作流在事务内写完后，调用 Provider 来 reconcile / 验证
//   - L5 网关查询用 Provider.Load(uuid) 拿到完整 Agent 视图
//   - 反向索引：按 owner DID 查所有 agent
//
// 与 L1 Repository 的区别：
//   - L1 Repository = 原始 CRUD（直接对 ent.PG / mock store）
//   - IdentityProvider = 多 backend 统一抽象（PG 优先 / onchain 兜底 / hybrid 合并）
//
// 实现位于 internal/core/backend/（P8 批次）。
type IdentityProvider interface {
	// BackendName 返回后端名（"local" / "onchain" / "hybrid"）。
	BackendName() string

	// Load 加载 Agent（按 UUID）。
	Load(ctx context.Context, uuid domain.UUID) (*domain.Agent, error)

	// LoadByOwner 加载 owner 名下所有 agent（按 owner DID）。
	LoadByOwner(ctx context.Context, ownerDID string) ([]*domain.Agent, error)

	// LoadByPubKey 加载 agent（按公钥，用于反查 challenge response）。
	LoadByPubKey(ctx context.Context, pub ed25519.PublicKey) (*domain.Agent, error)

	// Exists 检查 agent 是否存在（轻量级，比 Load 省一次序列化）。
	Exists(ctx context.Context, uuid domain.UUID) (bool, error)

	// HealthCheck 健康检查。
	HealthCheck(ctx context.Context) error
}
