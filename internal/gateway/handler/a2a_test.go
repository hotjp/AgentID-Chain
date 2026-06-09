package handler

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/authz/a2a"
	"github.com/alicebob/miniredis/v2"
	"github.com/agentid-chain/agentid-chain/internal/cache"
)

func newTestA2AHandler(t *testing.T) (*A2AHandler, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := a2a.NewIssuer(a2a.IssuerConfig{
		DomainKey: priv,
		Issuer:    "test-issuer",
		KeyID:     "test-kid",
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := a2a.NewVerifier(a2a.VerifierConfig{
		ExpectedIssuer:   "test-issuer",
		ExpectedAudience: "did:agentid:bob",
		Resolver:         &a2a.StaticKeyResolver{PublicKey: pub},
	})
	if err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	cch := cache.NewMiniredis(mr)
	revoker, err := a2a.NewRevoker(a2a.RevokerConfig{Cache: cch})
	if err != nil {
		t.Fatal(err)
	}
	return &A2AHandler{
		Issuer:       issuer,
		Verifier:     verifier,
		Revoker:      revoker,
		DefaultTTL:   10 * time.Minute,
		PublicKey:    pub,
		KeyID:        "test-kid",
	}, priv
}

func postJSON(t *testing.T, h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func TestA2A_Negotiate(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	w := postJSON(t, h.Negotiate, NegotiateRequest{
		Subject:    "did:agentid:alice",
		Audience:   "did:agentid:bob",
		Scope:      "read:tags",
		TrustLevel: 80,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp NegotiateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" {
		t.Error("empty token")
	}
	if resp.JTI == "" {
		t.Error("empty jti")
	}
	if resp.TrustLevel != 80 {
		t.Errorf("trust = %d", resp.TrustLevel)
	}
	if !strings.Contains(resp.Token, ".") {
		t.Error("token should be 3-segment JWT")
	}
}

func TestA2A_Negotiate_MissingFields(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	cases := []NegotiateRequest{
		{},                                        // empty
		{Subject: "x"},                            // no audience
		{Audience: "y"},                           // no subject
		{Subject: "x", Audience: "y", TrustLevel: 200}, // bad trust
	}
	for _, c := range cases {
		w := postJSON(t, h.Negotiate, c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for %+v, got %d", c, w.Code)
		}
	}
}

func TestA2A_Verify(t *testing.T) {
	h, _ := newTestA2AHandler(t)

	// 颁发
	nw := postJSON(t, h.Negotiate, NegotiateRequest{
		Subject:    "did:agentid:alice",
		Audience:   "did:agentid:bob",
		TrustLevel: 70,
	})
	var nr NegotiateResponse
	_ = json.Unmarshal(nw.Body.Bytes(), &nr)

	// 校验
	vw := postJSON(t, h.Verify, VerifyRequest{Token: nr.Token})
	if vw.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", vw.Code, vw.Body.String())
	}
	var vr VerifyResponse
	_ = json.Unmarshal(vw.Body.Bytes(), &vr)
	if !vr.OK {
		t.Errorf("verify failed: %s", vr.Error)
	}
	if vr.Claims == nil || vr.Claims.Subject != "did:agentid:alice" {
		t.Errorf("claims mismatch: %+v", vr.Claims)
	}
}

func TestA2A_Verify_BadToken(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	w := postJSON(t, h.Verify, VerifyRequest{Token: "garbage"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var vr VerifyResponse
	_ = json.Unmarshal(w.Body.Bytes(), &vr)
	if vr.OK {
		t.Error("expected OK=false")
	}
}

func TestA2A_Verify_RevokedToken(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	nw := postJSON(t, h.Negotiate, NegotiateRequest{
		Subject: "did:agentid:alice", Audience: "did:agentid:bob",
	})
	var nr NegotiateResponse
	_ = json.Unmarshal(nw.Body.Bytes(), &nr)

	// 撤销
	rw := postJSON(t, h.Revoke, RevokeRequest{JTI: nr.JTI, Reason: "test"})
	if rw.Code != http.StatusOK {
		t.Fatalf("revoke status = %d", rw.Code)
	}

	// 再次校验应失败
	vw := postJSON(t, h.Verify, VerifyRequest{Token: nr.Token})
	var vr VerifyResponse
	_ = json.Unmarshal(vw.Body.Bytes(), &vr)
	if vr.OK {
		t.Error("expected OK=false after revoke")
	}
	if !strings.Contains(vr.Error, "revoked") {
		t.Errorf("error = %q", vr.Error)
	}
}

func TestA2A_Revoke(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	w := postJSON(t, h.Revoke, RevokeRequest{JTI: "test-jti", Reason: "manual"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var rr RevokeResponse
	_ = json.Unmarshal(w.Body.Bytes(), &rr)
	if !rr.OK || rr.JTI != "test-jti" {
		t.Errorf("revoke response: %+v", rr)
	}
}

func TestA2A_Revoke_MissingJTI(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	w := postJSON(t, h.Revoke, RevokeRequest{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestA2A_List(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	subject := "did:agentid:list-user"

	// 颁发 3 个 token
	for i := 0; i < 3; i++ {
		_ = postJSON(t, h.Negotiate, NegotiateRequest{Subject: subject, Audience: "aud"})
	}

	w := postJSON(t, h.List, ListRequest{Subject: subject})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var lr ListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &lr)
	if lr.Count < 3 {
		t.Errorf("count = %d, want >= 3", lr.Count)
	}
}

func TestA2A_List_EmptySubject(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	w := postJSON(t, h.List, ListRequest{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestA2A_JWKS(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	w := httptest.NewRecorder()
	h.JWKSHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/jwk-set+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=") {
		t.Errorf("Cache-Control = %q", cc)
	}
	var jwks a2a.JWKS
	if err := json.Unmarshal(w.Body.Bytes(), &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(jwks.Keys))
	}
	jwk := jwks.Keys[0]
	if jwk.Kty != "OKP" || jwk.Crv != "Ed25519" || jwk.Alg != "EdDSA" {
		t.Errorf("jwk fields: %+v", jwk)
	}
	if jwk.Kid != "test-kid" {
		t.Errorf("kid = %q", jwk.Kid)
	}
	// 验证 base64url 解码是 32 字节
	raw, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		t.Errorf("decode X: %v", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		t.Errorf("X length = %d, want %d", len(raw), ed25519.PublicKeySize)
	}
}

func TestA2A_Negotiate_MethodNotAllowed(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.Negotiate(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", w.Code)
	}
}

func TestA2A_Negotiate_IssuerUnavailable(t *testing.T) {
	h := &A2AHandler{} // 无 issuer
	w := postJSON(t, h.Negotiate, NegotiateRequest{Subject: "x", Audience: "y"})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d", w.Code)
	}
}

func TestA2A_CustomTTL(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	w := postJSON(t, h.Negotiate, NegotiateRequest{
		Subject: "x", Audience: "y", TTLSec: 30,
	})
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	var resp NegotiateResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	dur := time.Until(resp.ExpiresAt)
	if dur < 25*time.Second || dur > 35*time.Second {
		t.Errorf("ttl = %v, want ~30s", dur)
	}
}
