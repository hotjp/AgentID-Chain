// Package telemetry: 缓存命中率监控 (P19.7)。
//
// 指标：
//   - cache_operations_total{backend, op, result} counter
//   - cache_hit_ratio{backend}                        gauge（自计算）
//   - cache_latency_seconds{backend, op}              histogram
//
// result: hit | miss | error
package telemetry

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// CacheMetrics 缓存指标集。
type CacheMetrics struct {
	Operations *prometheus.CounterVec
	Latency    *prometheus.HistogramVec
	Bytes      *prometheus.HistogramVec
	Keys       prometheus.Gauge

	// 内部状态：用于计算 hit ratio
	mu        sync.Mutex
	hitCount  uint64
	missCount uint64
}

// NewCacheMetrics 构造缓存指标集。
func NewCacheMetrics(reg prometheus.Registerer) *CacheMetrics {
	factory := promauto.With(reg)
	return &CacheMetrics{
		Operations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_operations_total",
				Help: "Total cache operations",
			},
			[]string{"backend", "op", "result"}, // result: hit|miss|error
		),
		Latency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cache_latency_seconds",
				Help:    "Cache operation latency",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.05, 0.1},
			},
			[]string{"backend", "op"},
		),
		Bytes: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cache_value_size_bytes",
				Help:    "Cache value size in bytes",
				Buckets: prometheus.ExponentialBuckets(64, 4, 8),
			},
			[]string{"backend"},
		),
		Keys: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "cache_keys",
				Help: "Current number of keys in cache",
			},
		),
	}
}

// ObserveHit 记录命中。
func (m *CacheMetrics) ObserveHit(backend, op string, d time.Duration) {
	m.Operations.WithLabelValues(backend, op, "hit").Inc()
	m.Latency.WithLabelValues(backend, op).Observe(d.Seconds())
	m.mu.Lock()
	m.hitCount++
	m.mu.Unlock()
}

// ObserveMiss 记录未命中。
func (m *CacheMetrics) ObserveMiss(backend, op string, d time.Duration) {
	m.Operations.WithLabelValues(backend, op, "miss").Inc()
	m.Latency.WithLabelValues(backend, op).Observe(d.Seconds())
	m.mu.Lock()
	m.missCount++
	m.mu.Unlock()
}

// ObserveError 记录错误。
func (m *CacheMetrics) ObserveError(backend, op string) {
	m.Operations.WithLabelValues(backend, op, "error").Inc()
}

// ObserveSize 记录 value 字节数。
func (m *CacheMetrics) ObserveSize(backend string, size int) {
	m.Bytes.WithLabelValues(backend).Observe(float64(size))
}

// SetKeyCount 设置当前 key 数。
func (m *CacheMetrics) SetKeyCount(n int) {
	m.Keys.Set(float64(n))
}

// HitRatio 计算命中率（0.0 - 1.0）。
func (m *CacheMetrics) HitRatio() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := m.hitCount + m.missCount
	if total == 0 {
		return 0
	}
	return float64(m.hitCount) / float64(total)
}

// ResetRatio 重置命中率统计。
func (m *CacheMetrics) ResetRatio() {
	m.mu.Lock()
	m.hitCount = 0
	m.missCount = 0
	m.mu.Unlock()
}

// 全局默认
var (
	defaultCacheMetricsOnce sync.Once
	defaultCacheMetrics     *CacheMetrics
)

// DefaultCacheMetrics 返回全局默认指标集。
func DefaultCacheMetrics() *CacheMetrics {
	defaultCacheMetricsOnce.Do(func() {
		defaultCacheMetrics = NewCacheMetrics(prometheus.DefaultRegisterer)
	})
	return defaultCacheMetrics
}
