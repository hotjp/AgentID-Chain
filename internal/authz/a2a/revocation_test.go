package a2a

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/alicebob/miniredis/v2"
)

func newTestRevoker(t *testing.T) *Revoker {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	r, err := NewRevoker(RevokerConfig{Cache: cache.NewMiniredis(mr)})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// =============================================================================
// NewRevoker
// =============================================================================

func TestNewRevoker_NilCache(t *testing.T) {
	_, err := NewRevoker(RevokerConfig{})
	if !errors.Is(err, ErrCacheUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestNewRevoker_Defaults(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	r, err := NewRevoker(RevokerConfig{Cache: cache.NewMiniredis(mr)})
	if err != nil {
		t.Fatal(err)
	}
	if r.cfg.MaxTTL != MaxRevocationTTL {
		t.Errorf("MaxTTL = %v", r.cfg.MaxTTL)
	}
	if r.cfg.KeyPrefix != "a2a:revoked:" {
		t.Errorf("KeyPrefix = %q", r.cfg.KeyPrefix)
	}
	if r.cfg.Clock == nil {
		t.Error("clock should default")
	}
}

// =============================================================================
// Track
// =============================================================================

func TestTrack_EmptyUUID(t *testing.T) {
	r := newTestRevoker(t)
	err := r.Track("", "jti", time.Now())
	if err == nil {
		t.Error("expected error")
	}
}

func TestTrack_EmptyJTI(t *testing.T) {
	r := newTestRevoker(t)
	err := r.Track("uuid", "", time.Now())
	if !errors.Is(err, ErrJTIRequired) {
		t.Errorf("err = %v", err)
	}
}

func TestTrack_OK(t *testing.T) {
	r := newTestRevoker(t)
	if err := r.Track("uuid-1", "jti-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := r.ActiveJTIs("uuid-1"); len(got) != 1 || got[0] != "jti-1" {
		t.Errorf("ActiveJTIs = %v", got)
	}
}

func TestTrack_MultipleTokens(t *testing.T) {
	r := newTestRevoker(t)
	exp := time.Now().Add(time.Hour)
	_ = r.Track("uuid-1", "jti-1", exp)
	_ = r.Track("uuid-1", "jti-2", exp)
	_ = r.Track("uuid-1", "jti-3", exp)
	if len(r.ActiveJTIs("uuid-1")) != 3 {
		t.Errorf("count = %d", len(r.ActiveJTIs("uuid-1")))
	}
}

// =============================================================================
// Revoke 单 token
// =============================================================================

func TestRevoke_EmptyJTI(t *testing.T) {
	r := newTestRevoker(t)
	if err := r.Revoke(context.Background(), ""); !errors.Is(err, ErrJTIRequired) {
		t.Errorf("err = %v", err)
	}
}

func TestRevoke_Happy(t *testing.T) {
	r := newTestRevoker(t)
	exp := time.Now().Add(30 * time.Minute)
	_ = r.Track("uuid-1", "jti-1", exp)
	if err := r.Revoke(context.Background(), "jti-1"); err != nil {
		t.Fatal(err)
	}
	if !r.IsRevoked(context.Background(), "jti-1") {
		t.Error("should be revoked")
	}
	// 索引应被清空
	if len(r.ActiveJTIs("uuid-1")) != 0 {
		t.Error("index not cleared")
	}
}

func TestRevoke_UntrackedJTI(t *testing.T) {
	r := newTestRevoker(t)
	// 没 Track 过，但仍可 Revoke（用 MaxTTL）
	if err := r.Revoke(context.Background(), "jti-unknown"); err != nil {
		t.Fatal(err)
	}
	if !r.IsRevoked(context.Background(), "jti-unknown") {
		t.Error("should be revoked")
	}
}

func TestRevoke_ExpiredToken_Skip(t *testing.T) {
	// 已过期的 token 不再写 revoked 标记，仅清理索引
	r := newTestRevoker(t)
	past := time.Now().Add(-time.Minute)
	_ = r.Track("uuid-1", "jti-old", past)
	if err := r.Revoke(context.Background(), "jti-old"); err != nil {
		t.Fatal(err)
	}
	if r.IsRevoked(context.Background(), "jti-old") {
		t.Error("expired should not be marked revoked")
	}
	if len(r.ActiveJTIs("uuid-1")) != 0 {
		t.Error("index not cleared")
	}
}

func TestRevoke_TTLCappedAtMax(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	r, _ := NewRevoker(RevokerConfig{
		Cache:  cache.NewMiniredis(mr),
		MaxTTL: 1 * time.Minute,
	})
	// Track 一个 24h 后过期的 token
	_ = r.Track("uuid-1", "jti-long", time.Now().Add(24*time.Hour))
	_ = r.Revoke(context.Background(), "jti-long")
	ttl := mr.TTL("a2a:revoked:jti-long")
	if ttl > time.Minute+5*time.Second {
		t.Errorf("TTL = %v, want <= 1min+leeway", ttl)
	}
}

// =============================================================================
// RevokeByAgent
// =============================================================================

func TestRevokeByAgent_EmptyUUID(t *testing.T) {
	r := newTestRevoker(t)
	_, err := r.RevokeByAgent(context.Background(), "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestRevokeByAgent_NoTokens(t *testing.T) {
	r := newTestRevoker(t)
	jtis, err := r.RevokeByAgent(context.Background(), "uuid-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(jtis) != 0 {
		t.Errorf("jtis = %v", jtis)
	}
}

func TestRevokeByAgent_AllRevoked(t *testing.T) {
	r := newTestRevoker(t)
	exp := time.Now().Add(time.Hour)
	_ = r.Track("uuid-1", "j1", exp)
	_ = r.Track("uuid-1", "j2", exp)
	_ = r.Track("uuid-1", "j3", exp)
	_ = r.Track("uuid-2", "other", exp) // 不应被撤销

	revoked, err := r.RevokeByAgent(context.Background(), "uuid-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(revoked) != 3 {
		t.Errorf("revoked count = %d", len(revoked))
	}
	for _, j := range []string{"j1", "j2", "j3"} {
		if !r.IsRevoked(context.Background(), j) {
			t.Errorf("%s not revoked", j)
		}
	}
	if r.IsRevoked(context.Background(), "other") {
		t.Error("uuid-2 token should not be revoked")
	}
}

// =============================================================================
// IsRevoked
// =============================================================================

func TestIsRevoked_EmptyJTI(t *testing.T) {
	r := newTestRevoker(t)
	if r.IsRevoked(context.Background(), "") {
		t.Error("empty jti")
	}
}

func TestIsRevoked_NotRevoked(t *testing.T) {
	r := newTestRevoker(t)
	if r.IsRevoked(context.Background(), "j-unknown") {
		t.Error("should not be revoked")
	}
}

func TestIsRevoked_CacheErrorFailsOpen(t *testing.T) {
	mr, _ := miniredis.Run()
	c := cache.NewMiniredis(mr)
	mr.Close() // 模拟 cache 故障
	r, _ := NewRevoker(RevokerConfig{Cache: c})
	if r.IsRevoked(context.Background(), "any") {
		t.Error("cache error should fail open (return false)")
	}
}

// =============================================================================
// GC
// =============================================================================

func TestGC_RemovesExpired(t *testing.T) {
	r := newTestRevoker(t)
	now := time.Now()
	_ = r.Track("u1", "j-fresh", now.Add(time.Hour))
	_ = r.Track("u1", "j-old", now.Add(-time.Minute))
	_ = r.Track("u2", "j2-old", now.Add(-time.Minute))

	n := r.GC()
	if n != 2 {
		t.Errorf("cleaned %d, want 2", n)
	}
	if len(r.ActiveJTIs("u1")) != 1 {
		t.Errorf("u1 active = %v", r.ActiveJTIs("u1"))
	}
	if len(r.ActiveJTIs("u2")) != 0 {
		t.Errorf("u2 active = %v", r.ActiveJTIs("u2"))
	}
}

// =============================================================================
// keyFor 前缀去重
// =============================================================================

func TestKeyFor_NoDoublePrefix(t *testing.T) {
	r := newTestRevoker(t)
	k1 := r.keyFor("jti-1")
	k2 := r.keyFor("a2a:revoked:jti-1")
	if k1 != "a2a:revoked:jti-1" {
		t.Errorf("k1 = %q", k1)
	}
	if k2 != "a2a:revoked:jti-1" {
		t.Errorf("k2 = %q (double prefix)", k2)
	}
}

func TestKeyFor_CustomPrefix(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	r, _ := NewRevoker(RevokerConfig{
		Cache:     cache.NewMiniredis(mr),
		KeyPrefix: "custom:",
	})
	if got := r.keyFor("x"); got != "custom:x" {
		t.Errorf("got %q", got)
	}
}
