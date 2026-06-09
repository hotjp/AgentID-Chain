// Package middleware: Metrics 中间件（P7.4）。
//
// 记录请求延迟直方图 + 状态码计数（Prometheus）。
// 直方图默认 buckets；指标名固定 agentid_http_request_*。
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// httpRequestDuration 请求延迟直方图（按 method+path+status 分桶）。
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentid_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)
	// httpRequestTotal 请求计数。
	httpRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentid_http_request_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)

// Metrics 返回 prometheus 指标中间件。
func Metrics() HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			latency := time.Since(start).Seconds()
			path := r.URL.Path
			status := strconv.Itoa(rec.status)
			httpRequestDuration.WithLabelValues(r.Method, path, status).Observe(latency)
			httpRequestTotal.WithLabelValues(r.Method, path, status).Inc()
		})
	}
}
