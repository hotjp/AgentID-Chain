// Package middleware: L5 中间件链（P7.2-P7.9）。
//
// 设计：纯 net/http 风格中间件（func(http.Handler) http.Handler）。
// 装配顺序由 server 包在 NewServer 时决定；中间件本包不感知。
//
// 链顺序（与 docs §3.3 决策顺序对齐）：
//
//	1. Recover           panic 兜底
//	2. RequestID         注入 X-Request-ID
//	3. Logging           slog 结构化
//	4. Metrics           prometheus 自动
//	5. CORS              跨域
//	6. UA-Block          拦截恶意 UA
//	7. APIKey            静态 API Key 认证
//	8. AAP               AAP 协议握手
package middleware

import "net/http"

// HandlerFunc 标准中间件签名。
type HandlerFunc func(http.Handler) http.Handler

// Chain 中间件链。
type Chain struct {
	mws []HandlerFunc
}

// NewChain 构造空链。
func NewChain(mws ...HandlerFunc) *Chain {
	return &Chain{mws: mws}
}

// Append 追加中间件（链式风格）。
func (c *Chain) Append(mw HandlerFunc) *Chain {
	c.mws = append(c.mws, mw)
	return c
}

// Then 包装终端 handler。
func (c *Chain) Then(h http.Handler) http.Handler {
	// 反向 wrap：先 append 的中间件在最外层
	for i := len(c.mws) - 1; i >= 0; i-- {
		h = c.mws[i](h)
	}
	return h
}

// Wrap 便捷方法：Then(terminal)。
func (c *Chain) Wrap(h http.Handler) http.Handler { return c.Then(h) }

// Len 返回链长度（测试用）。
func (c *Chain) Len() int { return len(c.mws) }
