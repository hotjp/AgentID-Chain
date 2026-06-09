package aap

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cache"
	"github.com/alicebob/miniredis/v2"
)

func newTestKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func newTestStore(t *testing.T) cache.Cache {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	return cache.NewMiniredis(mr)
}

func newTestGenerator(t *testing.T) *Generator {
	t.Helper()
	g, err := NewGenerator(newTestStore(t), Config{
		DomainKey:    newTestKey(t),
		ChallengeTTL: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestNewGenerator_OK(t *testing.T) {
	g, err := NewGenerator(newTestStore(t), Config{DomainKey: newTestKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if g.TTL() != 30*time.Second {
		t.Errorf("default TTL = %v, want 30s", g.TTL())
	}
}

func TestNewGenerator_NilStore(t *testing.T) {
	_, err := NewGenerator(nil, Config{DomainKey: newTestKey(t)})
	if !errors.Is(err, ErrStoreUnavailable) {
		t.Errorf("err = %v, want ErrStoreUnavailable", err)
	}
}

func TestNewGenerator_EmptyDomainKey(t *testing.T) {
	_, err := NewGenerator(newTestStore(t), Config{})
	if !errors.Is(err, ErrEmptyDomain) {
		t.Errorf("err = %v, want ErrEmptyDomain", err)
	}
}

func TestNewGenerator_DefaultTTL(t *testing.T) {
	g, err := NewGenerator(newTestStore(t), Config{DomainKey: newTestKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if g.TTL() != 30*time.Second {
		t.Errorf("default TTL = %v", g.TTL())
	}
}

func TestNewGenerator_CustomClock(t *testing.T) {
	fixed := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	g, err := NewGenerator(newTestStore(t), Config{
		DomainKey: newTestKey(t),
		Clock:     func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !c.IssuedAt.Equal(fixed) {
		t.Errorf("IssuedAt = %v, want %v", c.IssuedAt, fixed)
	}
}

func TestGenerate_HappyPath(t *testing.T) {
	g := newTestGenerator(t)
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 字段校验
	if c.ChallengeID == "" {
		t.Error("empty challenge_id")
	}
	if len(c.Nonce) != 32 {
		t.Errorf("nonce length = %d, want 32", len(c.Nonce))
	}
	if c.DomainSig == "" {
		t.Error("empty domain_sig")
	}
	if c.ExpiresAt.Before(c.IssuedAt) {
		t.Error("expires_at before issued_at")
	}
	// Nonce 必须能 hex decode
	if _, err := hex.DecodeString(c.Nonce); err != nil {
		t.Errorf("nonce not hex: %v", err)
	}
	// Domain sig 必须能 base64 decode
	if _, err := base64.RawURLEncoding.DecodeString(c.DomainSig); err != nil {
		t.Errorf("domain_sig not base64: %v", err)
	}
}

func TestGenerate_TTLExpiry(t *testing.T) {
	g := newTestGenerator(t)
	now := time.Now()
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// ExpiresAt - IssuedAt = TTL
	delta := c.ExpiresAt.Sub(c.IssuedAt)
	if delta != 30*time.Second {
		t.Errorf("delta = %v, want 30s", delta)
	}
	if c.IsExpired(now) {
		t.Error("should not be expired immediately")
	}
	if !c.IsExpired(now.Add(31 * time.Second)) {
		t.Error("should be expired after 31s")
	}
}

func TestGenerate_UniqueIDs(t *testing.T) {
	g := newTestGenerator(t)
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		c, err := g.Generate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if seen[c.ChallengeID] {
			t.Fatalf("duplicate challenge_id: %s", c.ChallengeID)
		}
		seen[c.ChallengeID] = true
	}
}

func TestGenerate_UniqueNonces(t *testing.T) {
	g := newTestGenerator(t)
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		c, err := g.Generate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if seen[c.Nonce] {
			t.Fatalf("duplicate nonce: %s", c.Nonce)
		}
		seen[c.Nonce] = true
	}
}

func TestGenerate_PersistsInStore(t *testing.T) {
	g := newTestGenerator(t)
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := g.LoadChallenge(context.Background(), c.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Nonce != c.Nonce {
		t.Errorf("loaded.Nonce = %s, want %s", loaded.Nonce, c.Nonce)
	}
	if loaded.DomainSig != c.DomainSig {
		t.Errorf("loaded.DomainSig = %s, want %s", loaded.DomainSig, c.DomainSig)
	}
}

func TestLoadChallenge_NotExist(t *testing.T) {
	g := newTestGenerator(t)
	_, err := g.LoadChallenge(context.Background(), "deadbeef")
	if !errors.Is(err, cache.ErrMiss) {
		t.Errorf("err = %v, want ErrMiss", err)
	}
}

func TestConsumeChallenge_DeletesAfterRead(t *testing.T) {
	g := newTestGenerator(t)
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 第一次 consume 成功
	consumed, err := g.ConsumeChallenge(context.Background(), c.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	if consumed.Nonce != c.Nonce {
		t.Error("nonce mismatch")
	}
	// 第二次 consume 应该 miss
	if _, err := g.ConsumeChallenge(context.Background(), c.ChallengeID); !errors.Is(err, cache.ErrMiss) {
		t.Errorf("err = %v, want ErrMiss", err)
	}
}

func TestGenerateWithID_OK(t *testing.T) {
	g := newTestGenerator(t)
	id := "0123456789abcdef0123456789abcdef"
	c, err := g.GenerateWithID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if c.ChallengeID != id {
		t.Errorf("ChallengeID = %s, want %s", c.ChallengeID, id)
	}
}

func TestGenerateWithID_DuplicateID(t *testing.T) {
	g := newTestGenerator(t)
	id := "0123456789abcdef0123456789abcdef"
	if _, err := g.GenerateWithID(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	_, err := g.GenerateWithID(context.Background(), id)
	if !errors.Is(err, ErrInvalidChallenge) {
		t.Errorf("err = %v, want ErrInvalidChallenge", err)
	}
}

func TestGenerateWithID_InvalidID(t *testing.T) {
	g := newTestGenerator(t)
	cases := []string{
		"",
		"not-hex",
		"GG",
		"abc", // odd length
	}
	for _, id := range cases {
		_, err := g.GenerateWithID(context.Background(), id)
		if !errors.Is(err, ErrInvalidChallengeID) {
			t.Errorf("id=%q: err = %v, want ErrInvalidChallengeID", id, err)
		}
	}
}

func TestIsValidChallengeID(t *testing.T) {
	cases := []struct {
		in  string
		ok  bool
	}{
		{"abcd", true},
		{"0123456789abcdef", true},
		{"ABCDEF", true},
		{"ABCDEF0123456789", true},
		{"", false},
		{"abc", false}, // odd length
		{"xyz", false}, // not hex
		{"ab cd", false}, // space
		{string(make([]byte, 70)), false}, // too long (>64)
	}
	for _, c := range cases {
		got := isValidChallengeID(c.in)
		if got != c.ok {
			t.Errorf("isValidChallengeID(%q) = %v, want %v", c.in, got, c.ok)
		}
	}
}

func TestSignPayload_Deterministic(t *testing.T) {
	id := "abcd1234"
	nonce := "0xff"
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	p1 := signPayload(id, nonce, now)
	p2 := signPayload(id, nonce, now)
	if string(p1) != string(p2) {
		t.Error("signPayload not deterministic")
	}
	// 包含必要字段
	s := string(p1)
	if !contains(s, id) || !contains(s, nonce) {
		t.Error("payload missing id/nonce")
	}
}

func TestDomainSig_VerifiesWithPublicKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewGenerator(newTestStore(t), Config{DomainKey: priv})
	if err != nil {
		t.Fatal(err)
	}
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(c.DomainSig)
	if err != nil {
		t.Fatal(err)
	}
	payload := signPayload(c.ChallengeID, c.Nonce, c.IssuedAt)
	if !ed25519.Verify(pub, payload, sig) {
		t.Error("domain_sig failed verification")
	}
}

func TestEncodeDecodeChallenge_RoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	c := &Challenge{
		ChallengeID: "abcd1234",
		Nonce:       "00112233445566778899aabbccddeeff",
		IssuedAt:    now,
		ExpiresAt:   now.Add(30 * time.Second),
		DomainSig:   "fake-sig",
	}
	data, err := encodeChallenge(c)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := decodeChallenge(data)
	if err != nil {
		t.Fatal(err)
	}
	if c2.ChallengeID != c.ChallengeID {
		t.Errorf("ChallengeID = %s", c2.ChallengeID)
	}
	if c2.Nonce != c.Nonce {
		t.Errorf("Nonce = %s", c2.Nonce)
	}
	if c2.DomainSig != c.DomainSig {
		t.Errorf("DomainSig = %s", c2.DomainSig)
	}
	if !c2.IssuedAt.Equal(c.IssuedAt) {
		t.Errorf("IssuedAt = %v", c2.IssuedAt)
	}
	if !c2.ExpiresAt.Equal(c.ExpiresAt) {
		t.Errorf("ExpiresAt = %v", c2.ExpiresAt)
	}
}

func TestDecodeChallenge_Malformed(t *testing.T) {
	_, err := decodeChallenge([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for malformed data")
	}
}

func TestRandomBytes(t *testing.T) {
	a, _ := randomBytes(16)
	b, _ := randomBytes(16)
	if len(a) != 16 {
		t.Errorf("len(a) = %d, want 16", len(a))
	}
	if string(a) == string(b) {
		t.Error("random bytes should differ")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
