package a2a

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Resolver
// =============================================================================

func TestStaticResolver_OK(t *testing.T) {
	is := newTestIssuer(t)
	r := &StaticKeyResolver{PublicKey: is.PublicKey()}
	pub, err := r.Resolve("any-kid")
	if err != nil {
		t.Fatal(err)
	}
	if string(pub) != string(is.PublicKey()) {
		t.Error("key mismatch")
	}
}

func TestStaticResolver_Empty(t *testing.T) {
	r := &StaticKeyResolver{}
	_, err := r.Resolve("any")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestMapResolver_HitByKid(t *testing.T) {
	is := newTestIssuer(t)
	r := &MapKeyResolver{
		Keys: map[string]ed25519.PublicKey{
			"k1": is.PublicKey(),
		},
	}
	pub, err := r.Resolve("k1")
	if err != nil {
		t.Fatal(err)
	}
	if string(pub) != string(is.PublicKey()) {
		t.Error("mismatch")
	}
}

func TestMapResolver_FallbackToDefault(t *testing.T) {
	is := newTestIssuer(t)
	r := &MapKeyResolver{
		Keys:    map[string]ed25519.PublicKey{},
		Default: is.PublicKey(),
	}
	pub, err := r.Resolve("unknown")
	if err != nil {
		t.Fatal(err)
	}
	if string(pub) != string(is.PublicKey()) {
		t.Error("mismatch")
	}
}

func TestMapResolver_NotFound(t *testing.T) {
	r := &MapKeyResolver{Keys: map[string]ed25519.PublicKey{}}
	_, err := r.Resolve("none")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("err = %v", err)
	}
}

// =============================================================================
// NewVerifier
// =============================================================================

func newTestVerifier(t *testing.T, is *Issuer) *Verifier {
	t.Helper()
	v, err := NewVerifier(VerifierConfig{
		Resolver:         &StaticKeyResolver{PublicKey: is.PublicKey()},
		ExpectedIssuer:   is.Issuer(),
		ExpectedAudience: "did:agent:target",
	})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNewVerifier_NilResolver(t *testing.T) {
	_, err := NewVerifier(VerifierConfig{
		ExpectedIssuer:   "i",
		ExpectedAudience: "a",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewVerifier_EmptyIssuer(t *testing.T) {
	_, err := NewVerifier(VerifierConfig{
		Resolver:         &StaticKeyResolver{},
		ExpectedAudience: "a",
	})
	if !errors.Is(err, ErrClaimInvalid) {
		t.Errorf("err = %v", err)
	}
}

func TestNewVerifier_EmptyAudience(t *testing.T) {
	_, err := NewVerifier(VerifierConfig{
		Resolver:       &StaticKeyResolver{},
		ExpectedIssuer: "i",
	})
	if !errors.Is(err, ErrClaimInvalid) {
		t.Errorf("err = %v", err)
	}
}

func TestNewVerifier_Defaults(t *testing.T) {
	v, err := NewVerifier(VerifierConfig{
		Resolver:         &StaticKeyResolver{},
		ExpectedIssuer:   "i",
		ExpectedAudience: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.cfg.Clock == nil {
		t.Error("clock should default")
	}
	if v.cfg.LeewaySeconds != 5 {
		t.Errorf("leeway = %d", v.cfg.LeewaySeconds)
	}
}

// =============================================================================
// Verify happy path
// =============================================================================

func TestVerify_HappyPath(t *testing.T) {
	is := newTestIssuer(t)
	v := newTestVerifier(t, is)
	tok, err := is.Sign(SignInput{
		Subject:    "agent-001",
		Audience:   "did:agent:target",
		JTI:        "j-001",
		TTL:        time.Minute,
		Scope:      "read:tags write:logs",
		TrustLevel: 80,
		AuditID:    "audit-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if c.Subject != "agent-001" {
		t.Errorf("Subject = %q", c.Subject)
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

// =============================================================================
// Verify 失败路径
// =============================================================================

func TestVerify_Malformed_Parts(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	_, err := v.Verify("a.b")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_Malformed_HeaderB64(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	_, err := v.Verify("!!!.eyJ.x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_Malformed_HeaderJSON(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	body := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
	_, err := v.Verify(h + "." + body + ".x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_BadAlg_None(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	c := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"test-issuer","aud":"did:agent:target","sub":"s","jti":"j","exp":9999999999}`))
	_, err := v.Verify(h + "." + c + ".x")
	if !errors.Is(err, ErrUnsupportedAlg) {
		t.Errorf("err = %v, want ErrUnsupportedAlg", err)
	}
}

func TestVerify_BadAlg_HS256(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	c := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"test-issuer","aud":"did:agent:target","sub":"s","jti":"j","exp":9999999999}`))
	_, err := v.Verify(h + "." + c + ".x")
	if !errors.Is(err, ErrUnsupportedAlg) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_BadTyp(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"NotJWT"}`))
	c := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
	_, err := v.Verify(h + "." + c + ".x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_Malformed_ClaimsB64(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	_, err := v.Verify(h + ".!!!.x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_Malformed_ClaimsJSON(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	c := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	_, err := v.Verify(h + "." + c + ".x")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_EmptySubject(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	c := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"test-issuer","aud":"did:agent:target","jti":"j","exp":9999999999}`))
	_, err := v.Verify(h + "." + c + ".x")
	if !errors.Is(err, ErrClaimInvalid) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_EmptyJTI(t *testing.T) {
	v := newTestVerifier(t, newTestIssuer(t))
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	c := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"test-issuer","aud":"did:agent:target","sub":"s","exp":9999999999}`))
	_, err := v.Verify(h + "." + c + ".x")
	if !errors.Is(err, ErrClaimInvalid) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_IssuerMismatch(t *testing.T) {
	is := newTestIssuer(t)
	v := newTestVerifier(t, is)
	// 用一个不同 issuer 的 Issuer 颁发
	other, _ := NewIssuer(IssuerConfig{
		DomainKey: is.cfg.DomainKey,
		Issuer:    "other-issuer",
	})
	tok, _ := other.Sign(SignInput{Subject: "s", Audience: "did:agent:target", JTI: "j"})
	_, err := v.Verify(tok)
	if !errors.Is(err, ErrIssuerMismatch) {
		t.Errorf("err = %v, want ErrIssuerMismatch", err)
	}
}

func TestVerify_AudienceMismatch(t *testing.T) {
	is := newTestIssuer(t)
	v := newTestVerifier(t, is)
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "did:agent:other", JTI: "j"})
	_, err := v.Verify(tok)
	if !errors.Is(err, ErrAudienceMismatch) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	is := newTestIssuer(t)
	// 过去时间签名
	past := time.Now().Add(-1 * time.Hour)
	is.cfg.Clock = func() time.Time { return past }
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "did:agent:target", JTI: "j", TTL: time.Minute})
	is.cfg.Clock = time.Now
	v := newTestVerifier(t, is)
	_, err := v.Verify(tok)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_LeewayAllowsRecentExp(t *testing.T) {
	is := newTestIssuer(t)
	v, _ := NewVerifier(VerifierConfig{
		Resolver:         &StaticKeyResolver{PublicKey: is.PublicKey()},
		ExpectedIssuer:   is.Issuer(),
		ExpectedAudience: "did:agent:target",
		LeewaySeconds:    300, // 5 分钟容忍
	})
	// 1 分钟前过期，但 5 分钟 leeway 内
	past := time.Now().Add(-2 * time.Minute)
	is.cfg.Clock = func() time.Time { return past }
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "did:agent:target", JTI: "j", TTL: time.Minute})
	is.cfg.Clock = time.Now
	if _, err := v.Verify(tok); err != nil {
		t.Errorf("expected leeway to allow: %v", err)
	}
}

func TestVerify_BadTrustLevel(t *testing.T) {
	is := newTestIssuer(t)
	v := newTestVerifier(t, is)
	// 手动构造带非法 trust_level 的 claims
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT","kid":"test-key-1"}`))
	cs := jwtClaims{
		Issuer: "test-issuer", Audience: "did:agent:target",
		Subject: "s", JTI: "j",
		IssuedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		TrustLevel: 200,
	}
	cb, _ := json.Marshal(cs)
	c := base64.RawURLEncoding.EncodeToString(cb)
	sig := ed25519.Sign(is.cfg.DomainKey, []byte(h+"."+c))
	tok := h + "." + c + "." + base64.RawURLEncoding.EncodeToString(sig)
	_, err := v.Verify(tok)
	if !errors.Is(err, ErrInvalidTrustLevel) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_KeyNotFound(t *testing.T) {
	is := newTestIssuer(t)
	v, _ := NewVerifier(VerifierConfig{
		Resolver:         &MapKeyResolver{Keys: map[string]ed25519.PublicKey{}},
		ExpectedIssuer:   is.Issuer(),
		ExpectedAudience: "did:agent:target",
	})
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "did:agent:target", JTI: "j"})
	_, err := v.Verify(tok)
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	is := newTestIssuer(t)
	v := newTestVerifier(t, is)
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "did:agent:target", JTI: "j"})
	// 破坏 sig
	parts := strings.Split(tok, ".")
	parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	_, err := v.Verify(strings.Join(parts, "."))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_BadSignatureB64(t *testing.T) {
	is := newTestIssuer(t)
	v := newTestVerifier(t, is)
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "did:agent:target", JTI: "j"})
	parts := strings.Split(tok, ".")
	_, err := v.Verify(parts[0] + "." + parts[1] + ".!!!")
	if !errors.Is(err, ErrTokenMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestVerify_DifferentDomainKey(t *testing.T) {
	is := newTestIssuer(t)
	other := newTestIssuer(t) // 不同 key
	v, _ := NewVerifier(VerifierConfig{
		Resolver:         &StaticKeyResolver{PublicKey: other.PublicKey()},
		ExpectedIssuer:   is.Issuer(),
		ExpectedAudience: "did:agent:target",
	})
	tok, _ := is.Sign(SignInput{Subject: "s", Audience: "did:agent:target", JTI: "j"})
	_, err := v.Verify(tok)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v", err)
	}
}

// =============================================================================
// HasScope
// =============================================================================

func TestHasScope(t *testing.T) {
	c := &TokenClaims{Scope: "read:tags write:logs admin"}
	if !c.HasScope("read:tags") {
		t.Error("should have read:tags")
	}
	if !c.HasScope("write:logs") {
		t.Error("should have write:logs")
	}
	if c.HasScope("delete:tags") {
		t.Error("should not have delete:tags")
	}
}

func TestHasScope_EmptyClaims(t *testing.T) {
	if (&TokenClaims{}).HasScope("x") {
		t.Error("empty scope")
	}
}

func TestHasScope_NilReceiver(t *testing.T) {
	var c *TokenClaims
	if c.HasScope("x") {
		t.Error("nil receiver")
	}
}

func TestHasScope_EmptyScopeArg(t *testing.T) {
	c := &TokenClaims{Scope: "a b"}
	if c.HasScope("") {
		t.Error("empty scope arg")
	}
}
