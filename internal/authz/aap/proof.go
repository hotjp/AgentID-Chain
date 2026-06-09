// Package aap AAP Proof JWT 格式（EdDSA 签名 + 标准 JWT 序列化）。
//
// 与 verify.go 的关系：
//   - verify.go 的 IssueProof 输出 *Proof（内存结构）
//   - proof.go 提供 Sign/Verify 把 Proof 编/解码为 JWT 字符串（线上传输格式）
//
// JWT 结构（RFC 7519）：
//   <base64url(header)>.<base64url(claims)>.<base64url(sig)>
//
// header  = {"alg":"EdDSA","typ":"JWT"}
// claims  = {"iss":"agentid-chain","sub":"<agent_uuid>","iat":...,"exp":...,"pubkey":"<base64>"}
// sig     = ed25519.Sign(domainKey, header + "." + claims)
//
// 安全约束：
//   - 验证时必须校验 exp（不允许过期 token）
//   - 验证时必须校验 iss（不允许跨签发者）
//   - 验证时必须校验 alg（防止 alg=none 攻击）
package aap

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

// ErrProofMalformed Proof 字符串格式不合法（非 3 段 / 段内非 base64url）。
var ErrProofMalformed = errors.New("aap: proof malformed")

// ErrProofIssuerMismatch 颁发者不匹配。
var ErrProofIssuerMismatch = errors.New("aap: proof issuer mismatch")

// ErrProofUnsupportedAlg 不支持的 alg（防 alg=none 攻击）。
var ErrProofUnsupportedAlg = errors.New("aap: unsupported alg")

// ErrProofClaimInvalid claims 字段缺失或非法。
var ErrProofClaimInvalid = errors.New("aap: proof claim invalid")

// =============================================================================
// JWT Header / Claims
// =============================================================================

// proofHeader JWT header.
type proofHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// proofClaims JWT claims（v2.0.1 最小子集）。
type proofClaims struct {
	// Issuer 颁发者（默认 "agentid-chain"）
	Issuer string `json:"iss"`
	// Subject 颁发对象 = agent_uuid
	Subject string `json:"sub"`
	// IssuedAt 颁发时间（Unix 秒）
	IssuedAt int64 `json:"iat"`
	// ExpiresAt 过期时间（Unix 秒）
	ExpiresAt int64 `json:"exp"`
	// PublicKey 客户端公钥（base64url 编码的 32 字节 Ed25519）
	PublicKey string `json:"pubkey"`
	// JTI 唯一 ID（防重放）
	JTI string `json:"jti,omitempty"`
}

// =============================================================================
// Proof JWT 编码器
// =============================================================================

// ProofSigner Proof 签发器。
type ProofSigner struct {
	domainKey ed25519.PrivateKey
	domainPub ed25519.PublicKey
	issuer    string
	clock     func() time.Time
}

// NewProofSigner 构造签发器。
func NewProofSigner(domainKey ed25519.PrivateKey, issuer string) (*ProofSigner, error) {
	if len(domainKey) != ed25519.PrivateKeySize {
		return nil, ErrEmptyDomain
	}
	pub, ok := domainKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("aap: cannot extract domain public key")
	}
	if issuer == "" {
		issuer = "agentid-chain"
	}
	return &ProofSigner{
		domainKey: domainKey,
		domainPub: pub,
		issuer:    issuer,
		clock:     time.Now,
	}, nil
}

// SetClock 注入时间源（测试用）。
func (s *ProofSigner) SetClock(c func() time.Time) { s.clock = c }

// Issuer 返回颁发者名。
func (s *ProofSigner) Issuer() string { return s.issuer }

// Sign 给定入参签发 JWT。
func (s *ProofSigner) Sign(in SignInput) (string, error) {
	if in.AgentUUID == "" {
		return "", ErrEmptyAgentUUID
	}
	if err := validateUUIDString(in.AgentUUID); err != nil {
		return "", fmt.Errorf("%w: %s", ErrEmptyAgentUUID, in.AgentUUID)
	}
	if in.JTI == "" {
		return "", ErrProofClaimInvalid
	}
	if in.TTL <= 0 {
		in.TTL = 5 * time.Minute
	}
	now := s.clock()

	header := proofHeader{Alg: "EdDSA", Typ: "JWT"}
	claims := proofClaims{
		Issuer:    s.issuer,
		Subject:   in.AgentUUID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(in.TTL).Unix(),
		PublicKey: base64.RawURLEncoding.EncodeToString(in.AgentPubKey),
		JTI:       in.JTI,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("aap: marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("aap: marshal claims: %w", err)
	}

	h64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	c64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := h64 + "." + c64
	sig := ed25519.Sign(s.domainKey, []byte(signingInput))
	s64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + s64, nil
}

// Verify 验签 JWT 字符串。
//
// 步骤：
//  1. 拆 3 段
//  2. base64url 解码 header → 校验 alg
//  3. base64url 解码 claims → 校验 iss / exp
//  4. ed25519.Verify(domainPub, signingInput, sig)
func (s *ProofSigner) Verify(token string) (*ProofClaimsView, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: got %d parts", ErrProofMalformed, len(parts))
	}
	// 1. header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: header decode: %v", ErrProofMalformed, err)
	}
	var h proofHeader
	if err := json.Unmarshal(headerJSON, &h); err != nil {
		return nil, fmt.Errorf("%w: header parse: %v", ErrProofMalformed, err)
	}
	if h.Alg != "EdDSA" {
		return nil, fmt.Errorf("%w: alg=%s", ErrProofUnsupportedAlg, h.Alg)
	}
	if h.Typ != "JWT" {
		return nil, fmt.Errorf("%w: typ=%s", ErrProofMalformed, h.Typ)
	}

	// 2. claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: claims decode: %v", ErrProofMalformed, err)
	}
	var c proofClaims
	if err := json.Unmarshal(claimsJSON, &c); err != nil {
		return nil, fmt.Errorf("%w: claims parse: %v", ErrProofMalformed, err)
	}
	if c.Issuer != s.issuer {
		return nil, fmt.Errorf("%w: got=%s want=%s", ErrProofIssuerMismatch, c.Issuer, s.issuer)
	}
	now := s.clock()
	if c.ExpiresAt <= now.Unix() {
		return nil, fmt.Errorf("%w: expired at %d", ErrResponseExpired, c.ExpiresAt)
	}
	if c.Subject == "" {
		return nil, fmt.Errorf("%w: empty sub", ErrProofClaimInvalid)
	}

	// 3. signature
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: sig decode: %v", ErrProofMalformed, err)
	}
	if !ed25519.Verify(s.domainPub, []byte(parts[0]+"."+parts[1]), sig) {
		return nil, ErrSignatureInvalid
	}

	// 解码 pubkey
	pub, err := base64.RawURLEncoding.DecodeString(c.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("%w: pubkey decode: %v", ErrProofMalformed, err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: pubkey length %d", ErrProofClaimInvalid, len(pub))
	}

	return &ProofClaimsView{
		Issuer:     c.Issuer,
		AgentUUID:  c.Subject,
		IssuedAt:   time.Unix(c.IssuedAt, 0),
		ExpiresAt:  time.Unix(c.ExpiresAt, 0),
		AgentPubKey: ed25519.PublicKey(pub),
		JTI:        c.JTI,
	}, nil
}

// SignInput Sign 入参。
type SignInput struct {
	AgentUUID   string
	AgentPubKey ed25519.PublicKey
	JTI         string
	TTL         time.Duration
}

// ProofClaimsView Verify 出参（claims 视图）。
type ProofClaimsView struct {
	Issuer      string
	AgentUUID   string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	AgentPubKey ed25519.PublicKey
	JTI         string
}

// =============================================================================
// 工具函数
// =============================================================================

// validateUUIDString 校验 UUID 字符串（避免 import 循环直接 import domain）。
func validateUUIDString(s string) error {
	// 简化：检查长度 32/36 + 全 hex
	if len(s) != 32 && len(s) != 36 {
		return fmt.Errorf("invalid uuid length %d", len(s))
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
			return fmt.Errorf("invalid uuid char %q", c)
		}
	}
	return nil
}
