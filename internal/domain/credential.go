// Package domain AgentCredential 值对象。
//
// AgentCredential 是 Agent 身份凭证（用于 AAP 握手 / 跨服务签名）。
// 它是不可变的"数据袋"（value object）：所有字段在构造后不可变。
//
// 字段：
//   - UUID      凭证唯一 ID（与 Agent UUID 不同；凭证可独立轮换）
//   - Signature EdDSA 签名 hex（与公钥配对）
//   - IssuedAt  颁发时间
//   - ExpiresAt 过期时间（绝对时间）
//   - IssuerDID 颁发者 DID
//
// 设计要点：
//   - 零第三方依赖：hex 用 stdlib encoding/hex
//   - 不变量：ExpiresAt > IssuedAt；UUID 非空；Signature 非空
//   - 不可变：所有字段无 setter；变更通过 IssueNew 构造新对象
package domain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrCredentialExpired 凭证已过期。
var ErrCredentialExpired = errors.New("domain: credential expired")

// ErrCredentialNotYetValid 凭证尚未生效（IssuedAt 在未来）。
var ErrCredentialNotYetValid = errors.New("domain: credential not yet valid")

// ErrInvalidSignature 签名格式错（非 hex / 长度不对 / 为空）。
var ErrInvalidSignature = errors.New("domain: invalid signature")

// EdDSA signature 长度（Ed25519 签名为 64 字节 → 128 hex chars）。
const eddsaSignatureHexLen = 128

// =============================================================================
// AgentCredential 值对象
// =============================================================================

// AgentCredential Agent 身份凭证。
//
// 不可变；并发安全（所有字段值类型）。
type AgentCredential struct {
	UUID      UUID      // 凭证 UUID
	Signature string    // EdDSA 签名 hex
	IssuedAt  time.Time // 颁发时间
	ExpiresAt time.Time // 过期时间
	IssuerDID string    // 颁发者 DID
	AgentUUID UUID      // 归属 Agent 的 UUID（冗余便于反查）
}

// NewAgentCredential 构造 AgentCredential。
//
// 字段校验：
//   - UUID 非零
//   - AgentUUID 非零
//   - Signature 合法 hex + 长度 128（Ed25519）
//   - ExpiresAt > IssuedAt
func NewAgentCredential(
	uuid UUID,
	agentUUID UUID,
	signature string,
	issuerDID string,
	issuedAt, expiresAt time.Time,
) (*AgentCredential, error) {
	if uuid.IsZero() {
		return nil, fmt.Errorf("%w: uuid is empty", ErrInvalidUUID)
	}
	if agentUUID.IsZero() {
		return nil, fmt.Errorf("%w: agent uuid is empty", ErrInvalidUUID)
	}
	if err := validateSignature(signature); err != nil {
		return nil, err
	}
	if issuerDID == "" {
		return nil, errors.New("domain: issuer_did is empty")
	}
	if !expiresAt.After(issuedAt) {
		return nil, fmt.Errorf("domain: expires_at must be after issued_at (issued=%s expires=%s)",
			issuedAt, expiresAt)
	}
	return &AgentCredential{
		UUID:      uuid,
		AgentUUID: agentUUID,
		Signature: signature,
		IssuerDID: issuerDID,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}, nil
}

// validateSignature 校验 EdDSA 签名 hex。
func validateSignature(sig string) error {
	if sig == "" {
		return fmt.Errorf("%w: empty", ErrInvalidSignature)
	}
	raw, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("%w: not hex: %v", ErrInvalidSignature, err)
	}
	if len(raw) != 64 { // Ed25519 = 64 bytes
		return fmt.Errorf("%w: length=%d, want 64", ErrInvalidSignature, len(raw))
	}
	_ = eddsaSignatureHexLen // 显式引用常量（lint hint）
	return nil
}

// IsValid 当前时刻是否有效（未过期 + 已生效）。
func (c *AgentCredential) IsValid(now time.Time) bool {
	return !c.IsExpired(now) && c.IsEffective(now)
}

// IsExpired 是否已过期。
func (c *AgentCredential) IsExpired(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}

// IsEffective 是否已生效（IssuedAt <= now）。
func (c *AgentCredential) IsEffective(now time.Time) bool {
	return !now.Before(c.IssuedAt)
}

// RemainingLifetime 剩余有效时间（负数表示已过期）。
func (c *AgentCredential) RemainingLifetime(now time.Time) time.Duration {
	return c.ExpiresAt.Sub(now)
}

// =============================================================================
// 凭证链（Issuance）— L4 Service 颁发新凭证时使用
// =============================================================================

// IssueNew 颁发新凭证（轮换用）。
//
// 设计：
//   - 保留 AgentUUID 与 IssuerDID
//   - 新 UUID + 新 Signature + 新时间窗
//   - 旧凭证不变（不可变）
//
// 业务规则：
//   - 旧凭证仍然有效直到 ExpiresAt 过期（轮换是"叠加"非"替换"）
func (c *AgentCredential) IssueNew(
	newUUID UUID,
	newSignature string,
	newIssuerDID string,
	issuedAt, expiresAt time.Time,
) (*AgentCredential, error) {
	return NewAgentCredential(newUUID, c.AgentUUID, newSignature, newIssuerDID, issuedAt, expiresAt)
}
