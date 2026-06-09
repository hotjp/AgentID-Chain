// Package middleware: Logging 中间件（P7.5）。
//
// 记录每次请求的 method/path/status/latency/request_id。
// 跳过 /healthz、/live、/readyz、/metrics、/debug/pprof 探活路径，避免日志风暴。
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// skipPaths 不记录访问日志的路径。
var skipPaths = map[string]bool{
	"/live":        true,
	"/readyz":      true,
	"/ready":       true,
	"/healthz":     true,
	"/metrics":     true,
	"/favicon.ico": true,
}

// responseRecorder 包装 ResponseWriter 以捕获 status code。
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Logging 返回结构化日志中间件。
func Logging(logger *slog.Logger) HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)
			latency := time.Since(start)
			logger.Info("http request",
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("latency", latency),
				slog.String("remote", r.RemoteAddr),
			)
		})
	}
}
