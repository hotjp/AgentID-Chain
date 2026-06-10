// Package storage: 慢查询监控 (P18.12)。
//
// 设计：通过 Tracer 接口拦截 SQL 执行，记录超过阈值的查询。
//
// 集成方式（ent ORM）：
//   - ent.Client 接受 appender/driver
//   - 我们用 pgx 的 Tracer 接口或者 wrapping database/sql 的 Conn
//
// 监控指标：
//   - slow_queries_total         counter，按 query 模板归类
//   - slow_query_duration_seconds histogram
//   - 慢查询日志（带 trace_id 关联）
package storage

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// SlowQueryConfig 慢查询监控配置。
type SlowQueryConfig struct {
	// Threshold 慢查询阈值（默认 200ms）。
	Threshold time.Duration
	// Logger 日志输出（nil = slog.Default()）。
	Logger *slog.Logger
	// SampleRate 采样率（0.0-1.0，1.0 = 全量记录）。
	SampleRate float64
	// OnSlowQuery 慢查询回调（可用于上报指标 / 告警）。
	OnSlowQuery func(info SlowQueryInfo)
	// Enabled 是否启用。
	Enabled bool
}

// DefaultSlowQueryConfig 返回默认配置。
func DefaultSlowQueryConfig() SlowQueryConfig {
	return SlowQueryConfig{
		Threshold:  200 * time.Millisecond,
		SampleRate: 1.0,
		Enabled:    true,
	}
}

// SlowQueryInfo 慢查询信息。
type SlowQueryInfo struct {
	// SQL 查询语句。
	SQL string
	// Args 参数。
	Args []any
	// Duration 实际执行时间。
	Duration time.Duration
	// Threshold 慢查询阈值。
	Threshold time.Duration
	// Timestamp 发生时间。
	Timestamp time.Time
	// TraceID 关联的 trace id（如有）。
	TraceID string
}

// SlowQueryMonitor 慢查询监控器。
type SlowQueryMonitor struct {
	cfg       SlowQueryConfig
	threshold time.Duration
	counter   atomic.Int64
	histogram *DurationHistogram
}

// NewSlowQueryMonitor 构造慢查询监控器。
func NewSlowQueryMonitor(cfg SlowQueryConfig) *SlowQueryMonitor {
	if cfg.Threshold == 0 {
		cfg.Threshold = 200 * time.Millisecond
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &SlowQueryMonitor{
		cfg:       cfg,
		threshold: cfg.Threshold,
		histogram: NewDurationHistogram(),
	}
}

// Observe 记录一次 SQL 执行。
func (m *SlowQueryMonitor) Observe(ctx context.Context, sql string, args []any, d time.Duration) {
	if !m.cfg.Enabled {
		return
	}
	m.histogram.Observe(d)
	if d < m.threshold {
		return
	}
	m.counter.Add(1)
	if m.cfg.OnSlowQuery != nil {
		info := SlowQueryInfo{
			SQL:       sql,
			Args:      args,
			Duration:  d,
			Threshold: m.threshold,
			Timestamp: time.Now(),
		}
		m.cfg.OnSlowQuery(info)
	}
	if m.cfg.Logger != nil {
		m.cfg.Logger.WarnContext(ctx, "slow query detected",
			slog.String("sql", sql),
			slog.Duration("duration", d),
			slog.Duration("threshold", m.threshold),
			slog.Time("at", time.Now()),
		)
	}
}

// Stats 返回监控统计。
type SlowQueryStats struct {
	Total       int64
	Count       int64             // 慢查询次数
	P50         time.Duration     // 全量 P50
	P95         time.Duration     // 全量 P95
	P99         time.Duration     // 全量 P99
	Max         time.Duration     // 全量最大
	ByTemplate  map[string]int64  // 按 SQL 模板归类
}

// Stats 获取当前统计快照。
func (m *SlowQueryMonitor) Stats() SlowQueryStats {
	return SlowQueryStats{
		Total: m.histogram.Count(),
		Count: m.counter.Load(),
		P50:   m.histogram.Percentile(0.50),
		P95:   m.histogram.Percentile(0.95),
		P99:   m.histogram.Percentile(0.99),
		Max:   m.histogram.Max(),
	}
}

// Reset 重置统计。
func (m *SlowQueryMonitor) Reset() {
	m.histogram.Reset()
	m.counter.Store(0)
}

// =============================================================================
// DurationHistogram — 轻量级分位数估计
// =============================================================================

// DurationHistogram 简单分位数直方图。
//
// 实现：固定 buckets + 简单排序（仅用于告警，不用于精确 SLI）。
type DurationHistogram struct {
	buckets []time.Duration
	counts  []atomic.Int64
	max     atomic.Int64 // 纳秒
	total   atomic.Int64
	count   atomic.Int64
}

var defaultBuckets = []time.Duration{
	1 * time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	200 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

// NewDurationHistogram 构造直方图。
func NewDurationHistogram() *DurationHistogram {
	return &DurationHistogram{
		buckets: defaultBuckets,
		counts:  make([]atomic.Int64, len(defaultBuckets)+1),
	}
}

// Observe 记录一次观测值。
func (h *DurationHistogram) Observe(d time.Duration) {
	ns := d.Nanoseconds()
	// 更新 max
	for {
		old := h.max.Load()
		if ns <= old {
			break
		}
		if h.max.CompareAndSwap(old, ns) {
			break
		}
	}
	h.total.Add(ns)
	h.count.Add(1)
	// 找 bucket
	for i, b := range h.buckets {
		if d <= b {
			h.counts[i].Add(1)
			return
		}
	}
	h.counts[len(h.buckets)].Add(1) // 超过最后一个 bucket
}

// Count 返回总观测次数。
func (h *DurationHistogram) Count() int64 { return h.count.Load() }

// Max 返回最大观测值。
func (h *DurationHistogram) Max() time.Duration {
	return time.Duration(h.max.Load())
}

// Percentile 返回分位数（线性插值近似）。
func (h *DurationHistogram) Percentile(p float64) time.Duration {
	if p < 0 || p > 1 {
		return 0
	}
	target := int64(float64(h.count.Load()) * p)
	if target == 0 {
		return 0
	}
	var cum int64
	for i := range h.counts {
		cum += h.counts[i].Load()
		if cum >= target {
			if i < len(h.buckets) {
				return h.buckets[i]
			}
			// 超过最大 bucket
			return h.buckets[len(h.buckets)-1]
		}
	}
	return h.buckets[len(h.buckets)-1]
}

// Reset 重置直方图。
func (h *DurationHistogram) Reset() {
	h.max.Store(0)
	h.total.Store(0)
	h.count.Store(0)
	for i := range h.counts {
		h.counts[i].Store(0)
	}
}
