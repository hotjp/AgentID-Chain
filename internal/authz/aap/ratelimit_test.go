package aap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/alicebob/miniredis/v2"
)

func newLimiter(t *testing.T, limit int64, window time.Duration) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	store := cache.NewMiniredis(mr)
	l, err := NewLimiter(LimiterConfig{
		Cache:  store,
		Limit:  limit,
		Window: window,
	})
	if err != nil {
		t.Fatal(err)
	}
	return l, mr
}

func TestNewLimiter_Defaults(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	l, err := NewLimiter(LimiterConfig{Cache: cache.NewMiniredis(mr)})
	if err != nil {
		t.Fatal(err)
	}
	if l.Limit() != 10 {
		t.Errorf("default Limit = %d", l.Limit())
	}
	if l.Window() != time.Minute {
		t.Errorf("default Window = %v", l.Window())
	}
}

func TestNewLimiter_NilCache(t *testing.T) {
	_, err := NewLimiter(LimiterConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLimiter_Allow_OK(t *testing.T) {
	l, _ := newLimiter(t, 10, time.Minute)
	for i := 0; i < 10; i++ {
		ok, remaining, err := l.Allow(context.Background(), "1.2.3.4", "owner1")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("request %d denied", i)
		}
		if remaining != 10-int64(i+1) {
			t.Errorf("request %d remaining = %d, want %d", i, remaining, 10-(i+1))
		}
	}
}

func TestLimiter_Allow_OverLimit(t *testing.T) {
	l, _ := newLimiter(t, 3, time.Minute)
	// 3 次允许
	for i := 0; i < 3; i++ {
		ok, _, _ := l.Allow(context.Background(), "1.2.3.4", "owner1")
		if !ok {
			t.Fatalf("request %d should pass", i)
		}
	}
	// 第 4 次拒绝
	ok, _, err := l.Allow(context.Background(), "1.2.3.4", "owner1")
	if ok {
		t.Error("4th request should be denied")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestLimiter_DifferentOwnerDIDs(t *testing.T) {
	l, _ := newLimiter(t, 2, time.Minute)
	// owner1 满
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	ok, _, _ := l.Allow(context.Background(), "1.2.3.4", "owner1")
	if ok {
		t.Error("owner1 should be limited")
	}
	// owner2 独立计数
	ok, _, _ = l.Allow(context.Background(), "1.2.3.4", "owner2")
	if !ok {
		t.Error("owner2 should not be limited")
	}
}

func TestLimiter_DifferentIPs(t *testing.T) {
	l, _ := newLimiter(t, 2, time.Minute)
	// IP 1 满
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	// IP 2 独立
	ok, _, _ := l.Allow(context.Background(), "5.6.7.8", "owner1")
	if !ok {
		t.Error("different IP should not be limited")
	}
}

func TestLimiter_WindowReset(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	store := cache.NewMiniredis(mr)
	now := time.Now()
	l, _ := NewLimiter(LimiterConfig{
		Cache:  store,
		Limit:  2,
		Window: 60 * time.Second,
		Clock:  func() time.Time { return now },
	})
	// 满
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	ok, _, _ := l.Allow(context.Background(), "1.2.3.4", "owner1")
	if ok {
		t.Fatal("should be limited")
	}
	// 时间前进 → 跨窗口
	l.cfg.Clock = func() time.Time { return now.Add(120 * time.Second) }
	ok, _, _ = l.Allow(context.Background(), "1.2.3.4", "owner1")
	if !ok {
		t.Error("new window should reset count")
	}
}

func TestLimiter_Reset(t *testing.T) {
	l, _ := newLimiter(t, 2, time.Minute)
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	l.Allow(context.Background(), "1.2.3.4", "owner1")
	if err := l.Reset(context.Background(), "1.2.3.4", "owner1"); err != nil {
		t.Fatal(err)
	}
	ok, remaining, _ := l.Allow(context.Background(), "1.2.3.4", "owner1")
	if !ok {
		t.Error("after reset should pass")
	}
	if remaining != 1 {
		t.Errorf("remaining = %d, want 1", remaining)
	}
}

func TestLimiter_AllowWith(t *testing.T) {
	l, _ := newLimiter(t, 2, time.Minute)
	ok, _, _ := l.AllowWith(context.Background(), "custom-suffix")
	if !ok {
		t.Error("first should pass")
	}
	ok, _, _ = l.AllowWith(context.Background(), "custom-suffix")
	if !ok {
		t.Error("second should pass")
	}
	ok, _, _ = l.AllowWith(context.Background(), "custom-suffix")
	if ok {
		t.Error("third should be limited")
	}
}

func TestLimiter_FailOpen(t *testing.T) {
	// 用一个会失败的 store
	badStore := &failingCache{}
	l, err := NewLimiter(LimiterConfig{
		Cache:   badStore,
		Limit:   2,
		Window:  time.Minute,
		FailOpen: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ok, _, err := l.Allow(context.Background(), "1.2.3.4", "owner1")
	if !ok {
		t.Error("FailOpen should allow")
	}
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestLimiter_FailClose(t *testing.T) {
	badStore := &failingCache{}
	l, err := NewLimiter(LimiterConfig{
		Cache:    badStore,
		Limit:    2,
		Window:   time.Minute,
		FailOpen: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	ok, _, err := l.Allow(context.Background(), "1.2.3.4", "owner1")
	if ok {
		t.Error("FailClose should deny")
	}
	if err == nil {
		t.Error("expected error")
	}
}

func TestLimiter_WrapHTTP_Allows(t *testing.T) {
	l, _ := newLimiter(t, 5, time.Minute)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := l.WrapHTTP(next)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.Header.Set("X-AAP-User-DID", "owner1")
	handler.ServeHTTP(rr, r)
	if !called {
		t.Error("next should be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d", rr.Code)
	}
	if rr.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("X-RateLimit-Limit = %s", rr.Header().Get("X-RateLimit-Limit"))
	}
}

func TestLimiter_WrapHTTP_Denies(t *testing.T) {
	l, _ := newLimiter(t, 1, time.Minute)
	// 第一次：放行
	rr1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.Header.Set("X-Forwarded-For", "1.2.3.4")
	r1.Header.Set("X-AAP-User-DID", "owner1")
	handler := l.WrapHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr1, r1)
	if rr1.Code != http.StatusOK {
		t.Errorf("first status = %d", rr1.Code)
	}
	// 第二次：拒绝
	rr2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("X-Forwarded-For", "1.2.3.4")
	r2.Header.Set("X-AAP-User-DID", "owner1")
	handler.ServeHTTP(rr2, r2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second status = %d, want 429", rr2.Code)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	r.RemoteAddr = "10.0.0.1:1234"
	if got := clientIP(r); got != "1.2.3.4" {
		t.Errorf("got %q, want 1.2.3.4", got)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "1.2.3.4")
	if got := clientIP(r); got != "1.2.3.4" {
		t.Errorf("got %q, want 1.2.3.4", got)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:5678"
	if got := clientIP(r); got != "1.2.3.4" {
		t.Errorf("got %q, want 1.2.3.4", got)
	}
}

func TestClientIP_NoAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = ""
	if got := clientIP(r); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestKey_Format(t *testing.T) {
	l, _ := newLimiter(t, 10, time.Minute)
	key := l.key("1.2.3.4", "owner1")
	if key == "" {
		t.Error("empty key")
	}
	// 应包含 prefix、ip、owner_did
	if !contains(key, "rl:aap") || !contains(key, "1.2.3.4") || !contains(key, "owner1") {
		t.Errorf("key missing parts: %s", key)
	}
}

// failingCache 用于测试错误路径的 cache stub。
type failingCache struct{}

func (f *failingCache) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("fail")
}
func (f *failingCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return errors.New("fail")
}
func (f *failingCache) Del(_ context.Context, _ ...string) error         { return errors.New("fail") }
func (f *failingCache) Exists(_ context.Context, _ string) (bool, error)  { return false, errors.New("fail") }
func (f *failingCache) Expire(_ context.Context, _ string, _ time.Duration) error {
	return errors.New("fail")
}
func (f *failingCache) Incr(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, errors.New("fail")
}
func (f *failingCache) StoreOnce(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return errors.New("fail")
}
func (f *failingCache) Close() error { return nil }
