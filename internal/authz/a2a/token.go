// Package a2a A2A (Agent-to-Agent) Token 颁发与解析。
//
// 业务背景（v2.0.1 §4.3.3）：
//   - A2A Token 是 agent 之间调用的"身份凭证 + 授权令牌"
//   - 由 AgentID-Chain 颁发，签名算法 EdDSA（与 AAP Proof 同算法 / 不同 issuer / 不同 audience）
//   - 包含丰富 claims：iss / aud / sub / exp / jti / scope / trust_level / audit_id
//
// 与 AAP Proof 的区别：
//   - AAP Proof：客户端身份证明（"我是谁"）
//   - A2A Token：调用授权令牌（"我能干什么、信任度多高、审计 ID 多少"）
//
// JWT 结构（RFC 7519）：
//
//	<base64url(header)>.<base64url(claims)>.<base64url(sig)>
//
// header = {"alg":"EdDSA","typ":"JWT","kid":"<key-id>"}
// claims = {"iss":..,"aud":..,"sub":..,"iat":..,"exp":..,"jti":..,"scope":..,"trust_level":..,"audit_id":..}
// sig    = ed25519.Sign(domainKey, header + "." + claims)
//
// 安全约束（防 alg=none 攻击）：验签端必须强制要求 alg=EdDSA。
package a2a

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrEmptyDomainKey domain 私钥未配置。
var ErrEmptyDomainKey = errors.New("a2a: empty domain key")

// ErrEmptySubject subject 不能为空。
var ErrEmptySubject = errors.New("a2a: empty subject")

// ErrEmptyAudience audience 不能为空。
var ErrEmptyAudience = errors.New("a2a: empty audience")

// ErrEmptyJTI jti 不能为空（防重放）。
var ErrEmptyJTI = errors.New("a2a: empty jti")

// ErrTokenMalformed token 格式不合法（非 3 段 / 段内非 base64url）。
var ErrTokenMalformed = errors.New("a2a: token malformed")

// ErrTokenExpired token 已过期。
var ErrTokenExpired = errors.New("a2a: token expired")

// ErrIssuerMismatch 颁发者不匹配。
var ErrIssuerMismatch = errors.New("a2a: issuer mismatch")

// ErrAudienceMismatch 接收方不匹配。
var ErrAudienceMismatch = errors.New("a2a: audience mismatch")

// ErrUnsupportedAlg 算法不支持（防 alg=none / HS256 攻击）。
var ErrUnsupportedAlg = errors.New("a2a: unsupported alg")

// ErrSignatureInvalid 签名无效。
var ErrSignatureInvalid = errors.New("a2a: signature invalid")

// ErrClaimInvalid claims 字段缺失或非法。
var ErrClaimInvalid = errors.New("a2a: claim invalid")

// ErrInvalidTrustLevel trust_level 超出 [0, 100]。
var ErrInvalidTrustLevel = errors.New("a2a: trust_level out of range")

// =============================================================================
// JWT Header / Claims
// =============================================================================

// jwtHeader JWT 头部。
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

// jwtClaims A2A JWT claims（v2.0.1 §4.3.3 标准字段）。
type jwtClaims struct {
	Issuer     string `json:"iss"`
	Audience   string `json:"aud"`
	Subject    string `json:"sub"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
	JTI        string `json:"jti"`
	Scope      string `json:"scope,omitempty"`
	TrustLevel int    `json:"trust_level,omitempty"`
	AuditID    string `json:"audit_id,omitempty"`
}

// =============================================================================
// 公开类型：SignInput / TokenClaims
// =============================================================================

// SignInput 颁发 A2A token 的入参。
type SignInput struct {
	// Subject 调用方 agent DID/UUID
	Subject string
	// Audience 目标 agent / 服务标识
	Audience string
	// JTI 唯一 ID（防重放）
	JTI string
	// TTL token 有效期（默认 15min）
	TTL time.Duration
	// Scope 授权范围（空格分隔，如 "read:tags write:logs"）
	Scope string
	// TrustLevel 信任度评分 [0, 100]（由 TrustLevel 计算器输出）
	TrustLevel int
	// AuditID 关联审计日志 ID（可选）
	AuditID string
}

// TokenClaims Verify 输出的 claims 视图。
type TokenClaims struct {
	Issuer     string
	Audience   string
	Subject    string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	JTI        string
	Scope      string
	TrustLevel int
	AuditID    string
	Kid        string
}

// =============================================================================
// Issuer：A2A token 签发器
// =============================================================================

// IssuerConfig 签发器配置。
type IssuerConfig struct {
	// DomainKey 颁发用的 Ed25519 私钥（必填）
	DomainKey ed25519.PrivateKey
	// Issuer 颁发者标识（默认 "agentid-chain"）
	Issuer string
	// KeyID JWKS 中匹配的 kid（可选，便于密钥轮换）
	KeyID string
	// DefaultTTL Sign 入参 TTL<=0 时使用（默认 15min）
	DefaultTTL time.Duration
	// Clock 时间源（测试用，默认 time.Now）
	Clock func() time.Time
}

// Issuer A2A token 签发器。
//
// 线程安全：所有方法可并发调用（无可变状态）。
type Issuer struct {
	cfg       IssuerConfig
	domainPub ed25519.PublicKey
}

// NewIssuer 构造签发器。
func NewIssuer(cfg IssuerConfig) (*Issuer, error) {
	if len(cfg.DomainKey) != ed25519.PrivateKeySize {
		return nil, ErrEmptyDomainKey
	}
	pub, ok := cfg.DomainKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("a2a: cannot extract domain public key")
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "agentid-chain"
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 15 * time.Minute
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Issuer{cfg: cfg, domainPub: pub}, nil
}

// Issuer 返回 issuer 名（便于上游构造）。
func (i *Issuer) Issuer() string { return i.cfg.Issuer }

// KeyID 返回 kid（用于 JWKS 查询）。
func (i *Issuer) KeyID() string { return i.cfg.KeyID }

// PublicKey 返回 domain 公钥（用于 JWKS 暴露）。
func (i *Issuer) PublicKey() ed25519.PublicKey { return i.domainPub }

// Sign 颁发 A2A token。
func (i *Issuer) Sign(in SignInput) (string, error) {
	if in.Subject == "" {
		return "", ErrEmptySubject
	}
	if in.Audience == "" {
		return "", ErrEmptyAudience
	}
	if in.JTI == "" {
		return "", ErrEmptyJTI
	}
	if in.TrustLevel < 0 || in.TrustLevel > 100 {
		return "", fmt.Errorf("%w: got %d", ErrInvalidTrustLevel, in.TrustLevel)
	}
	if in.TTL <= 0 {
		in.TTL = i.cfg.DefaultTTL
	}
	now := i.cfg.Clock()

	header := jwtHeader{Alg: "EdDSA", Typ: "JWT", Kid: i.cfg.KeyID}
	claims := jwtClaims{
		Issuer:     i.cfg.Issuer,
		Audience:   in.Audience,
		Subject:    in.Subject,
		IssuedAt:   now.Unix(),
		ExpiresAt:  now.Add(in.TTL).Unix(),
		JTI:        in.JTI,
		Scope:      in.Scope,
		TrustLevel: in.TrustLevel,
		AuditID:    in.AuditID,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("a2a: marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("a2a: marshal claims: %w", err)
	}

	h64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	c64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := h64 + "." + c64
	sig := ed25519.Sign(i.cfg.DomainKey, []byte(signingInput))
	s64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + s64, nil
}

// =============================================================================
// 辅助：仅做格式解析（不验签）。Verify 在 verify.go 中实现。
// =============================================================================

// ParseUnverified 解析 token 但不验证签名（调试 / 日志用）。
//
// ⚠️ 警告：永远不要把 ParseUnverified 的结果当作可信！
func ParseUnverified(token string) (*TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: got %d parts", ErrTokenMalformed, len(parts))
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: header decode: %v", ErrTokenMalformed, err)
	}
	var h jwtHeader
	if err := json.Unmarshal(headerJSON, &h); err != nil {
		return nil, fmt.Errorf("%w: header parse: %v", ErrTokenMalformed, err)
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: claims decode: %v", ErrTokenMalformed, err)
	}
	var c jwtClaims
	if err := json.Unmarshal(claimsJSON, &c); err != nil {
		return nil, fmt.Errorf("%w: claims parse: %v", ErrTokenMalformed, err)
	}
	return &TokenClaims{
		Issuer:     c.Issuer,
		Audience:   c.Audience,
		Subject:    c.Subject,
		IssuedAt:   time.Unix(c.IssuedAt, 0),
		ExpiresAt:  time.Unix(c.ExpiresAt, 0),
		JTI:        c.JTI,
		Scope:      c.Scope,
		TrustLevel: c.TrustLevel,
		AuditID:    c.AuditID,
		Kid:        h.Kid,
	}, nil
}
