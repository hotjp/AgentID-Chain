// Package aap AAP Proof 验签中间件（net/http 标准实现，兼容 connect-go / gin）。
//
// 设计：
//   - 函数式 Wrap(handler, signer) → handler 风格
//   - 不依赖特定 HTTP 框架（使用 net/http）
//   - 通过 context.WithValue 注入 user_context
//   - 失败立即返回 401（不泄露细节）
//
// 数据流：
//
//	HTTP Request  →  Middleware.Wrap(handler)
//	                  │
//	                  ├─ 读取 X-AAP-Proof header
//	                  ├─ signer.Verify(token)
//	                  ├─ inject UserContext → context
//	                  └─ next.ServeHTTP(w, r)
//
// 安全约束：
//   - 强制 HTTPS（当 config.AAP.RequireHTTPS = true）
//   - 拒绝 alg=none（Verify 内部已校验）
//   - 拒绝过期 token（Verify 内部已校验 exp）
//   - 拒绝错误 issuer（Verify 内部已校验 iss）
package aap

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"strings"
	"time"
)

// =============================================================================
// 错误定义
// =============================================================================

// ErrMissingProof X-AAP-Proof header 缺失。
var ErrMissingProof = errors.New("aap: missing X-AAP-Proof header")

// ErrHTTPSRequired 要求 HTTPS 但当前不是。
var ErrHTTPSRequired = errors.New("aap: HTTPS required")

// =============================================================================
// UserContext 注入到 ctx 的业务信息
// =============================================================================

// UserContext 用户上下文（middleware 注入到 request context）。
//
// 上游 handler 通过 FromContext(ctx) 取出。
type UserContext struct {
	// AgentUUID 通过 proof 颁发给的 Agent UUID
	AgentUUID string
	// AgentPubKey 客户端公钥（32 字节）
	AgentPubKey ed25519.PublicKey
	// Issuer Proof 颁发方
	Issuer string
	// IssuedAt Proof 颁发时间
	IssuedAt time.Time
	// ExpiresAt Proof 过期时间
	ExpiresAt time.Time
	// JTI Proof 唯一 ID
	JTI string
	// Proof 原始 token（供审计 / 日志）
	Proof string
}

// userCtxKey context.Value 用的私有 key（避免和其他包冲突）。
type userCtxKey struct{}

// WithUserContext 注入 UserContext 到 ctx。
//
// 业务侧可调用此函数构造测试 ctx；中间件内部也用。
func WithUserContext(ctx context.Context, u *UserContext) context.Context {
	if u == nil {
		return ctx
	}
	return context.WithValue(ctx, userCtxKey{}, u)
}

// FromContext 从 ctx 取出 UserContext；不存在返回 nil。
func FromContext(ctx context.Context) *UserContext {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(userCtxKey{}).(*UserContext)
	return v
}

// =============================================================================
// Middleware
// =============================================================================

// MiddlewareConfig 中间件配置。
type MiddlewareConfig struct {
	// Signer Proof 验签器
	Signer *ProofSigner
	// HeaderName 自定义 header 名（默认 "X-AAP-Proof"）
	HeaderName string
	// RequireHTTPS 强制 HTTPS（默认 false；生产应开启）
	RequireHTTPS bool
	// Skipper 可选：跳过中间件（白名单路径）
	Skipper func(*http.Request) bool
	// ErrorHandler 自定义错误响应（默认 401 + JSON）
	ErrorHandler func(http.ResponseWriter, *http.Request, error)
}

// Middleware AAP Proof 验签中间件。
type Middleware struct {
	cfg MiddlewareConfig
}

// NewMiddleware 构造中间件。
func NewMiddleware(cfg MiddlewareConfig) (*Middleware, error) {
	if cfg.Signer == nil {
		return nil, errors.New("aap: nil signer")
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-AAP-Proof"
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = defaultErrorHandler
	}
	return &Middleware{cfg: cfg}, nil
}

// Wrap 包装 next；返回新的 http.Handler。
//
// 用法（mux 风格）：
//
//	mw, _ := aap.NewMiddleware(...)
//	handler := mw.Wrap(yourHandler)
//
// 用法（connect-go）：
//
//	interceptor := connect.WithInterceptors(connect.UnaryInterceptorFunc(mw.ConnectUnary))
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Skipper 检查
		if m.cfg.Skipper != nil && m.cfg.Skipper(r) {
			next.ServeHTTP(w, r)
			return
		}

		// 2. HTTPS 强制
		if m.cfg.RequireHTTPS && r.TLS == nil && !isForwardedHTTPS(r) {
			m.cfg.ErrorHandler(w, r, ErrHTTPSRequired)
			return
		}

		// 3. 读 header
		tok := r.Header.Get(m.cfg.HeaderName)
		if tok == "" {
			m.cfg.ErrorHandler(w, r, ErrMissingProof)
			return
		}

		// 4. 验签
		view, err := m.cfg.Signer.Verify(tok)
		if err != nil {
			m.cfg.ErrorHandler(w, r, err)
			return
		}

		// 5. 注入 user_context
		uc := &UserContext{
			AgentUUID:   view.AgentUUID,
			AgentPubKey: view.AgentPubKey,
			Issuer:      view.Issuer,
			IssuedAt:    view.IssuedAt,
			ExpiresAt:   view.ExpiresAt,
			JTI:         view.JTI,
			Proof:       tok,
		}
		ctx := WithUserContext(r.Context(), uc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ConnectUnary connect-go 风格的拦截器适配（Unary）。
//
// 用法：
//   interceptor := connect.WithInterceptors(connect.UnaryInterceptorFunc(mw.ConnectUnary))
//
// ConnectUnary 内部走 Wrap，因此行为一致。
func (m *Middleware) ConnectUnary(next http.Handler) http.Handler {
	return m.Wrap(next)
}

// defaultErrorHandler 默认 401 + 简洁 JSON。
func defaultErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	code := http.StatusUnauthorized
	msg := "unauthorized"
	switch {
	case errors.Is(err, ErrHTTPSRequired):
		code = http.StatusUpgradeRequired
		msg = "https required"
	case errors.Is(err, ErrMissingProof):
		msg = "missing proof"
	case errors.Is(err, ErrResponseExpired):
		msg = "proof expired"
	case errors.Is(err, ErrSignatureInvalid):
		msg = "invalid signature"
	case errors.Is(err, ErrProofIssuerMismatch):
		msg = "issuer mismatch"
	case errors.Is(err, ErrProofUnsupportedAlg):
		msg = "unsupported alg"
	case errors.Is(err, ErrProofMalformed):
		msg = "malformed proof"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + msg + `","code":` + intToStr(code) + `}`))
}

// isForwardedHTTPS 检查反向代理 X-Forwarded-Proto。
func isForwardedHTTPS(r *http.Request) bool {
	proto := r.Header.Get("X-Forwarded-Proto")
	return strings.EqualFold(proto, "https")
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
