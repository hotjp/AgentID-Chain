// Package telemetry: AAP 监控指标 (P19.4)。
//
// 指标：
//   - aap_challenge_total{version, domain, result}  counter
//   - aap_verify_total{version, result}              counter
//   - aap_challenge_duration_seconds                 histogram
//   - aap_active_sessions                            gauge
//   - aap_success_ratio                              自计算（rate 派生）
package telemetry

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// AAPMetrics AAP 指标集。
type AAPMetrics struct {
	ChallengeTotal   *prometheus.CounterVec
	VerifyTotal      *prometheus.CounterVec
	ChallengeLatency prometheus.Histogram
	VerifyLatency    prometheus.Histogram
	ActiveSessions   prometheus.Gauge
	NonceReplays     prometheus.Counter
}

// NewAAPMetrics 构造 AAP 指标集。
func NewAAPMetrics(reg prometheus.Registerer) *AAPMetrics {
	factory := promauto.With(reg)
	return &AAPMetrics{
		ChallengeTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aap_challenge_total",
				Help: "Total AAP challenges issued",
			},
			[]string{"version", "domain", "result"}, // result: success|failure
		),
		VerifyTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "aap_verify_total",
				Help: "Total AAP verify attempts",
			},
			[]string{"version", "result", "reason"},
		),
		ChallengeLatency: factory.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "aap_challenge_duration_seconds",
				Help:    "AAP challenge generation duration",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
			},
		),
		VerifyLatency: factory.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "aap_verify_duration_seconds",
				Help:    "AAP verify duration",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
			},
		),
		ActiveSessions: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "aap_active_sessions",
				Help: "Current active AAP sessions",
			},
		),
		NonceReplays: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "aap_nonce_replays_total",
				Help: "Total detected nonce replays",
			},
		),
	}
}

// ObserveChallenge 记录一次 challenge。
func (m *AAPMetrics) ObserveChallenge(version, domain, result string, d time.Duration) {
	m.ChallengeTotal.WithLabelValues(version, domain, result).Inc()
	m.ChallengeLatency.Observe(d.Seconds())
}

// ObserveVerify 记录一次 verify。
func (m *AAPMetrics) ObserveVerify(version, result, reason string, d time.Duration) {
	m.VerifyTotal.WithLabelValues(version, result, reason).Inc()
	m.VerifyLatency.Observe(d.Seconds())
}

// IncActiveSession 新增活跃 session。
func (m *AAPMetrics) IncActiveSession() { m.ActiveSessions.Inc() }

// DecActiveSession 减少活跃 session。
func (m *AAPMetrics) DecActiveSession() { m.ActiveSessions.Dec() }

// IncNonceReplay 检测到 nonce 重放。
func (m *AAPMetrics) IncNonceReplay() { m.NonceReplays.Inc() }

// SuccessRate 计算 AAP 验证成功率（0.0 - 1.0）。
//
// 公式：verify_total{result="success"} / verify_total
func (m *AAPMetrics) SuccessRate() float64 {
	// 简化：假设 success counter 在 label result="success"
	// 实际 PromQL：sum(rate(aap_verify_total{result="success"}[5m])) / sum(rate(aap_verify_total[5m]))
	// 这里只返回占位
	return 0.0
}

// 全局默认
var (
	defaultAAPMetricsOnce sync.Once
	defaultAAPMetrics     *AAPMetrics
)

// DefaultAAPMetrics 返回全局默认指标集。
func DefaultAAPMetrics() *AAPMetrics {
	defaultAAPMetricsOnce.Do(func() {
		defaultAAPMetrics = NewAAPMetrics(prometheus.DefaultRegisterer)
	})
	return defaultAAPMetrics
}
