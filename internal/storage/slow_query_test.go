// Package storage: 慢查询监控测试。
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func newTestMonitor(t *testing.T, threshold time.Duration) (*SlowQueryMonitor, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := SlowQueryConfig{
		Enabled:   true,
		Threshold: threshold,
		Logger:    logger,
	}
	return NewSlowQueryMonitor(cfg), buf
}

func TestSlowQuery_BelowThreshold_NoLog(t *testing.T) {
	m, buf := newTestMonitor(t, 100*time.Millisecond)
	m.Observe(context.Background(), "SELECT 1", nil, 50*time.Millisecond)
	if buf.Len() > 0 {
		t.Fatalf("expected no log, got: %s", buf.String())
	}
}

func TestSlowQuery_AboveThreshold_Logs(t *testing.T) {
	m, buf := newTestMonitor(t, 200*time.Millisecond)
	m.Observe(context.Background(), "SELECT * FROM users WHERE id = $1", []any{42}, 300*time.Millisecond)
	if buf.Len() == 0 {
		t.Fatal("expected log entry")
	}
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["msg"] != "slow query detected" {
		t.Fatalf("unexpected msg: %v", entry["msg"])
	}
	if !strings.Contains(entry["sql"].(string), "SELECT * FROM users") {
		t.Fatalf("sql not in log: %v", entry)
	}
}

func TestSlowQuery_Counter(t *testing.T) {
	m, _ := newTestMonitor(t, 100*time.Millisecond)
	m.Observe(context.Background(), "q1", nil, 50*time.Millisecond)
	m.Observe(context.Background(), "q2", nil, 150*time.Millisecond)
	m.Observe(context.Background(), "q3", nil, 200*time.Millisecond)
	m.Observe(context.Background(), "q4", nil, 300*time.Millisecond)
	stats := m.Stats()
	if stats.Count != 3 {
		t.Fatalf("expected 3 slow queries, got %d", stats.Count)
	}
	if stats.Total != 4 {
		t.Fatalf("expected 4 total, got %d", stats.Total)
	}
}

func TestSlowQuery_Callback(t *testing.T) {
	called := 0
	var info SlowQueryInfo
	cfg := SlowQueryConfig{
		Enabled:   true,
		Threshold: 100 * time.Millisecond,
		OnSlowQuery: func(i SlowQueryInfo) {
			called++
			info = i
		},
	}
	m := NewSlowQueryMonitor(cfg)
	m.Observe(context.Background(), "SELECT 1", nil, 200*time.Millisecond)
	if called != 1 {
		t.Fatalf("expected 1 callback, got %d", called)
	}
	if info.Duration != 200*time.Millisecond {
		t.Fatalf("duration mismatch: %v", info.Duration)
	}
}

func TestSlowQuery_Disabled(t *testing.T) {
	cfg := SlowQueryConfig{Enabled: false, Threshold: 0}
	m := NewSlowQueryMonitor(cfg)
	m.Observe(context.Background(), "q", nil, 10*time.Second)
	if m.Stats().Count != 0 {
		t.Fatal("disabled monitor should not count")
	}
}

func TestHistogram_Basic(t *testing.T) {
	h := NewDurationHistogram()
	h.Observe(5 * time.Millisecond)
	h.Observe(50 * time.Millisecond)
	h.Observe(500 * time.Millisecond)
	h.Observe(2 * time.Second)
	if h.Count() != 4 {
		t.Fatalf("expected count=4, got %d", h.Count())
	}
	if h.Max() != 2*time.Second {
		t.Fatalf("expected max=2s, got %v", h.Max())
	}
	p50 := h.Percentile(0.50)
	p99 := h.Percentile(0.99)
	if p50 == 0 || p99 == 0 {
		t.Fatalf("expected non-zero percentiles, got p50=%v p99=%v", p50, p99)
	}
}

func TestHistogram_Reset(t *testing.T) {
	h := NewDurationHistogram()
	h.Observe(time.Second)
	h.Reset()
	if h.Count() != 0 || h.Max() != 0 {
		t.Fatalf("expected reset, got count=%d max=%v", h.Count(), h.Max())
	}
}

func TestSlowQuery_DefaultThreshold(t *testing.T) {
	m := NewSlowQueryMonitor(SlowQueryConfig{})
	if m.threshold != 200*time.Millisecond {
		t.Fatalf("expected default 200ms, got %v", m.threshold)
	}
}
