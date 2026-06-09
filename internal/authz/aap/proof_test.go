package aap

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestSigner(t *testing.T) *ProofSigner {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewProofSigner(priv, "agentid-chain")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewProofSigner_OK(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, err := NewProofSigner(priv, "test-issuer")
	if err != nil {
		t.Fatal(err)
	}
	if s.Issuer() != "test-issuer" {
		t.Errorf("Issuer = %s", s.Issuer())
	}
}

func TestNewProofSigner_DefaultIssuer(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, err := NewProofSigner(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Issuer() != "agentid-chain" {
		t.Errorf("default Issuer = %s", s.Issuer())
	}
}

func TestNewProofSigner_BadKey(t *testing.T) {
	_, err := NewProofSigner(ed25519.PrivateKey(make([]byte, 10)), "test")
	if !errors.Is(err, ErrEmptyDomain) {
		t.Errorf("err = %v, want ErrEmptyDomain", err)
	}
}

func TestSign_HappyPath(t *testing.T) {
	s := newTestSigner(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	tok, err := s.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "test-jti-001",
		TTL:         5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d, want 3", len(parts))
	}
}

func TestSign_DefaultTTL(t *testing.T) {
	s := newTestSigner(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	tok, err := s.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "test-jti-001",
		TTL:         0, // default
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	delta := view.ExpiresAt.Sub(view.IssuedAt)
	if delta != 5*time.Minute {
		t.Errorf("default TTL delta = %v, want 5m", delta)
	}
}

func TestSign_EmptyAgentUUID(t *testing.T) {
	s := newTestSigner(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := s.Sign(SignInput{
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "test",
	})
	if !errors.Is(err, ErrEmptyAgentUUID) {
		t.Errorf("err = %v, want ErrEmptyAgentUUID", err)
	}
}

func TestSign_InvalidAgentUUID(t *testing.T) {
	s := newTestSigner(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := s.Sign(SignInput{
		AgentUUID:   "not-a-uuid",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "test",
	})
	if !errors.Is(err, ErrEmptyAgentUUID) {
		t.Errorf("err = %v, want ErrEmptyAgentUUID", err)
	}
}

func TestSign_EmptyJTI(t *testing.T) {
	s := newTestSigner(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := s.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "",
	})
	if !errors.Is(err, ErrProofClaimInvalid) {
		t.Errorf("err = %v, want ErrProofClaimInvalid", err)
	}
}

func TestVerify_RoundTrip(t *testing.T) {
	s := newTestSigner(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)
	tok, err := s.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: pub,
		JTI:         "test-jti-001",
		TTL:         5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if view.AgentUUID != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Errorf("AgentUUID = %s", view.AgentUUID)
	}
	if view.JTI != "test-jti-001" {
		t.Errorf("JTI = %s", view.JTI)
	}
	if !ed25519.Verify(pub, []byte{}, []byte{}) {
		// 仅仅作为对比：公钥一致
	}
	if !view.AgentPubKey.Equal(pub) {
		t.Error("AgentPubKey mismatch")
	}
}

func TestVerify_Expired(t *testing.T) {
	s := newTestSigner(t)
	fixed := time.Now()
	s.SetClock(func() time.Time { return fixed.Add(-1 * time.Hour) })
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	tok, err := s.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "j",
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 现在切换到真实时间，token 已过期
	s.SetClock(time.Now)
	_, err = s.Verify(tok)
	if !errors.Is(err, ErrResponseExpired) {
		t.Errorf("err = %v, want ErrResponseExpired", err)
	}
}

func TestVerify_Malformed_PartCount(t *testing.T) {
	s := newTestSigner(t)
	for _, bad := range []string{"", "a", "a.b", "a.b.c.d"} {
		_, err := s.Verify(bad)
		if !errors.Is(err, ErrProofMalformed) {
			t.Errorf("Verify(%q) err = %v, want ErrProofMalformed", bad, err)
		}
	}
}

func TestVerify_Malformed_Base64Header(t *testing.T) {
	s := newTestSigner(t)
	_, err := s.Verify("not-base64!@#.claim.sig")
	if !errors.Is(err, ErrProofMalformed) {
		t.Errorf("err = %v, want ErrProofMalformed", err)
	}
}

func TestVerify_BadAlg(t *testing.T) {
	s := newTestSigner(t)
	// 构造 alg=none 的 token
	header := proofHeader{Alg: "none", Typ: "JWT"}
	headerJSON, _ := json.Marshal(header)
	h64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claims := proofClaims{
		Issuer:    "agentid-chain",
		Subject:   "01234567-89ab-cdef-0123-456789abcdef",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		PublicKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
	claimsJSON, _ := json.Marshal(claims)
	c64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	tok := h64 + "." + c64 + ".sig"
	_, err := s.Verify(tok)
	if !errors.Is(err, ErrProofUnsupportedAlg) {
		t.Errorf("err = %v, want ErrProofUnsupportedAlg", err)
	}
}

func TestVerify_BadTyp(t *testing.T) {
	s := newTestSigner(t)
	header := proofHeader{Alg: "EdDSA", Typ: "JWE"}
	hJSON, _ := json.Marshal(header)
	h64 := base64.RawURLEncoding.EncodeToString(hJSON)
	claims := proofClaims{
		Issuer:    "agentid-chain",
		Subject:   "01234567-89ab-cdef-0123-456789abcdef",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		PublicKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
	cJSON, _ := json.Marshal(claims)
	c64 := base64.RawURLEncoding.EncodeToString(cJSON)
	tok := h64 + "." + c64 + ".sig"
	_, err := s.Verify(tok)
	if !errors.Is(err, ErrProofMalformed) {
		t.Errorf("err = %v, want ErrProofMalformed", err)
	}
}

func TestVerify_IssuerMismatch(t *testing.T) {
	_, priv1, _ := ed25519.GenerateKey(rand.Reader)
	_, priv2, _ := ed25519.GenerateKey(rand.Reader)
	s1, _ := NewProofSigner(priv1, "issuer-A")
	s2, _ := NewProofSigner(priv2, "issuer-B")
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	tok, err := s1.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "j",
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s2.Verify(tok)
	if !errors.Is(err, ErrProofIssuerMismatch) {
		t.Errorf("err = %v, want ErrProofIssuerMismatch", err)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	s := newTestSigner(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	tok, err := s.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "j",
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 修改最后一段
	parts := strings.Split(tok, ".")
	parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 64))
	tampered := strings.Join(parts, ".")
	_, err = s.Verify(tampered)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("err = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerify_EmptySubject(t *testing.T) {
	s := newTestSigner(t)
	// 手工构造一个合法 alg + typ + 正确签名但 sub 为空的 token
	header := proofHeader{Alg: "EdDSA", Typ: "JWT"}
	claims := proofClaims{
		Issuer:    "agentid-chain",
		Subject:   "",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		PublicKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
	hJSON, _ := json.Marshal(header)
	cJSON, _ := json.Marshal(claims)
	h64 := base64.RawURLEncoding.EncodeToString(hJSON)
	c64 := base64.RawURLEncoding.EncodeToString(cJSON)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	// 颁发者用 verifier 自己的 key
	signerForSign, _ := NewProofSigner(s.domainKey, "agentid-chain")
	signerForSign.domainKey = priv
	_ = signerForSign
	sig := ed25519.Sign(s.domainKey, []byte(h64+"."+c64))
	tok := h64 + "." + c64 + "." + base64.RawURLEncoding.EncodeToString(sig)
	_, err := s.Verify(tok)
	if !errors.Is(err, ErrProofClaimInvalid) {
		t.Errorf("err = %v, want ErrProofClaimInvalid", err)
	}
}

func TestVerify_BadPubKeyInClaims(t *testing.T) {
	s := newTestSigner(t)
	header := proofHeader{Alg: "EdDSA", Typ: "JWT"}
	claims := proofClaims{
		Issuer:    "agentid-chain",
		Subject:   "01234567-89ab-cdef-0123-456789abcdef",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		PublicKey: base64.RawURLEncoding.EncodeToString(make([]byte, 16)), // wrong length
	}
	hJSON, _ := json.Marshal(header)
	cJSON, _ := json.Marshal(claims)
	h64 := base64.RawURLEncoding.EncodeToString(hJSON)
	c64 := base64.RawURLEncoding.EncodeToString(cJSON)
	sig := ed25519.Sign(s.domainKey, []byte(h64+"."+c64))
	tok := h64 + "." + c64 + "." + base64.RawURLEncoding.EncodeToString(sig)
	_, err := s.Verify(tok)
	if !errors.Is(err, ErrProofClaimInvalid) {
		t.Errorf("err = %v, want ErrProofClaimInvalid", err)
	}
}

func TestVerify_HeaderNotJSON(t *testing.T) {
	s := newTestSigner(t)
	// header 是有效 base64，但不是 JSON
	h64 := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	c64 := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"x"}`))
	tok := h64 + "." + c64 + ".sig"
	_, err := s.Verify(tok)
	if !errors.Is(err, ErrProofMalformed) {
		t.Errorf("err = %v, want ErrProofMalformed", err)
	}
}

func TestVerify_BadClaimsJSON(t *testing.T) {
	s := newTestSigner(t)
	header := proofHeader{Alg: "EdDSA", Typ: "JWT"}
	hJSON, _ := json.Marshal(header)
	h64 := base64.RawURLEncoding.EncodeToString(hJSON)
	c64 := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	tok := h64 + "." + c64 + ".sig"
	_, err := s.Verify(tok)
	if !errors.Is(err, ErrProofMalformed) {
		t.Errorf("err = %v, want ErrProofMalformed", err)
	}
}

func TestVerify_BadSigEncoding(t *testing.T) {
	s := newTestSigner(t)
	header := proofHeader{Alg: "EdDSA", Typ: "JWT"}
	claims := proofClaims{
		Issuer:    "agentid-chain",
		Subject:   "01234567-89ab-cdef-0123-456789abcdef",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		PublicKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
	hJSON, _ := json.Marshal(header)
	cJSON, _ := json.Marshal(claims)
	h64 := base64.RawURLEncoding.EncodeToString(hJSON)
	c64 := base64.RawURLEncoding.EncodeToString(cJSON)
	tok := h64 + "." + c64 + ".!!notbase64!!"
	_, err := s.Verify(tok)
	if !errors.Is(err, ErrProofMalformed) {
		t.Errorf("err = %v, want ErrProofMalformed", err)
	}
}

func TestValidateUUIDString(t *testing.T) {
	cases := []struct {
		in  string
		ok  bool
	}{
		{"01234567-89ab-cdef-0123-456789abcdef", true},
		{"0123456789abcdef0123456789abcdef", true},
		{"ABCDEF12-3456-7890-ABCD-EF1234567890", true},
		{"", false},
		{"not-a-uuid", false}, // contains dash + non-hex chars
		{"xyz", false},
		{string(make([]byte, 70)), false},
	}
	for _, c := range cases {
		err := validateUUIDString(c.in)
		if c.ok && err != nil {
			t.Errorf("validateUUIDString(%q) err = %v, want nil", c.in, err)
		}
		if !c.ok && err == nil {
			t.Errorf("validateUUIDString(%q) err = nil, want error", c.in)
		}
	}
}
