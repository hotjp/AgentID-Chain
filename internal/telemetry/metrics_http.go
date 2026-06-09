// Package telemetry: HTTP 监控指标 (P19.3)。
//
// 提供：
//   - http_request_duration_seconds  histogram
//   - http_requests_total            counter
//   - http_requests_in_flight        gauge
//
// 标签：
//   - method, route, status_code
package telemetry

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPMetrics HTTP 指标集。
type HTTPMetrics struct {
	Duration  *prometheus.HistogramVec
	Requests  *prometheus.CounterVec
	InFlight  prometheus.Gauge
	BytesIn   *prometheus.HistogramVec
	BytesOut  *prometheus.HistogramVec
	StartTime time.Time
}

// NewHTTPMetrics 构造 HTTP 指标集。
func NewHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	factory := promauto.With(reg)
	// 默认 buckets：覆盖 1ms ~ 30s
	buckets := []float64{
		0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
	}
	return &HTTPMetrics{
		StartTime: time.Now(),
		Duration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: buckets,
			},
			[]string{"method", "route", "status_code"},
		),
		Requests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"method", "route", "status_code"},
		),
		InFlight: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Current in-flight HTTP requests",
			},
		),
		BytesIn: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "HTTP request body size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 4, 8),
			},
			[]string{"method", "route"},
		),
		BytesOut: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response body size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 4, 8),
			},
			[]string{"method", "route"},
		),
	}
}

// Observe 记录一次 HTTP 请求。
func (m *HTTPMetrics) Observe(method, route string, status int, duration time.Duration, bytesIn, bytesOut int64) {
	statusStr := statusCodeToString(status)
	m.Duration.WithLabelValues(method, route, statusStr).Observe(duration.Seconds())
	m.Requests.WithLabelValues(method, route, statusStr).Inc()
	if bytesIn > 0 {
		m.BytesIn.WithLabelValues(method, route).Observe(float64(bytesIn))
	}
	if bytesOut > 0 {
		m.BytesOut.WithLabelValues(method, route).Observe(float64(bytesOut))
	}
}

// IncInFlight 增加 in-flight 计数。
func (m *HTTPMetrics) IncInFlight() { m.InFlight.Inc() }

// DecInFlight 减少 in-flight 计数。
func (m *HTTPMetrics) DecInFlight() { m.InFlight.Dec() }

// UptimeSeconds 服务运行时长（秒）。
func (m *HTTPMetrics) UptimeSeconds() float64 {
	return time.Since(m.StartTime).Seconds()
}

func statusCodeToString(code int) string {
	switch {
	case code < 200:
		return "1xx"
	case code < 300:
		return "2xx"
	case code < 400:
		return "3xx"
	case code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// =============================================================================
// 全局默认注册表（避免每次都 new）
// =============================================================================

var (
	defaultMetricsOnce sync.Once
	defaultMetrics     *HTTPMetrics
)

// DefaultHTTPMetrics 返回全局默认指标集。
func DefaultHTTPMetrics() *HTTPMetrics {
	defaultMetricsOnce.Do(func() {
		defaultMetrics = NewHTTPMetrics(prometheus.DefaultRegisterer)
	})
	return defaultMetrics
}
