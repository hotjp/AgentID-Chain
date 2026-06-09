// Package handler: A2A（Agent-to-Agent）协议 HTTP 端点（P11.1-P11.5）。
//
// 端点（与 v2.0.1 §4.3.3 对齐）：
//
//	POST /a2a/negotiate              — 协商 scope / trust_level，颁发 A2A Token
//	POST /a2a/verify                 — 校验传入的 A2A Token
//	POST /a2a/revoke                 — 撤销 A2A Token（jti 加入吊销列表）
//	POST /a2a/list                   — 列出某 agent 的活跃 A2A Token
//	GET  /.well-known/jwks.json      — JWKS 端点（公开）
//
// 所有端点都委托 internal/authz/a2a 包；本文件不写业务逻辑，只做参数解析。
package handler

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/authz/a2a"
	"github.com/google/uuid"
)

// A2AHandler A2A HTTP 处理器聚合。
type A2AHandler struct {
	// 颁发方：用于 negotiate
	Issuer *a2a.Issuer
	// 验签方：用于 verify
	Verifier *a2a.Verifier
	// 吊销列表：用于 revoke / verify 黑名单
	Revoker *a2a.Revoker
	// 默认 TTL
	DefaultTTL time.Duration
	// JWKS 缓存 TTL
	JWKSCacheTTL time.Duration
	// PublicKey 颁发方公钥（用于 JWKS）
	PublicKey ed25519.PublicKey
	// KeyID 公钥 ID
	KeyID string
}

// =============================================================================
// POST /a2a/negotiate
// =============================================================================

// NegotiateRequest 协商请求。
type NegotiateRequest struct {
	// Subject 发起方（agent UUID）
	Subject string `json:"subject"`
	// Audience 目标方（agent UUID 或服务名）
	Audience string `json:"audience"`
	// Scope 请求的作用域（空格分隔字符串）
	Scope string `json:"scope,omitempty"`
	// TrustLevel 期望的信任等级（0-100；默认 50）
	TrustLevel int `json:"trust_level,omitempty"`
	// AuditID 关联的审计 ID（可选）
	AuditID string `json:"audit_id,omitempty"`
	// TTLSec 自定义 TTL（秒；0 = 用 DefaultTTL）
	TTLSec int `json:"ttl_seconds,omitempty"`
}

// NegotiateResponse 协商响应。
type NegotiateResponse struct {
	// Token 颁发的 A2A Token
	Token string `json:"token"`
	// ExpiresAt token 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// JTI token 唯一 ID
	JTI string `json:"jti"`
	// TrustLevel 实际颁发的 trust_level
	TrustLevel int `json:"trust_level"`
	// Scope 实际颁发的 scope
	Scope string `json:"scope"`
}

// Negotiate 协商 handler。
func (h *A2AHandler) Negotiate(w http.ResponseWriter, r *http.Request) {
	if h.Issuer == nil {
		http.Error(w, "issuer unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req NegotiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Subject == "" || req.Audience == "" {
		http.Error(w, "subject and audience are required", http.StatusBadRequest)
		return
	}
	if req.TrustLevel == 0 {
		req.TrustLevel = 50
	}
	if req.TrustLevel < 0 || req.TrustLevel > 100 {
		http.Error(w, "trust_level must be in [0, 100]", http.StatusBadRequest)
		return
	}
	ttl := h.DefaultTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if req.TTLSec > 0 {
		ttl = time.Duration(req.TTLSec) * time.Second
	}
	jti := uuid.NewString()
	tok, err := h.Issuer.Sign(a2a.SignInput{
		Subject:    req.Subject,
		Audience:   req.Audience,
		Scope:      req.Scope,
		TrustLevel: req.TrustLevel,
		AuditID:    req.AuditID,
		JTI:        jti,
		TTL:        ttl,
	})
	if err != nil {
		http.Error(w, "sign: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// 跟踪 token 便于后续 revoke
	if h.Revoker != nil {
		_ = h.Revoker.Track(req.Subject, jti, time.Now().Add(ttl))
	}
	resp := NegotiateResponse{
		Token:      tok,
		ExpiresAt:  time.Now().Add(ttl),
		JTI:        jti,
		TrustLevel: req.TrustLevel,
		Scope:      req.Scope,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// =============================================================================
// POST /a2a/verify
// =============================================================================

// VerifyRequest 校验请求。
type VerifyRequest struct {
	// Token 待校验的 A2A Token
	Token string `json:"token"`
}

// VerifyResponse 校验响应。
type VerifyResponse struct {
	// OK 是否通过
	OK bool `json:"ok"`
	// Claims 解析出的 claims（OK=true 时）
	Claims *a2a.TokenClaims `json:"claims,omitempty"`
	// Error 错误信息（OK=false 时）
	Error string `json:"error,omitempty"`
}

// Verify 校验 handler。
func (h *A2AHandler) Verify(w http.ResponseWriter, r *http.Request) {
	if h.Verifier == nil {
		http.Error(w, "verifier unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}
	claims, err := h.Verifier.Verify(req.Token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(VerifyResponse{OK: false, Error: err.Error()})
		return
	}
	if h.Revoker != nil && h.Revoker.IsRevoked(context.Background(), claims.JTI) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(VerifyResponse{OK: false, Error: "token revoked"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(VerifyResponse{OK: true, Claims: claims})
}

// =============================================================================
// POST /a2a/revoke
// =============================================================================

// RevokeRequest 撤销请求。
type RevokeRequest struct {
	// JTI token 唯一 ID
	JTI string `json:"jti"`
	// Reason 撤销原因
	Reason string `json:"reason,omitempty"`
}

// RevokeResponse 撤销响应。
type RevokeResponse struct {
	OK     bool   `json:"ok"`
	JTI    string `json:"jti"`
	Reason string `json:"reason,omitempty"`
}

// Revoke 撤销 handler。
func (h *A2AHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if h.Revoker == nil {
		http.Error(w, "revoker unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.JTI == "" {
		http.Error(w, "jti is required", http.StatusBadRequest)
		return
	}
	if err := h.Revoker.Revoke(r.Context(), req.JTI); err != nil {
		http.Error(w, "revoke: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(RevokeResponse{OK: true, JTI: req.JTI, Reason: req.Reason})
}

// =============================================================================
// POST /a2a/list
// =============================================================================

// ListRequest 列表请求。
type ListRequest struct {
	// Subject 列出该 subject 的活跃 token（jti 列表）
	Subject string `json:"subject"`
}

// ListResponse 列表响应。
type ListResponse struct {
	Subject string   `json:"subject"`
	JTIs    []string `json:"jtis"`
	Count   int      `json:"count"`
}

// List 列出活跃 token（jti 列表）。
func (h *A2AHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.Revoker == nil {
		http.Error(w, "revoker unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Subject == "" {
		http.Error(w, "subject is required", http.StatusBadRequest)
		return
	}
	jtis := h.Revoker.ActiveJTIs(req.Subject)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ListResponse{Subject: req.Subject, JTIs: jtis, Count: len(jtis)})
}

// =============================================================================
// GET /.well-known/jwks.json
// =============================================================================

// JWKSHandler JWKS 端点（公开）。
func (h *A2AHandler) JWKSHandler(w http.ResponseWriter, r *http.Request) {
	if h.PublicKey == nil {
		http.Error(w, "public key unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jwk := JWKFromEd25519(h.KeyID, h.PublicKey)
	ttl := h.JWKSCacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	w.Header().Set("Content-Type", "application/jwk-set+json")
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(ttl.Seconds())))
	_ = json.NewEncoder(w).Encode(a2a.JWKS{Keys: []a2a.JWK{jwk}})
}

// =============================================================================
// 错误哨兵
// =============================================================================

// ErrA2ANilService nil service 哨兵（供测试断言）。
var ErrA2ANilService = errors.New("a2a: nil service")

// JWKFromEd25519 构造 OKP JWK。
func JWKFromEd25519(kid string, pub ed25519.PublicKey) a2a.JWK {
	return a2a.JWK{
		Kty: "OKP",
		Crv: "Ed25519",
		Use: "sig",
		Alg: "EdDSA",
		Kid: kid,
		X:   base64.RawURLEncoding.EncodeToString(pub),
	}
}
