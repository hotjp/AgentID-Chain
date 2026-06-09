package storage

import (
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
