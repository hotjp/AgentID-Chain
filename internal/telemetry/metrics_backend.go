// Package telemetry: 后端延迟监控 (P19.6)。
//
// 指标：
//   - backend_request_duration_seconds{type, op, status} histogram
//   - backend_requests_total{type, op, status}          counter
//   - backend_errors_total{type, op, reason}            counter
//
// type: postgres | redis | chain | mock
// op:   具体操作名
// status: success|failure
package telemetry

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BackendMetrics 后端指标集。
type BackendMetrics struct {
	RequestDuration *prometheus.HistogramVec
	Requests        *prometheus.CounterVec
	Errors          *prometheus.CounterVec
	PoolSize        *prometheus.GaugeVec
	PoolWait        *prometheus.HistogramVec
}

// NewBackendMetrics 构造后端指标集。
func NewBackendMetrics(reg prometheus.Registerer) *BackendMetrics {
	factory := promauto.With(reg)
	return &BackendMetrics{
		RequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "backend_request_duration_seconds",
				Help: "Backend operation duration in seconds",
				// 1ms - 30s
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			},
			[]string{"type", "op", "status"},
		),
		Requests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "backend_requests_total",
				Help: "Total backend requests",
			},
			[]string{"type", "op", "status"},
		),
		Errors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "backend_errors_total",
				Help: "Total backend errors",
			},
			[]string{"type", "op", "reason"},
		),
		PoolSize: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backend_pool_size",
				Help: "Backend connection pool size",
			},
			[]string{"type", "state"}, // state: total|idle|active
		),
		PoolWait: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "backend_pool_wait_seconds",
				Help:    "Time to acquire backend connection",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			},
			[]string{"type"},
		),
	}
}

// Observe 记录一次后端操作。
func (m *BackendMetrics) Observe(btype, op string, d time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "failure"
	}
	m.RequestDuration.WithLabelValues(btype, op, status).Observe(d.Seconds())
	m.Requests.WithLabelValues(btype, op, status).Inc()
}

// ObserveError 记录错误（带 reason）。
func (m *BackendMetrics) ObserveError(btype, op, reason string) {
	m.Errors.WithLabelValues(btype, op, reason).Inc()
}

// SetPoolSize 设置池状态。
func (m *BackendMetrics) SetPoolSize(btype, state string, size float64) {
	m.PoolSize.WithLabelValues(btype, state).Set(size)
}

// ObservePoolWait 记录池等待时间。
func (m *BackendMetrics) ObservePoolWait(btype string, d time.Duration) {
	m.PoolWait.WithLabelValues(btype).Observe(d.Seconds())
}

// 全局默认
var (
	defaultBackendMetricsOnce sync.Once
	defaultBackendMetrics     *BackendMetrics
)

// DefaultBackendMetrics 返回全局默认指标集。
func DefaultBackendMetrics() *BackendMetrics {
	defaultBackendMetricsOnce.Do(func() {
		defaultBackendMetrics = NewBackendMetrics(prometheus.DefaultRegisterer)
	})
	return defaultBackendMetrics
}

// P50BackendDuration P50 延迟（便于代码内查询）
//
// 实际值应通过 PromQL：histogram_quantile(0.5, ...)
func P50BackendDuration() float64 { return 0 }

// P95BackendDuration P95 延迟。
func P95BackendDuration() float64 { return 0 }

// P99BackendDuration P99 延迟。
func P99BackendDuration() float64 { return 0 }
