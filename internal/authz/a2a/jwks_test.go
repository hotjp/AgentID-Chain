package a2a

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newPubKey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

// =============================================================================
// KeySource implementations
// =============================================================================

func TestStaticKeySource_SortsByKid(t *testing.T) {
	src := &StaticKeySource{
		Entries: []KeyEntry{
			{Kid: "c", Public: newPubKey(t)},
			{Kid: "a", Public: newPubKey(t)},
			{Kid: "b", Public: newPubKey(t)},
		},
	}
	out, err := src.Keys()
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Kid != "a" || out[1].Kid != "b" || out[2].Kid != "c" {
		t.Errorf("not sorted: %v", out)
	}
}

func TestStaticKeySource_Empty(t *testing.T) {
	src := &StaticKeySource{}
	out, err := src.Keys()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("len = %d", len(out))
	}
}

func TestIssuerKeySource_OK(t *testing.T) {
	is := newTestIssuer(t)
	src := &IssuerKeySource{Issuer: is}
	out, err := src.Keys()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0].Kid != "test-key-1" {
		t.Errorf("kid = %q", out[0].Kid)
	}
}

func TestIssuerKeySource_NilIssuer(t *testing.T) {
	src := &IssuerKeySource{}
	_, err := src.Keys()
	if err == nil {
		t.Error("expected error")
	}
}

// =============================================================================
// NewJWKSHandler
// =============================================================================

func TestNewJWKSHandler_NilSource(t *testing.T) {
	_, err := NewJWKSHandler(JWKSHandlerConfig{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestNewJWKSHandler_Defaults(t *testing.T) {
	h, err := NewJWKSHandler(JWKSHandlerConfig{Source: &StaticKeySource{}})
	if err != nil {
		t.Fatal(err)
	}
	if h.cfg.CacheTTL != 5*time.Minute {
		t.Errorf("CacheTTL = %v", h.cfg.CacheTTL)
	}
	if h.cfg.Clock == nil {
		t.Error("clock should default")
	}
}

// =============================================================================
// ServeHTTP
// =============================================================================

func newTestHandler(t *testing.T) (*JWKSHandler, ed25519.PublicKey) {
	t.Helper()
	pub := newPubKey(t)
	h, err := NewJWKSHandler(JWKSHandlerConfig{
		Source: &StaticKeySource{Entries: []KeyEntry{
			{Kid: "k1", Public: pub},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, pub
}

func TestServeHTTP_Get_OK(t *testing.T) {
	h, pub := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.HasPrefix(cc, "public, max-age=") {
		t.Errorf("Cache-Control = %q", cc)
	}

	var jwks JWKS
	if err := json.NewDecoder(w.Body).Decode(&jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("keys = %d", len(jwks.Keys))
	}
	k := jwks.Keys[0]
	if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Alg != "EdDSA" || k.Use != "sig" {
		t.Errorf("bad jwk: %+v", k)
	}
	if k.Kid != "k1" {
		t.Errorf("kid = %q", k.Kid)
	}
	dec, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != string(pub) {
		t.Error("pubkey mismatch")
	}
}

func TestServeHTTP_Head(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodHead, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("HEAD body should be empty, got %d bytes", w.Body.Len())
	}
}

func TestServeHTTP_PostRejected(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d", w.Code)
	}
}

func TestServeHTTP_SourceError(t *testing.T) {
	h, _ := NewJWKSHandler(JWKSHandlerConfig{Source: &failingSource{}})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d", w.Code)
	}
}

func TestServeHTTP_SkipInvalidKey(t *testing.T) {
	h, _ := NewJWKSHandler(JWKSHandlerConfig{
		Source: &StaticKeySource{Entries: []KeyEntry{
			{Kid: "ok", Public: newPubKey(t)},
			{Kid: "bad", Public: []byte("too-short")},
		}},
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var jwks JWKS
	_ = json.NewDecoder(w.Body).Decode(&jwks)
	if len(jwks.Keys) != 1 {
		t.Errorf("should skip invalid: got %d keys", len(jwks.Keys))
	}
}

type failingSource struct{}

func (failingSource) Keys() ([]KeyEntry, error) {
	return nil, errors.New("boom")
}

// =============================================================================
// 缓存 / 刷新
// =============================================================================

func TestSnapshot_ReusesCache(t *testing.T) {
	src := &countingSource{}
	h, _ := NewJWKSHandler(JWKSHandlerConfig{Source: src})
	_, _ = h.Snapshot()
	_, _ = h.Snapshot()
	if src.calls != 1 {
		t.Errorf("calls = %d, want 1", src.calls)
	}
}

func TestSnapshot_RefreshInterval(t *testing.T) {
	src := &countingSource{}
	now := time.Now()
	clock := now
	h, _ := NewJWKSHandler(JWKSHandlerConfig{
		Source:          src,
		RefreshInterval: 100 * time.Millisecond,
		Clock:           func() time.Time { return clock },
	})
	_, _ = h.Snapshot() // fetch 1
	_, _ = h.Snapshot() // cached
	if src.calls != 1 {
		t.Errorf("after 2 calls: %d", src.calls)
	}
	// 推进时间到 interval 后
	clock = now.Add(200 * time.Millisecond)
	_, _ = h.Snapshot()
	if src.calls != 2 {
		t.Errorf("after refresh: %d", src.calls)
	}
}

func TestRefresh_Manual(t *testing.T) {
	src := &countingSource{}
	h, _ := NewJWKSHandler(JWKSHandlerConfig{Source: src})
	_ = h.Refresh()
	_ = h.Refresh()
	if src.calls != 2 {
		t.Errorf("calls = %d", src.calls)
	}
}

func TestSnapshot_DeepCopy(t *testing.T) {
	pub := newPubKey(t)
	h, _ := NewJWKSHandler(JWKSHandlerConfig{
		Source: &StaticKeySource{Entries: []KeyEntry{{Kid: "k", Public: pub}}},
	})
	a, _ := h.Snapshot()
	a.Keys[0].Kid = "MUTATED"
	b, _ := h.Snapshot()
	if b.Keys[0].Kid != "k" {
		t.Error("Snapshot returned shared reference")
	}
}

type countingSource struct {
	calls int
}

func (c *countingSource) Keys() ([]KeyEntry, error) {
	c.calls++
	return []KeyEntry{{Kid: "k", Public: make(ed25519.PublicKey, ed25519.PublicKeySize)}}, nil
}

// =============================================================================
// ResolverFromJWKS — JWKS → KeyResolver 闭环
// =============================================================================

func TestResolverFromJWKS_NilJWKS(t *testing.T) {
	_, err := ResolverFromJWKS(nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestResolverFromJWKS_RoundTrip(t *testing.T) {
	is := newTestIssuer(t)
	h, _ := NewJWKSHandler(JWKSHandlerConfig{
		Source: &IssuerKeySource{Issuer: is},
	})
	snap, err := h.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	r, err := ResolverFromJWKS(snap)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := r.Resolve("test-key-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(pub) != string(is.PublicKey()) {
		t.Error("round-trip pubkey mismatch")
	}
}

func TestResolverFromJWKS_SkipsWrongCrv(t *testing.T) {
	jwks := &JWKS{Keys: []JWK{
		{Kty: "RSA", Crv: "P-256", Kid: "rsa", X: "xxx"},
		{Kty: "OKP", Crv: "Ed25519", Kid: "ed",
			X: base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize))},
	}}
	r, _ := ResolverFromJWKS(jwks)
	mr := r.(*MapKeyResolver)
	if _, ok := mr.Keys["rsa"]; ok {
		t.Error("should skip RSA")
	}
	if _, ok := mr.Keys["ed"]; !ok {
		t.Error("should keep ed25519")
	}
}

func TestResolverFromJWKS_SkipsBadB64(t *testing.T) {
	jwks := &JWKS{Keys: []JWK{
		{Kty: "OKP", Crv: "Ed25519", Kid: "bad", X: "!!!"},
	}}
	r, _ := ResolverFromJWKS(jwks)
	mr := r.(*MapKeyResolver)
	if len(mr.Keys) != 0 {
		t.Error("bad b64 should be skipped")
	}
}
