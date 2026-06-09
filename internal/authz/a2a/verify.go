// Package a2a A2A Token 验签器（verify.go）。
//
// 验签流程（v2.0.1 §4.3.3）：
//  1. 拆 3 段（header.claims.sig）
//  2. 解析 header → 强制 alg=EdDSA, typ=JWT（防 alg=none）
//  3. 解析 claims → 校验 iss / aud / exp / sub / jti
//  4. 通过 KeyResolver 根据 kid 解析公钥（支持 JWKS 轮换）
//  5. ed25519.Verify(pub, signingInput, sig)
//  6. 返回 TokenClaims
//
// 与 Issuer 的关系：Issuer 颁发 token，Verifier 解析并验证。
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
// KeyResolver：根据 kid 解析公钥
// =============================================================================

// KeyResolver 公钥解析器接口（JWKS 抽象）。
//
// 实现方负责根据 kid 返回对应的 Ed25519 公钥；找不到时返回 ErrKeyNotFound。
type KeyResolver interface {
	// Resolve 根据 kid 返回公钥；kid 可以为空（fallback 默认 key）。
	Resolve(kid string) (ed25519.PublicKey, error)
}

// ErrKeyNotFound kid 对应的密钥不存在。
var ErrKeyNotFound = errors.New("a2a: key not found")

// StaticKeyResolver 单一公钥解析器（适合单 issuer / 无轮换场景）。
type StaticKeyResolver struct {
	PublicKey ed25519.PublicKey
}

// Resolve 不区分 kid，恒返回 PublicKey。
func (r *StaticKeyResolver) Resolve(_ string) (ed25519.PublicKey, error) {
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return nil, ErrKeyNotFound
	}
	return r.PublicKey, nil
}

// MapKeyResolver 按 kid → pubkey 映射解析（适合 JWKS 静态版本）。
type MapKeyResolver struct {
	// Keys kid → pubkey
	Keys map[string]ed25519.PublicKey
	// Default kid 为空或不在 Keys 时使用（可选）
	Default ed25519.PublicKey
}

// Resolve 根据 kid 查找公钥；找不到时返回 Default 或 ErrKeyNotFound。
func (r *MapKeyResolver) Resolve(kid string) (ed25519.PublicKey, error) {
	if k, ok := r.Keys[kid]; ok && len(k) == ed25519.PublicKeySize {
		return k, nil
	}
	if len(r.Default) == ed25519.PublicKeySize {
		return r.Default, nil
	}
	return nil, fmt.Errorf("%w: kid=%q", ErrKeyNotFound, kid)
}

// =============================================================================
// VerifierConfig / Verifier
// =============================================================================

// VerifierConfig 验签器配置。
type VerifierConfig struct {
	// Resolver 公钥解析器（必填）
	Resolver KeyResolver
	// ExpectedIssuer 期望的颁发者（必填，避免跨发行方）
	ExpectedIssuer string
	// ExpectedAudience 期望的接收方（必填）
	ExpectedAudience string
	// Clock 时间源（默认 time.Now）
	Clock func() time.Time
	// LeewaySeconds 时钟漂移容忍（单位秒，默认 5）
	LeewaySeconds int64
}

// Verifier A2A Token 验签器。
type Verifier struct {
	cfg VerifierConfig
}

// NewVerifier 构造验签器。
func NewVerifier(cfg VerifierConfig) (*Verifier, error) {
	if cfg.Resolver == nil {
		return nil, errors.New("a2a: nil resolver")
	}
	if cfg.ExpectedIssuer == "" {
		return nil, fmt.Errorf("%w: missing expected issuer", ErrClaimInvalid)
	}
	if cfg.ExpectedAudience == "" {
		return nil, fmt.Errorf("%w: missing expected audience", ErrClaimInvalid)
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.LeewaySeconds <= 0 {
		cfg.LeewaySeconds = 5
	}
	return &Verifier{cfg: cfg}, nil
}

// =============================================================================
// Verify
// =============================================================================

// Verify 验证 A2A token，返回解析后的 claims。
//
// 错误：
//   - ErrTokenMalformed：格式不合法
//   - ErrUnsupportedAlg：alg ≠ EdDSA
//   - ErrIssuerMismatch / ErrAudienceMismatch：claims 校验失败
//   - ErrTokenExpired：超过 exp + leeway
//   - ErrKeyNotFound：kid 对应公钥找不到
//   - ErrSignatureInvalid：签名错误
//   - ErrClaimInvalid：必填字段缺失
func (v *Verifier) Verify(token string) (*TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: got %d parts", ErrTokenMalformed, len(parts))
	}

	// 1. header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: header decode: %v", ErrTokenMalformed, err)
	}
	var h jwtHeader
	if err := json.Unmarshal(headerJSON, &h); err != nil {
		return nil, fmt.Errorf("%w: header parse: %v", ErrTokenMalformed, err)
	}
	if h.Alg != "EdDSA" {
		return nil, fmt.Errorf("%w: alg=%q", ErrUnsupportedAlg, h.Alg)
	}
	if h.Typ != "JWT" {
		return nil, fmt.Errorf("%w: typ=%q", ErrTokenMalformed, h.Typ)
	}

	// 2. claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: claims decode: %v", ErrTokenMalformed, err)
	}
	var c jwtClaims
	if err := json.Unmarshal(claimsJSON, &c); err != nil {
		return nil, fmt.Errorf("%w: claims parse: %v", ErrTokenMalformed, err)
	}

	// 必填校验
	if c.Subject == "" {
		return nil, fmt.Errorf("%w: empty sub", ErrClaimInvalid)
	}
	if c.JTI == "" {
		return nil, fmt.Errorf("%w: empty jti", ErrClaimInvalid)
	}
	if c.Issuer != v.cfg.ExpectedIssuer {
		return nil, fmt.Errorf("%w: got=%q want=%q",
			ErrIssuerMismatch, c.Issuer, v.cfg.ExpectedIssuer)
	}
	if c.Audience != v.cfg.ExpectedAudience {
		return nil, fmt.Errorf("%w: got=%q want=%q",
			ErrAudienceMismatch, c.Audience, v.cfg.ExpectedAudience)
	}
	now := v.cfg.Clock().Unix()
	if c.ExpiresAt+v.cfg.LeewaySeconds <= now {
		return nil, fmt.Errorf("%w: exp=%d now=%d", ErrTokenExpired, c.ExpiresAt, now)
	}
	if c.TrustLevel < 0 || c.TrustLevel > 100 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidTrustLevel, c.TrustLevel)
	}

	// 3. resolve key + verify sig
	pub, err := v.cfg.Resolver.Resolve(h.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: sig decode: %v", ErrTokenMalformed, err)
	}
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return nil, ErrSignatureInvalid
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

// HasScope 工具方法：检查 claims 中是否包含某个 scope（空格分隔）。
func (tc *TokenClaims) HasScope(scope string) bool {
	if tc == nil || tc.Scope == "" || scope == "" {
		return false
	}
	for _, s := range strings.Split(tc.Scope, " ") {
		if s == scope {
			return true
		}
	}
	return false
}
