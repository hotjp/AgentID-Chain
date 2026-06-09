// Package middleware: RequestID 中间件（P7.3）。
//
// 注入 X-Request-ID：优先读取请求头，缺失则生成 UUIDv4。
// 注入到 context + 响应头，便于全链路追踪。
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const (
	// HeaderRequestID 标准请求 ID 头。
	HeaderRequestID = "X-Request-ID"
	// ContextKeyRequestID ctx 中 RequestID 的 key。
	ContextKeyRequestID ctxKey = "request_id"
)

type ctxKey string

// RequestID 返回注入 RequestID 的中间件。
func RequestID() HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if rid == "" {
				rid = uuid.NewString()
			}
			w.Header().Set(HeaderRequestID, rid)
			ctx := context.WithValue(r.Context(), ContextKeyRequestID, rid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFromContext 从 ctx 取 RequestID。
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return v
	}
	return ""
}
