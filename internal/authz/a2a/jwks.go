// Package a2a JWKS (JSON Web Key Set) HTTP 端点（v2.0.1 §4.3.3 配套）。
//
// 业务用途：A2A 验签端通过此端点拿到颁发方的公钥，按 kid 匹配后验签 JWT。
//
// 标准：RFC 7517（JSON Web Key）+ RFC 8037（OKP / Ed25519 表示）
//
// 公钥 JWK 字段（kty=OKP, crv=Ed25519）：
//
//	{
//	  "kty": "OKP",
//	  "crv": "Ed25519",
//	  "use": "sig",
//	  "alg": "EdDSA",
//	  "kid": "<key-id>",
//	  "x":   "<base64url(32-byte pub)>"
//	}
//
// 端点：GET /.well-known/jwks.json
// 响应：{"keys": [JWK...]}
//
// 缓存：HTTP Cache-Control: public, max-age=<ttl-seconds>
package a2a

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// =============================================================================
// JWK 数据结构
// =============================================================================

// JWK 单个 JSON Web Key（OKP / Ed25519 子集）。
type JWK struct {
	Kty string `json:"kty"`           // "OKP"
	Crv string `json:"crv"`           // "Ed25519"
	Use string `json:"use,omitempty"` // "sig"
	Alg string `json:"alg,omitempty"` // "EdDSA"
	Kid string `json:"kid"`
	X   string `json:"x"` // base64url(32-byte pubkey)
}

// JWKS JWK 集合（响应体）。
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// =============================================================================
// KeySource：抽象密钥来源
// =============================================================================

// KeyEntry 单个密钥的元数据。
type KeyEntry struct {
	Kid    string
	Public ed25519.PublicKey
}

// KeySource 密钥来源接口（KMS / 静态配置 / 文件等）。
type KeySource interface {
	// Keys 返回当前所有 active 公钥（按 Kid 排序）。
	Keys() ([]KeyEntry, error)
}

// StaticKeySource 静态密钥列表（适合单 issuer / 启动加载）。
type StaticKeySource struct {
	Entries []KeyEntry
}

// Keys 返回 entries 副本，按 Kid 排序（稳定输出）。
func (s *StaticKeySource) Keys() ([]KeyEntry, error) {
	out := make([]KeyEntry, len(s.Entries))
	copy(out, s.Entries)
	sort.Slice(out, func(i, j int) bool { return out[i].Kid < out[j].Kid })
	return out, nil
}

// IssuerKeySource 适配 *Issuer，单 key 暴露。
type IssuerKeySource struct {
	Issuer *Issuer
}

// Keys 从 Issuer 提取单个 kid → pubkey。
func (s *IssuerKeySource) Keys() ([]KeyEntry, error) {
	if s.Issuer == nil {
		return nil, errors.New("a2a: nil issuer")
	}
	return []KeyEntry{{
		Kid:    s.Issuer.KeyID(),
		Public: s.Issuer.PublicKey(),
	}}, nil
}

// =============================================================================
// JWKSHandler
// =============================================================================

// JWKSHandlerConfig handler 配置。
type JWKSHandlerConfig struct {
	// Source 密钥来源（必填）
	Source KeySource
	// CacheTTL HTTP Cache-Control max-age（默认 5min）
	CacheTTL time.Duration
	// RefreshInterval 后台刷新间隔（0 = 不主动刷新，每次请求都查 Source）
	RefreshInterval time.Duration
	// Clock 时间源（测试用）
	Clock func() time.Time
}

// JWKSHandler /.well-known/jwks.json 处理器。
//
// 线程安全：内部 RWMutex 保护缓存。
type JWKSHandler struct {
	cfg JWKSHandlerConfig

	mu        sync.RWMutex
	cached    *JWKS  // 缓存的响应体
	cachedJSON []byte // 缓存的 JSON 编码（避免重复 marshal）
	lastFetch time.Time
}

// NewJWKSHandler 构造 handler。
func NewJWKSHandler(cfg JWKSHandlerConfig) (*JWKSHandler, error) {
	if cfg.Source == nil {
		return nil, errors.New("a2a: nil key source")
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &JWKSHandler{cfg: cfg}, nil
}

// ServeHTTP 实现 http.Handler。
//
// 响应：
//   - 200 application/json {"keys":[...]}
//   - 500 + 错误 JSON（Source.Keys 报错）
func (h *JWKSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := h.snapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(h.cfg.CacheTTL.Seconds())))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Snapshot 暴露当前缓存的 JWKS（便于测试 / 内部调用）。
func (h *JWKSHandler) Snapshot() (*JWKS, error) {
	if _, err := h.snapshot(); err != nil {
		return nil, err
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	// 深拷贝防止外部修改
	out := &JWKS{Keys: make([]JWK, len(h.cached.Keys))}
	copy(out.Keys, h.cached.Keys)
	return out, nil
}

// Refresh 主动刷新缓存（用于密钥轮换或后台 ticker）。
func (h *JWKSHandler) Refresh() error {
	_, err := h.fetch()
	return err
}

// =============================================================================
// 内部
// =============================================================================

// snapshot 返回当前的 JSON 编码（命中缓存 → 直接返回；过期 → fetch 重建）。
func (h *JWKSHandler) snapshot() ([]byte, error) {
	now := h.cfg.Clock()
	h.mu.RLock()
	if h.cached != nil && h.cfg.RefreshInterval > 0 &&
		now.Sub(h.lastFetch) < h.cfg.RefreshInterval {
		data := h.cachedJSON
		h.mu.RUnlock()
		return data, nil
	}
	// 无 RefreshInterval 时也复用缓存（只要 cached 非 nil）
	if h.cached != nil && h.cfg.RefreshInterval == 0 {
		data := h.cachedJSON
		h.mu.RUnlock()
		return data, nil
	}
	h.mu.RUnlock()
	return h.fetch()
}

// fetch 从 Source 加载并重建缓存。
func (h *JWKSHandler) fetch() ([]byte, error) {
	entries, err := h.cfg.Source.Keys()
	if err != nil {
		return nil, fmt.Errorf("a2a: key source: %w", err)
	}
	keys := make([]JWK, 0, len(entries))
	for _, e := range entries {
		if len(e.Public) != ed25519.PublicKeySize {
			continue // skip invalid
		}
		keys = append(keys, JWK{
			Kty: "OKP",
			Crv: "Ed25519",
			Use: "sig",
			Alg: "EdDSA",
			Kid: e.Kid,
			X:   base64.RawURLEncoding.EncodeToString(e.Public),
		})
	}
	jwks := &JWKS{Keys: keys}
	data, err := json.Marshal(jwks)
	if err != nil {
		return nil, fmt.Errorf("a2a: marshal jwks: %w", err)
	}
	h.mu.Lock()
	h.cached = jwks
	h.cachedJSON = data
	h.lastFetch = h.cfg.Clock()
	h.mu.Unlock()
	return data, nil
}

// writeError 简化错误响应。
func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
}

// =============================================================================
// 工具：把 JWKS 转回 KeyResolver（验签端便利）
// =============================================================================

// ResolverFromJWKS 把 JWKS 转成 KeyResolver，便于 Verifier 使用。
func ResolverFromJWKS(jwks *JWKS) (KeyResolver, error) {
	if jwks == nil {
		return nil, errors.New("a2a: nil jwks")
	}
	keys := make(map[string]ed25519.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			continue
		}
		pub, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			continue
		}
		if len(pub) != ed25519.PublicKeySize {
			continue
		}
		keys[k.Kid] = ed25519.PublicKey(pub)
	}
	return &MapKeyResolver{Keys: keys}, nil
}
