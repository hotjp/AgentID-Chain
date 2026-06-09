package a2a

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func newTestIssuer(t *testing.T) *Issuer {
	t.Helper()
	is, err := NewIssuer(IssuerConfig{
		DomainKey: newTestKey(t),
		Issuer:    "test-issuer",
		KeyID:     "test-key-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return is
}

// =============================================================================
// NewIssuer
// =============================================================================

func TestNewIssuer_EmptyKey(t *testing.T) {
	_, err := NewIssuer(IssuerConfig{})
	if !errors.Is(err, ErrEmptyDomainKey) {
		t.Errorf("err = %v, want ErrEmptyDomainKey", err)
	}
}

func TestNewIssuer_ShortKey(t *testing.T) {
	_, err := NewIssuer(IssuerConfig{DomainKey: []byte{1, 2, 3}})
	if !errors.Is(err, ErrEmptyDomainKey) {
		t.Errorf("err = %v, want ErrEmptyDomainKey", err)
	}
}

func TestNewIssuer_Defaults(t *testing.T) {
	is, err := NewIssuer(IssuerConfig{DomainKey: newTestKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if is.Issuer() != "agentid-chain" {
		t.Errorf("default issuer = %q", is.Issuer())
	}
	if is.cfg.DefaultTTL != 15*time.Minute {
		t.Errorf("default TTL = %v", is.cfg.DefaultTTL)
	}
	if is.cfg.Clock == nil {
		t.Error("clock should default to time.Now")
	}
}

func TestNewIssuer_CustomConfig(t *testing.T) {
	is := newTestIssuer(t)
	if is.Issuer() != "test-issuer" {
		t.Errorf("issuer = %q", is.Issuer())
	}
	if is.KeyID() != "test-key-1" {
		t.Errorf("kid = %q", is.KeyID())
	}
	if len(is.PublicKey()) != ed25519.PublicKeySize {
		t.Errorf("pub key len = %d", len(is.PublicKey()))
	}
}

// =============================================================================
// Sign 校验
// =============================================================================

func TestSign_EmptySubject(t *testing.T) {
	is := newTestIssuer(t)
	_, err := is.Sign(SignInput{Audience: "svc", JTI: "j1"})
	if !errors.Is(err, ErrEmptySubject) {
		t.Errorf("err = %v, want ErrEmptySubject", err)
	}
}

func TestSign_EmptyAudience(t *testing.T) {
	is := newTestIssuer(t)
	_, err := is.Sign(SignInput{Subject: "sub", JTI: "j1"})
	if !errors.Is(err, ErrEmptyAudience) {
		t.Errorf("err = %v, want ErrEmptyAudience", err)
	}
}

func TestSign_EmptyJTI(t *testing.T) {
	is := newTestIssuer(t)
	_, err := is.Sign(SignInput{Subject: "sub", Audience: "svc"})
	if !errors.Is(err, ErrEmptyJTI) {
		t.Errorf("err = %v, want ErrEmptyJTI", err)
	}
}

func TestSign_InvalidTrustLevel(t *testing.T) {
	is := newTestIssuer(t)
	_, err := is.Sign(SignInput{Subject: "sub", Audience: "svc", JTI: "j", TrustLevel: 150})
	if !errors.Is(err, ErrInvalidTrustLevel) {
		t.Errorf("err = %v, want ErrInvalidTrustLevel", err)
	}
	_, err = is.Sign(SignInput{Subject: "sub", Audience: "svc", JTI: "j", TrustLevel: -1})
	if !errors.Is(err, ErrInvalidTrustLevel) {
		t.Errorf("err = %v, want ErrInvalidTrustLevel for negative", err)
	}
}

func TestSign_DefaultTTL(t *testing.T) {
	is := newTestIssuer(t)
	tok, err := is.Sign(SignInput{Subject: "sub", Audience: "svc", JTI: "j"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseUnverified(tok)
	if err != nil {
		t.Fatal(err)
	}
	delta := c.ExpiresAt.Sub(c.IssuedAt)
	if delta < 14*time.Minute || delta > 16*time.Minute {
		t.Errorf("TTL = %v, want ~15min", delta)
	}
}

func TestSign_CustomTTL(t *testing.T) {
	is := newTestIssuer(t)
	tok, _ := is.Sign(SignInput{
		Subject:  "sub",
		Audience: "svc",
		JTI:      "j",
		TTL:      time.Hour,
	})
	c, _ := ParseUnverified(tok)
	delta := c.ExpiresAt.Sub(c.IssuedAt)
	if delta < 59*time.Minute || delta > 61*time.Minute {
		t.Errorf("TTL = %v, want ~1h", delta)
	}
}

// =============================================================================
// Sign happy path + 字段检查
// =============================================================================

func TestSign_HappyPath(t *testing.T) {
	is := newTestIssuer(t)
	tok, err := is.Sign(SignInput{
		Subject:    "agent-uuid-001",
		Audience:   "did:agent:target",
		JTI:        "jti-001",
		TTL:        time.Minute,
		Scope:      "read:tags write:logs",
		TrustLevel: 80,
		AuditID:    "audit-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 3 段
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("parts = %d", len(parts))
	}
	// 解析
	c, err := ParseUnverified(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "agent-uuid-001" {
		t.Errorf("Subject = %q", c.Subject)
	}
	if c.Audience != "did:agent:target" {
		t.Errorf("Audience = %q", c.Audience)
	}
	if c.Issuer != "test-issuer" {
		t.Errorf("Issuer = %q", c.Issuer)
	}
	if c.JTI != "jti-001" {
		t.Errorf("JTI = %q", c.JTI)
	}
	if c.Scope != "read:tags write:logs" {
		t.Errorf("Scope = %q", c.Scope)
	}
	if c.TrustLevel != 80 {
		t.Errorf("TrustLevel = %d", c.TrustLevel)
	}
	if c.AuditID != "audit-001" {
		t.Errorf("AuditID = %q", c.AuditID)
	}
	if c.Kid != "test-key-1" {
		t.Errorf("Kid = %q", c.Kid)
	}
}

func TestSign_TrustLevelBoundaries(t *testing.T) {
	is := newTestIssuer(t)
	for _, tl := range []int{0, 50, 100} {
		_, err := is.Sign(SignInput{
			Subject:    "sub",
			Audience:   "svc",
			JTI:        "j",
			TrustLevel: tl,
		})
		if err != nil {
			t.Errorf("trustLevel=%d: %v", tl, err)
		}
	}
}

func TestSign_ClockOverride(t *testing.T) {
	fixed := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	is, _ := NewIssuer(IssuerConfig{
		DomainKey: newTestKey(t),
		Clock:     func() time.Time { return fixed },
	})
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "a", JTI: "j", TTL: time.Hour})
	c, _ := ParseUnverified(tok)
	if c.IssuedAt.Unix() != fixed.Unix() {
		t.Errorf("IssuedAt = %v, want %v", c.IssuedAt, fixed)
	}
	if c.ExpiresAt.Unix() != fixed.Add(time.Hour).Unix() {
		t.Errorf("ExpiresAt mismatch")
	}
}

func TestSign_HeaderIsEdDSA(t *testing.T) {
	is := newTestIssuer(t)
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "a", JTI: "j"})
	parts := strings.Split(tok, ".")
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	hs := string(headerJSON)
	if !strings.Contains(hs, `"alg":"EdDSA"`) {
		t.Errorf("header missing EdDSA: %s", hs)
	}
	if !strings.Contains(hs, `"typ":"JWT"`) {
		t.Errorf("header missing JWT typ: %s", hs)
	}
}

func TestSign_SignatureValid(t *testing.T) {
	is := newTestIssuer(t)
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "a", JTI: "j"})
	parts := strings.Split(tok, ".")
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	signingInput := parts[0] + "." + parts[1]
	if !ed25519.Verify(is.PublicKey(), []byte(signingInput), sig) {
		t.Error("signature does not verify against domain pub key")
	}
}

// =============================================================================
// ParseUnverified
// =============================================================================

func TestParseUnverified_Malformed(t *testing.T) {
	_, err := ParseUnverified("not-jwt")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v, want ErrTokenMalformed", err)
	}
}

func TestParseUnverified_BadBase64Header(t *testing.T) {
	_, err := ParseUnverified("!!!.eyJh.x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v, want ErrTokenMalformed", err)
	}
}

func TestParseUnverified_BadBase64Claims(t *testing.T) {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	_, err := ParseUnverified(hdr + ".!!!.x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestParseUnverified_BadJSONHeader(t *testing.T) {
	hdr := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	body := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
	_, err := ParseUnverified(hdr + "." + body + ".x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestParseUnverified_BadJSONClaims(t *testing.T) {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	_, err := ParseUnverified(hdr + "." + body + ".x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestParseUnverified_EmptyKid(t *testing.T) {
	is, _ := NewIssuer(IssuerConfig{DomainKey: newTestKey(t)})
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "a", JTI: "j"})
	c, _ := ParseUnverified(tok)
	if c.Kid != "" {
		t.Errorf("Kid = %q, want empty", c.Kid)
	}
}
