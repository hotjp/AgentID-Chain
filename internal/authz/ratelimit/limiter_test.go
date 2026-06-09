package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/alicebob/miniredis/v2"
)

func newTestLimiter(t *testing.T, limit int64, window time.Duration) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	l, err := NewLimiter(LimiterConfig{
		Cache:  cache.NewMiniredis(mr),
		Limit:  limit,
		Window: window,
		Scope:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return l, mr
}

// =============================================================================
// NewLimiter
// =============================================================================

func TestNewLimiter_NilCache(t *testing.T) {
	_, err := NewLimiter(LimiterConfig{})
	if !errors.Is(err, ErrCacheUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestNewLimiter_Defaults(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	l, err := NewLimiter(LimiterConfig{Cache: cache.NewMiniredis(mr)})
	if err != nil {
		t.Fatal(err)
	}
	if l.cfg.Limit != 60 {
		t.Errorf("Limit = %d", l.cfg.Limit)
	}
	if l.cfg.Window != time.Minute {
		t.Errorf("Window = %v", l.cfg.Window)
	}
	if l.cfg.Scope != "default" {
		t.Errorf("Scope = %q", l.cfg.Scope)
	}
	if l.cfg.KeyPrefix != "rl:" {
		t.Errorf("KeyPrefix = %q", l.cfg.KeyPrefix)
	}
	if l.cfg.Clock == nil {
		t.Error("clock should default")
	}
}

// =============================================================================
// Allow / Check
// =============================================================================

func TestAllow_EmptyKey(t *testing.T) {
	l, _ := newTestLimiter(t, 5, time.Minute)
	_, err := l.Allow(context.Background(), "")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("err = %v", err)
	}
}

func TestAllow_UnderLimit(t *testing.T) {
	l, _ := newTestLimiter(t, 5, time.Minute)
	for i := 0; i < 5; i++ {
		ok, err := l.Allow(context.Background(), "user-1")
		if err != nil {
			t.Errorf("Allow %d: %v", i, err)
		}
		if !ok {
			t.Errorf("call %d denied", i)
		}
	}
}

func TestAllow_ExceedsLimit(t *testing.T) {
	l, _ := newTestLimiter(t, 3, time.Minute)
	// 3 次允许
	for i := 0; i < 3; i++ {
		ok, _ := l.Allow(context.Background(), "user-1")
		if !ok {
			t.Errorf("under limit %d denied", i)
		}
	}
	// 第 4 次拒绝
	ok, err := l.Allow(context.Background(), "user-1")
	if ok {
		t.Error("4th call should be denied")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v", err)
	}
}

func TestCheck_RemainingDecrements(t *testing.T) {
	l, _ := newTestLimiter(t, 5, time.Minute)
	prev := int64(5)
	for i := 0; i < 5; i++ {
		d, _ := l.Check(context.Background(), "user-1")
		if d.Remaining > prev {
			t.Errorf("call %d: remaining %d > prev %d", i, d.Remaining, prev)
		}
		prev = d.Remaining
	}
}

func TestCheck_RetryAfterSet(t *testing.T) {
	l, _ := newTestLimiter(t, 1, time.Minute)
	_, _ = l.Allow(context.Background(), "u")
	d, _ := l.Check(context.Background(), "u")
	if d.Allowed {
		t.Error("should be denied")
	}
	if d.RetryAfter <= 0 || d.RetryAfter > time.Minute {
		t.Errorf("RetryAfter = %v", d.RetryAfter)
	}
}

func TestAllow_DifferentKeysIndependent(t *testing.T) {
	l, _ := newTestLimiter(t, 2, time.Minute)
	for _, k := range []string{"a", "b", "c"} {
		for i := 0; i < 2; i++ {
			ok, _ := l.Allow(context.Background(), k)
			if !ok {
				t.Errorf("key %s call %d denied", k, i)
			}
		}
	}
}

// =============================================================================
// 滑动窗口估算
// =============================================================================

func TestCheck_SlidingDecay(t *testing.T) {
	// 在窗口开始时打满；推进时间到下一窗口的 50%，估算应 ≈ 0.5 * 上一桶 + 当前
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	mr, _ := miniredis.Run()
	defer mr.Close()
	clock := now
	l, _ := NewLimiter(LimiterConfig{
		Cache:  cache.NewMiniredis(mr),
		Limit:  100,
		Window: 60 * time.Second,
		Scope:  "test",
		Clock:  func() time.Time { return clock },
	})

	// 打满 50 次（当前桶 = bucket 0）
	for i := 0; i < 50; i++ {
		_, _ = l.Allow(context.Background(), "u")
	}

	// 推进 30s → 进入下一窗口的中点
	// 真实"上一桶"= 50, 当前桶 = 0, weight = 1 - 30/60 = 0.5
	// sliding ≈ 50 * 0.5 + 0 = 25（在调用 Allow 后会 +1）
	clock = now.Add(90 * time.Second)
	d, _ := l.Check(context.Background(), "u")
	if d.Current < 20 || d.Current > 30 {
		t.Errorf("sliding ≈ 25, got %.2f", d.Current)
	}
}

func TestCheck_NoPrevBucket(t *testing.T) {
	l, _ := newTestLimiter(t, 10, time.Minute)
	d, err := l.Check(context.Background(), "fresh-user")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed {
		t.Error("first call should be allowed")
	}
	if d.Current != 1 {
		t.Errorf("Current = %.2f", d.Current)
	}
}

// =============================================================================
// Reset
// =============================================================================

func TestReset_EmptyKey(t *testing.T) {
	l, _ := newTestLimiter(t, 5, time.Minute)
	if err := l.Reset(context.Background(), ""); !errors.Is(err, ErrEmptyKey) {
		t.Errorf("err = %v", err)
	}
}

func TestReset_ClearsCount(t *testing.T) {
	l, _ := newTestLimiter(t, 2, time.Minute)
	_, _ = l.Allow(context.Background(), "u")
	_, _ = l.Allow(context.Background(), "u")
	if err := l.Reset(context.Background(), "u"); err != nil {
		t.Fatal(err)
	}
	ok, _ := l.Allow(context.Background(), "u")
	if !ok {
		t.Error("after reset should be allowed")
	}
}

// =============================================================================
// Fail Open / Fail Close
// =============================================================================

type failingCache struct{}

func (failingCache) Get(context.Context, string) ([]byte, error)             { return nil, errors.New("boom") }
func (failingCache) Set(context.Context, string, []byte, time.Duration) error { return errors.New("boom") }
func (failingCache) Del(context.Context, ...string) error                     { return errors.New("boom") }
func (failingCache) Exists(context.Context, string) (bool, error)             { return false, errors.New("boom") }
func (failingCache) Expire(context.Context, string, time.Duration) error      { return errors.New("boom") }
func (failingCache) Incr(context.Context, string, time.Duration) (int64, error) {
	return 0, errors.New("boom")
}
func (failingCache) StoreOnce(context.Context, string, []byte, time.Duration) error {
	return errors.New("boom")
}
func (failingCache) Close() error { return nil }

func TestAllow_FailOpen(t *testing.T) {
	l, _ := NewLimiter(LimiterConfig{
		Cache:    failingCache{},
		Limit:    5,
		Window:   time.Minute,
		FailOpen: true,
	})
	ok, err := l.Allow(context.Background(), "u")
	if err != nil {
		t.Errorf("FailOpen should not propagate err: %v", err)
	}
	if !ok {
		t.Error("FailOpen should allow")
	}
}

func TestAllow_FailClose(t *testing.T) {
	l, _ := NewLimiter(LimiterConfig{
		Cache:    failingCache{},
		Limit:    5,
		Window:   time.Minute,
		FailOpen: false,
	})
	_, err := l.Allow(context.Background(), "u")
	if err == nil {
		t.Error("FailClose should propagate err")
	}
}

// =============================================================================
// bucketKey 形态
// =============================================================================

func TestBucketKey_Format(t *testing.T) {
	l, _ := newTestLimiter(t, 5, time.Minute)
	k := l.bucketKey("u-1", 123)
	want := "rl:test:u-1:123"
	if k != want {
		t.Errorf("got %q, want %q", k, want)
	}
}
