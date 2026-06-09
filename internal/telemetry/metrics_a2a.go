// Package telemetry: A2A 监控指标 (P19.5)。
//
// 指标：
//   - a2a_token_issued_total{op, counterparty}    counter
//   - a2a_token_revoked_total{reason}             counter
//   - a2a_token_active                            gauge
//   - a2a_negotiate_duration_seconds              histogram
//   - a2a_session_duration_seconds                histogram
package telemetry

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// A2AMetrics A2A 指标集。
type A2AMetrics struct {
	TokenIssued       *prometheus.CounterVec
	TokenRevoked      *prometheus.CounterVec
	TokenActive       prometheus.Gauge
	TokenVerifyTotal  *prometheus.CounterVec
	NegotiateDuration prometheus.Histogram
	SessionDuration   prometheus.Histogram
}

// NewA2AMetrics 构造 A2A 指标集。
func NewA2AMetrics(reg prometheus.Registerer) *A2AMetrics {
	factory := promauto.With(reg)
	return &A2AMetrics{
		TokenIssued: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "a2a_token_issued_total",
				Help: "Total A2A tokens issued",
			},
			[]string{"operation", "counterparty"},
		),
		TokenRevoked: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "a2a_token_revoked_total",
				Help: "Total A2A tokens revoked",
			},
			[]string{"reason"},
		),
		TokenActive: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "a2a_token_active",
				Help: "Current active A2A tokens",
			},
		),
		TokenVerifyTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "a2a_token_verify_total",
				Help: "Total A2A token verifications",
			},
			[]string{"result"}, // success|failure
		),
		NegotiateDuration: factory.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "a2a_negotiate_duration_seconds",
				Help:    "A2A negotiate duration",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			},
		),
		SessionDuration: factory.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "a2a_session_duration_seconds",
				Help:    "A2A session duration from create to close",
				Buckets: []float64{1, 10, 60, 300, 900, 1800, 3600, 7200, 86400},
			},
		),
	}
}

// ObserveIssue 记录 token 颁发。
func (m *A2AMetrics) ObserveIssue(op, counterparty string) {
	m.TokenIssued.WithLabelValues(op, counterparty).Inc()
	m.TokenActive.Inc()
}

// ObserveRevoke 记录 token 撤销。
func (m *A2AMetrics) ObserveRevoke(reason string) {
	m.TokenRevoked.WithLabelValues(reason).Inc()
	m.TokenActive.Dec()
}

// ObserveVerify 记录 token 验证。
func (m *A2AMetrics) ObserveVerify(result string) {
	m.TokenVerifyTotal.WithLabelValues(result).Inc()
}

// ObserveNegotiate 记录 negotiate 延迟。
func (m *A2AMetrics) ObserveNegotiate(d time.Duration) {
	m.NegotiateDuration.Observe(d.Seconds())
}

// ObserveSession 记录 session 生命周期。
func (m *A2AMetrics) ObserveSession(d time.Duration) {
	m.SessionDuration.Observe(d.Seconds())
}

// 全局默认
var (
	defaultA2AMetricsOnce sync.Once
	defaultA2AMetrics     *A2AMetrics
)

// DefaultA2AMetrics 返回全局默认指标集。
func DefaultA2AMetrics() *A2AMetrics {
	defaultA2AMetricsOnce.Do(func() {
		defaultA2AMetrics = NewA2AMetrics(prometheus.DefaultRegisterer)
	})
	return defaultA2AMetrics
}
