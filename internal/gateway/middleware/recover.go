// Package middleware: Recover 中间件（P7.2）。
//
// 捕获下游 handler 的 panic，写 500 + 记录日志。
//
// 注意：捕获 panic 后必须调用 http.Error 写响应并 abort，避免写入已
// 关闭的连接。
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover 返回 panic-recover 中间件。
//
// 用法：middleware.Recover(logger)(next)
func Recover(logger *slog.Logger) HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic in handler",
						slog.Any("err", rec),
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
						slog.String("stack", string(debug.Stack())),
					)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
