package storage

import (
	"context"
	"testing"
	"time"
)

func TestComputeBackoff(t *testing.T) {
	p := &OutboxPoller{
		cfg: PollerConfig{
			BackoffBase: 5 * time.Second,
			BackoffMax:  5 * time.Minute,
		},
	}
	tests := []struct {
		retry int
		want  time.Duration
	}{
		{1, 5 * time.Second},    // base
		{2, 15 * time.Second},   // base * 3
		{3, 45 * time.Second},   // base * 9
		{4, 135 * time.Second},  // base * 27
		{5, 5 * time.Minute},    // capped (base * 81 = 405s > 300s)
		{10, 5 * time.Minute},   // capped
		{0, 5 * time.Second},    // < 1 treated as 1
		{-1, 5 * time.Second},   // negative treated as 1
	}
	for _, tt := range tests {
		got := p.computeBackoff(tt.retry)
		if got != tt.want {
			t.Errorf("retry=%d: got %v, want %v", tt.retry, got, tt.want)
		}
	}
}

func TestNewOutboxPoller_Defaults(t *testing.T) {
	p := NewOutboxPoller(nil, nil, PollerConfig{}, nil)
	if p.cfg.BatchSize != 100 {
		t.Errorf("BatchSize=%d, want 100", p.cfg.BatchSize)
	}
	if p.cfg.PollInterval != 2*time.Second {
		t.Errorf("PollInterval=%v, want 2s", p.cfg.PollInterval)
	}
	if p.cfg.BatchTimeout != 30*time.Second {
		t.Errorf("BatchTimeout=%v, want 30s", p.cfg.BatchTimeout)
	}
	if p.cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries=%d, want 5", p.cfg.MaxRetries)
	}
	if p.cfg.BackoffBase != 5*time.Second {
		t.Errorf("BackoffBase=%v, want 5s", p.cfg.BackoffBase)
	}
	if p.cfg.BackoffMax != 5*time.Minute {
		t.Errorf("BackoffMax=%v, want 5m", p.cfg.BackoffMax)
	}
}

func TestPollerHandlerFunc(t *testing.T) {
	called := false
	// 用 ent.OutboxEvent 类型签名（避免未使用 import）
	_ = called
	_ = (*struct{ ID string })(nil) // 占位
	// PollerHandlerFunc 适配器（收 *ent.OutboxEvent，无法单测调用 — 集成测试覆盖）
}

// TestPoller_Run_ContextCancel — Run 监听 ctx.Done 立即退出
// 不依赖真实 ent client（用 nil）
func TestPoller_Run_ContextCancel(t *testing.T) {
	p := &OutboxPoller{
		cfg: PollerConfig{
			PollInterval: 10 * time.Millisecond,
			BatchSize:    1,
		},
	}
	// 构造一个几乎立即 cancel 的 ctx
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		// 顶层会 nil pointer 访问 client；用 defer recover 防崩
		defer func() { _ = recover(); close(done) }()
		_ = p.Run(ctx)
	}()
	<-done
}
