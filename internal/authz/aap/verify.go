// Package aap AAP Response 验签 + Proof 颁发。
//
// 协议流程（v2.0.1 §3.3.2.5）：
//
//	Client                           Server (this file)
//	  │                                    │
//	  │── POST /aap/challenge ────────────▶│  (P5.3 Generator.Generate)
//	  │◀─ {challenge_id, nonce, ts, ...} ──│
//	  │                                    │
//	  │  [client signs payload with       │
//	  │   its Ed25519 private key]         │
//	  │                                    │
//	  │── POST /aap/verify ───────────────▶│  (P5.4 Verifier.Verify)
//	  │   {challenge_id, response,        │
//	  │    agent_pubkey, agent_uuid}       │
//	  │                                    │  Validate: id exists, ts valid,
//	  │                                    │  nonce matches, sig verifies
//	  │◀─ {proof, expires_at, ...} ────────│  Issue Proof (P5.4 Verifier.IssueProof)
//	  │                                    │
//
// 关键约束：
//   - 一次性：Challenge 被消费后立即从 store 删除
//   - 时间窗口：Response 必须在 Challenge 颁发后 ResponseMaxTTL 内提交
//   - 签名内容：Client 必须签名 challenge_id || nonce || issued_at || agent_uuid
//   - Proof 颁发后由调用方写入 HTTP Response / 注入 Header
package aap

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/agentid-chain/agentid-chain/internal/domain"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrResponseExpired Response 提交超时（超过 ResponseMaxTTL）。
var ErrResponseExpired = errors.New("aap: response expired")

// ErrNonceMismatch Nonce 不匹配（可能重放）。
var ErrNonceMismatch = errors.New("aap: nonce mismatch")

// ErrSignatureInvalid 签名验证失败。
var ErrSignatureInvalid = errors.New("aap: signature invalid")

// ErrEmptyAgentPubKey Agent 公钥为空。
var ErrEmptyAgentPubKey = errors.New("aap: empty agent public key")

// ErrInvalidAgentPubKey Agent 公钥格式不合法（非 32 字节 Ed25519）。
var ErrInvalidAgentPubKey = errors.New("aap: invalid agent public key")

// ErrEmptyAgentUUID Agent UUID 为空。
var ErrEmptyAgentUUID = errors.New("aap: empty agent uuid")

// =============================================================================
// Response 入参
// =============================================================================

// VerifyInput 验签入参。
type VerifyInput struct {
	// ChallengeID 服务器颁发的 challenge ID
	ChallengeID string
	// Response client 用自己的 Ed25519 私钥对 (challenge_id || nonce || issued_at || agent_uuid) 的签名（base64）
	Response string
	// AgentPubKey client 的 Ed25519 公钥（32 字节；hex 或 base64 编码）
	AgentPubKey string
	// AgentUUID 客户端标识的 Agent UUID（32 字符 hex）
	AgentUUID string
	// Now 业务时间（注入便于测试）
	Now time.Time
}

// VerifyOutput 验签出参。
type VerifyOutput struct {
	// Challenge 消费前的 challenge（供审计/调试）
	Challenge *Challenge
	// AgentUUID 通过的 Agent UUID
	AgentUUID string
	// AgentPubKey 通过的 Agent 公钥（32 字节原始）
	AgentPubKey ed25519.PublicKey
	// VerifiedAt 验签通过时间
	VerifiedAt time.Time
}

// =============================================================================
// Verifier Response 验签器
// =============================================================================

// Verifier 验签器配置。
type VerifierConfig struct {
	// Generator 关联的 challenge 生成器（用于 LoadChallenge / ConsumeChallenge）
	Generator *Generator
	// ResponseMaxTTL Response 提交的最大时间窗口（默认 10 分钟）
	ResponseMaxTTL time.Duration
	// Clock 时间源
	Clock func() time.Time
}

// Verifier AAP Response 验签器 + Proof 颁发器。
type Verifier struct {
	gen          *Generator
	cfg          VerifierConfig
	domainPubKey ed25519.PublicKey // 服务器域公钥（用于 VerifyProof 验签 domain_sig）
}

// NewVerifier 构造验签器。
func NewVerifier(gen *Generator, cfg VerifierConfig) (*Verifier, error) {
	if gen == nil {
		return nil, errors.New("aap: nil generator")
	}
	if cfg.ResponseMaxTTL <= 0 {
		cfg.ResponseMaxTTL = 10 * time.Minute
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	cfg.Generator = gen
	// 提取服务器域公钥（用于 proof 验签）
	pub, ok := gen.cfg.DomainKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("aap: cannot extract domain public key")
	}
	return &Verifier{gen: gen, cfg: cfg, domainPubKey: pub}, nil
}

// Verify 验签 + 消费 challenge。
//
// 步骤：
//  1. 解析入参（challenge_id / response / agent_pubkey / agent_uuid）
//  2. 解析 client 公钥（先于 challenge 消费，避免浪费）
//  3. 加载 challenge（一次性 — 加载后立即删除）
//  4. 检查 ts 在 ResponseMaxTTL 窗口内
//  5. 验签：ed25519.Verify(pubkey, payload, response)
//  6. 返回 VerifyOutput（含 AgentPubKey 原始字节）
func (v *Verifier) Verify(ctx context.Context, in VerifyInput) (*VerifyOutput, error) {
	// 1. 基础校验
	if in.ChallengeID == "" {
		return nil, ErrInvalidChallengeID
	}
	if in.AgentUUID == "" {
		return nil, ErrEmptyAgentUUID
	}
	if err := domain.UUID(in.AgentUUID).Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEmptyAgentUUID, in.AgentUUID)
	}
	if in.AgentPubKey == "" {
		return nil, ErrEmptyAgentPubKey
	}
	if in.Response == "" {
		return nil, ErrSignatureInvalid
	}
	if in.Now.IsZero() {
		in.Now = v.cfg.Clock()
	}

	// 2. 解析 client 公钥（先于 challenge 消费）
	pub, err := parsePubKey(in.AgentPubKey)
	if err != nil {
		return nil, err
	}

	// 3. 加载 + 消费 challenge（一次性）
	c, err := v.gen.ConsumeChallenge(ctx, in.ChallengeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidChallenge, in.ChallengeID)
	}

	// 4. 时间窗口
	if c.IsExpired(in.Now) {
		return nil, fmt.Errorf("%w: issued=%s now=%s", ErrResponseExpired, c.IssuedAt, in.Now)
	}
	if in.Now.Sub(c.IssuedAt) > v.cfg.ResponseMaxTTL {
		return nil, fmt.Errorf("%w: delta=%v", ErrResponseExpired, in.Now.Sub(c.IssuedAt))
	}

	// 5. 验签
	sig, err := base64.RawURLEncoding.DecodeString(in.Response)
	if err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrSignatureInvalid, err)
	}
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, in.AgentUUID)
	if !ed25519.Verify(pub, payload, sig) {
		return nil, ErrSignatureInvalid
	}

	return &VerifyOutput{
		Challenge:   c,
		AgentUUID:   in.AgentUUID,
		AgentPubKey: pub,
		VerifiedAt:  in.Now,
	}, nil
}

// =============================================================================
// Proof 颁发
// =============================================================================

// Proof AAP Proof 数据结构（v2.0.1 §3.3.2.6）。
//
// Proof 是 AAP 握手成功的产物；携带 agent_uuid / 颁发时间 / 过期时间 / 服务器签名。
// 客户端在后续请求中以 AAP-Proof: <base64-proof> Header 提交。
type Proof struct {
	// ProofID 唯一 ID
	ProofID string
	// AgentUUID 颁发对象
	AgentUUID string
	// AgentPubKey 客户端公钥（base64 编码；便于 JSON 序列化）
	AgentPubKey string
	// IssuedAt 颁发时间
	IssuedAt time.Time
	// ExpiresAt 过期时间
	ExpiresAt time.Time
	// DomainSig 服务器对 (proof_id || agent_uuid || issued_at || expires_at) 的 Ed25519 签名
	DomainSig string
}

// IsExpired 业务判定：是否过期。
func (p *Proof) IsExpired(now time.Time) bool {
	return !now.Before(p.ExpiresAt)
}

// IssueProof 给定验签通过的结果 + TTL，颁发 Proof。
//
// ProofID 复用 challenge_id（同一握手的延伸）；客户端缓存后可避免重复握手。
func (v *Verifier) IssueProof(out *VerifyOutput, ttl time.Duration) (*Proof, error) {
	if out == nil {
		return nil, errors.New("aap: nil verify output")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := v.cfg.Clock()
	p := &Proof{
		ProofID:     out.Challenge.ChallengeID,
		AgentUUID:   out.AgentUUID,
		AgentPubKey: base64.RawURLEncoding.EncodeToString(out.AgentPubKey),
		IssuedAt:    now,
		ExpiresAt:   now.Add(ttl),
	}
	payload := proofPayload(p.ProofID, p.AgentUUID, p.IssuedAt, p.ExpiresAt)
	sig := ed25519.Sign(v.gen.cfg.DomainKey, payload)
	p.DomainSig = base64.RawURLEncoding.EncodeToString(sig)
	return p, nil
}

// VerifyProof 验签 Proof 本身（用于 middleware 阶段）。
//
// 步骤：
//  1. 检查 ExpiresAt
//  2. 重算 payload
//  3. ed25519.Verify（用服务器域公钥验签 domain_sig）
//  4. 解析 AgentPubKey（用于上游业务使用）
func (v *Verifier) VerifyProof(p *Proof) error {
	if p == nil {
		return errors.New("aap: nil proof")
	}
	if p.IsExpired(v.cfg.Clock()) {
		return fmt.Errorf("%w: proof expired", ErrResponseExpired)
	}
	// domain_sig 是服务器私钥签的，用服务器公钥验
	sig, err := base64.RawURLEncoding.DecodeString(p.DomainSig)
	if err != nil {
		return fmt.Errorf("%w: decode proof sig: %v", ErrSignatureInvalid, err)
	}
	payload := proofPayload(p.ProofID, p.AgentUUID, p.IssuedAt, p.ExpiresAt)
	if !ed25519.Verify(v.domainPubKey, payload, sig) {
		return ErrSignatureInvalid
	}
	// 顺便校验 agent pub key 格式
	if _, err := parsePubKey(p.AgentPubKey); err != nil {
		return err
	}
	return nil
}

// =============================================================================
// 工具函数
// =============================================================================

// responsePayload 构造客户端待签名 payload。
//
// 格式：challenge_id || ":" || nonce || ":" || issued_at || ":" || agent_uuid
func responsePayload(challengeID, nonce string, issuedAt time.Time, agentUUID string) []byte {
	return []byte(challengeID + ":" + nonce + ":" + issuedAt.Format(time.RFC3339Nano) + ":" + agentUUID)
}

// proofPayload 构造 Proof 签名 payload。
func proofPayload(proofID, agentUUID string, issuedAt, expiresAt time.Time) []byte {
	return []byte(
		proofID + ":" + agentUUID + ":" +
			issuedAt.Format(time.RFC3339Nano) + ":" +
			expiresAt.Format(time.RFC3339Nano),
	)
}

// parsePubKey 解析 client 公钥。
//
// 支持 hex（64 字符）和 base64（44 字符 / 64 字符 RawURL）。
// Ed25519 公钥固定 32 字节。
func parsePubKey(s string) (ed25519.PublicKey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ErrEmptyAgentPubKey
	}
	var b []byte
	var err error
	if len(s) == 64 || (len(s) == 66 && strings.HasPrefix(s, "0x")) {
		// hex
		clean := s
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			clean = s[2:]
		}
		b, err = hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("%w: hex decode: %v", ErrInvalidAgentPubKey, err)
		}
	} else {
		// base64 (RawURL or Std)
		if b, err = base64.RawURLEncoding.DecodeString(s); err != nil {
			if b, err = base64.StdEncoding.DecodeString(s); err != nil {
				return nil, fmt.Errorf("%w: base64 decode: %v", ErrInvalidAgentPubKey, err)
			}
		}
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: got %d bytes, want %d", ErrInvalidAgentPubKey, len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

// CacheErrMiss re-export 给 middleware 用。
var CacheErrMiss = cache.ErrMiss
