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

)

func newTestVerifier(t *testing.T, gen *Generator) *Verifier {
	t.Helper()
	v, err := NewVerifier(gen, VerifierConfig{})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func newTestVerifierWithClock(t *testing.T, gen *Generator, now time.Time) *Verifier {
	t.Helper()
	v, err := NewVerifier(gen, VerifierConfig{Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return v
}

// signResponse 用 client 私钥对 payload 签名，返回 base64。
func signResponse(t *testing.T, priv ed25519.PrivateKey, payload []byte) string {
	t.Helper()
	sig := ed25519.Sign(priv, payload)
	return base64.RawURLEncoding.EncodeToString(sig)
}

func TestNewVerifier_NilGenerator(t *testing.T) {
	_, err := NewVerifier(nil, VerifierConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewVerifier_DefaultResponseMaxTTL(t *testing.T) {
	g := newTestGenerator(t)
	v, err := NewVerifier(g, VerifierConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if v.cfg.ResponseMaxTTL != 10*time.Minute {
		t.Errorf("default ResponseMaxTTL = %v", v.cfg.ResponseMaxTTL)
	}
}

func TestVerify_HappyPath(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)

	// 准备 client 密钥对
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	pubHex := hex.EncodeToString(pub)

	// 生成 challenge
	c, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 客户端签名
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, "01234567-89ab-cdef-0123-456789abcdef")
	resp := signResponse(t, priv, payload)

	out, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Response:    resp,
		AgentPubKey: pubHex,
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if out.AgentUUID != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Errorf("AgentUUID mismatch")
	}
	if !ed25519.Verify(pub, payload, mustDecode(t, resp)) {
		t.Error("public key mismatch")
	}
}

func TestVerify_EmptyChallengeID(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.Verify(context.Background(), VerifyInput{
		AgentPubKey: "abcd",
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
	})
	if !errors.Is(err, ErrInvalidChallengeID) {
		t.Errorf("err = %v, want ErrInvalidChallengeID", err)
	}
}

func TestVerify_EmptyAgentUUID(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "abcd1234",
		AgentPubKey: "abcd",
	})
	if !errors.Is(err, ErrEmptyAgentUUID) {
		t.Errorf("err = %v, want ErrEmptyAgentUUID", err)
	}
}

func TestVerify_EmptyAgentPubKey(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "abcd1234",
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
	})
	if !errors.Is(err, ErrEmptyAgentPubKey) {
		t.Errorf("err = %v, want ErrEmptyAgentPubKey", err)
	}
}

func TestVerify_EmptyResponse(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "abcd1234",
		AgentPubKey: hex.EncodeToString(make([]byte, 32)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
	})
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerify_InvalidAgentPubKey(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "abcd1234",
		Response:    "sig",
		AgentPubKey: "not-hex-or-base64-zzzz",
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
	})
	if !errors.Is(err, ErrInvalidAgentPubKey) {
		t.Errorf("err = %v, want ErrInvalidAgentPubKey", err)
	}
}

func TestVerify_InvalidPubKeyLength(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	// 16 字节 — 不是 32
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "abcd1234",
		Response:    "sig",
		AgentPubKey: hex.EncodeToString(make([]byte, 16)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
	})
	if !errors.Is(err, ErrInvalidAgentPubKey) {
		t.Errorf("err = %v, want ErrInvalidAgentPubKey", err)
	}
}

func TestVerify_InvalidAgentUUID(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "abcd1234",
		Response:    "sig",
		AgentPubKey: hex.EncodeToString(make([]byte, 32)),
		AgentUUID:   "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVerify_ChallengeNotExist(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: "deadbeef00000000deadbeef00000000",
		Response:    "sig",
		AgentPubKey: hex.EncodeToString(make([]byte, 32)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
	})
	if !errors.Is(err, ErrInvalidChallenge) {
		t.Errorf("err = %v, want ErrInvalidChallenge", err)
	}
}

func TestVerify_ChallengeConsumed(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, _ := g.Generate(context.Background())
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, "01234567-89ab-cdef-0123-456789abcdef")
	resp := signResponse(t, priv, payload)

	in := VerifyInput{
		ChallengeID: c.ChallengeID,
		Response:    resp,
		AgentPubKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         time.Now(),
	}
	// 第一次成功
	if _, err := v.Verify(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	// 第二次 — challenge 已被消费
	if _, err := v.Verify(context.Background(), in); !errors.Is(err, ErrInvalidChallenge) {
		t.Errorf("err = %v, want ErrInvalidChallenge", err)
	}
}

func TestVerify_ResponseExpired(t *testing.T) {
	// 共享同一个 store
	store := newTestStore(t)
	domainKey := newTestKey(t)
	now := time.Now()

	// Generator 1：颁发 15 分钟前的 challenge（用同一 store）
	pastGen, err := NewGenerator(store, Config{
		DomainKey:    domainKey,
		ChallengeTTL: time.Hour,
		Clock:        func() time.Time { return now.Add(-15 * time.Minute) },
	})
	if err != nil {
		t.Fatal(err)
	}
	// Generator 2（verifier 内部使用）：时间用 real now
	nowGen, err := NewGenerator(store, Config{
		DomainKey:    domainKey,
		ChallengeTTL: time.Hour,
		Clock:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	v := newTestVerifierWithClock(t, nowGen, now)

	c, _ := pastGen.Generate(context.Background())
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, "01234567-89ab-cdef-0123-456789abcdef")
	resp := signResponse(t, priv, payload)

	_, err = v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Response:    resp,
		AgentPubKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         now,
	})
	if !errors.Is(err, ErrResponseExpired) {
		t.Errorf("err = %v, want ErrResponseExpired", err)
	}
}

func TestVerify_SignatureInvalid(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)

	c, _ := g.Generate(context.Background())
	// 用 wrong priv 签
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, "01234567-89ab-cdef-0123-456789abcdef")
	resp := signResponse(t, wrongPriv, payload)

	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Response:    resp,
		AgentPubKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         time.Now(),
	})
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerify_BadResponseEncoding(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, _ := g.Generate(context.Background())
	_, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Response:    "not-base64!!!",
		AgentPubKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         time.Now(),
	})
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v, want ErrSignatureInvalid", err)
	}
}

func TestIssueProof_HappyPath(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, _ := g.Generate(context.Background())
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, "01234567-89ab-cdef-0123-456789abcdef")
	resp := signResponse(t, priv, payload)

	out, err := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID,
		Response:    resp,
		AgentPubKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := v.IssueProof(out, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if proof.ProofID == "" {
		t.Error("empty proof id")
	}
	if proof.AgentUUID != out.AgentUUID {
		t.Errorf("AgentUUID = %s", proof.AgentUUID)
	}
	if proof.ExpiresAt.Before(proof.IssuedAt) {
		t.Error("expires before issued")
	}
	if proof.DomainSig == "" {
		t.Error("empty domain sig")
	}
}

func TestIssueProof_NilOutput(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, err := v.IssueProof(nil, time.Minute)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIssueProof_DefaultTTL(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, _ := g.Generate(context.Background())
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, "01234567-89ab-cdef-0123-456789abcdef")
	resp := signResponse(t, priv, payload)
	out, _ := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID, Response: resp,
		AgentPubKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         time.Now(),
	})
	proof, err := v.IssueProof(out, 0) // TTL=0 → 5min default
	if err != nil {
		t.Fatal(err)
	}
	delta := proof.ExpiresAt.Sub(proof.IssuedAt)
	if delta != 5*time.Minute {
		t.Errorf("default TTL delta = %v, want 5m", delta)
	}
}

func TestVerifyProof_RoundTrip(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, _ := g.Generate(context.Background())
	payload := responsePayload(c.ChallengeID, c.Nonce, c.IssuedAt, "01234567-89ab-cdef-0123-456789abcdef")
	resp := signResponse(t, priv, payload)
	out, _ := v.Verify(context.Background(), VerifyInput{
		ChallengeID: c.ChallengeID, Response: resp,
		AgentPubKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		Now:         time.Now(),
	})
	proof, _ := v.IssueProof(out, time.Minute)
	if err := v.VerifyProof(proof); err != nil {
		t.Errorf("VerifyProof: %v", err)
	}
}

func TestVerifyProof_Nil(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	if err := v.VerifyProof(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyProof_Expired(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	now := time.Now()
	proof := &Proof{
		ProofID:     "abcd1234",
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: hex.EncodeToString(make([]byte, 32)),
		IssuedAt:    now.Add(-time.Hour),
		ExpiresAt:   now.Add(-time.Minute), // 已过期
		DomainSig:   "sig",
	}
	if err := v.VerifyProof(proof); !errors.Is(err, ErrResponseExpired) {
		t.Errorf("err = %v, want ErrResponseExpired", err)
	}
}

func TestVerifyProof_BadPubKey(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	// 用真实 domain key 签一个有效 sig，跳过 sig 校验到达 pubkey 校验
	now := time.Now()
	issuedAt := now
	expiresAt := now.Add(time.Hour)
	payload := proofPayload("abcd1234", "01234567-89ab-cdef-0123-456789abcdef", issuedAt, expiresAt)
	validSig := ed25519.Sign(g.cfg.DomainKey, payload)
	proof := &Proof{
		ProofID:     "abcd1234",
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: "not-hex",
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
		DomainSig:   base64.RawURLEncoding.EncodeToString(validSig),
	}
	if err := v.VerifyProof(proof); !errors.Is(err, ErrInvalidAgentPubKey) {
		t.Errorf("err = %v, want ErrInvalidAgentPubKey", err)
	}
}

func TestVerifyProof_BadSigEncoding(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	proof := &Proof{
		ProofID:     "abcd1234",
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: hex.EncodeToString(make([]byte, 32)),
		IssuedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
		DomainSig:   "not-base64!!!",
	}
	if err := v.VerifyProof(proof); !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifyProof_BadSig(t *testing.T) {
	g := newTestGenerator(t)
	v := newTestVerifier(t, g)
	now := time.Now()
	proof := &Proof{
		ProofID:     "abcd1234",
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: hex.EncodeToString(make([]byte, 32)),
		IssuedAt:    now,
		ExpiresAt:   now.Add(time.Hour),
		DomainSig:   base64.RawURLEncoding.EncodeToString(make([]byte, 64)),
	}
	if err := v.VerifyProof(proof); !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v, want ErrSignatureInvalid", err)
	}
}

func TestProof_IsExpired(t *testing.T) {
	now := time.Now()
	p := &Proof{IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	if p.IsExpired(now) {
		t.Error("should not be expired now")
	}
	if !p.IsExpired(now.Add(2 * time.Hour)) {
		t.Error("should be expired after ExpiresAt")
	}
}

func TestParsePubKey_Hex(t *testing.T) {
	pub, err := parsePubKey(hex.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 32 {
		t.Errorf("len = %d", len(pub))
	}
}

func TestParsePubKey_HexWith0x(t *testing.T) {
	pub, err := parsePubKey("0x" + hex.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 32 {
		t.Errorf("len = %d", len(pub))
	}
}

func TestParsePubKey_Base64RawURL(t *testing.T) {
	pub, err := parsePubKey(base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 32 {
		t.Errorf("len = %d", len(pub))
	}
}

func TestParsePubKey_Base64Std(t *testing.T) {
	pub, err := parsePubKey(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 32 {
		t.Errorf("len = %d", len(pub))
	}
}

func TestParsePubKey_Empty(t *testing.T) {
	_, err := parsePubKey("")
	if !errors.Is(err, ErrEmptyAgentPubKey) {
		t.Errorf("err = %v, want ErrEmptyAgentPubKey", err)
	}
}

func TestParsePubKey_BadHex(t *testing.T) {
	_, err := parsePubKey("nothex")
	if !errors.Is(err, ErrInvalidAgentPubKey) {
		t.Errorf("err = %v, want ErrInvalidAgentPubKey", err)
	}
}

func TestParsePubKey_BadBase64(t *testing.T) {
	_, err := parsePubKey("!!notbase64!!")
	if !errors.Is(err, ErrInvalidAgentPubKey) {
		t.Errorf("err = %v, want ErrInvalidAgentPubKey", err)
	}
}

func TestParsePubKey_Trimmed(t *testing.T) {
	pub, err := parsePubKey("  " + hex.EncodeToString(make([]byte, 32)) + "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != 32 {
		t.Errorf("len = %d", len(pub))
	}
}

func mustDecode(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
