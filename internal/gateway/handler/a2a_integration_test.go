package handler

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/authz/a2a"
	"github.com/alicebob/miniredis/v2"
	"github.com/agentid-chain/agentid-chain/internal/cache"
)

// TestA2A_FullFlow 端到端：negotiate → verify → revoke → verify（fail）
func TestA2A_FullFlow(t *testing.T) {
	h, _ := newTestA2AHandler(t)

	// 把 4 个 handler 挂到 mux
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/negotiate", h.Negotiate)
	mux.HandleFunc("/a2a/verify", h.Verify)
	mux.HandleFunc("/a2a/revoke", h.Revoke)
	mux.HandleFunc("/a2a/list", h.List)
	mux.HandleFunc("/.well-known/jwks.json", h.JWKSHandler)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. JWKS discover
	resp, err := http.Get(srv.URL + "/.well-known/jwks.json")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("jwks status = %d", resp.StatusCode)
	}
	var jwks a2a.JWKS
	_ = json.NewDecoder(resp.Body).Decode(&jwks)
	resp.Body.Close()
	if len(jwks.Keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(jwks.Keys))
	}

	// 2. negotiate
	body, _ := json.Marshal(NegotiateRequest{
		Subject:    "did:agentid:alice",
		Audience:   "did:agentid:bob",
		Scope:      "read:tags",
		TrustLevel: 90,
		AuditID:    "audit-001",
		TTLSec:     60,
	})
	resp, err = http.Post(srv.URL+"/a2a/negotiate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("negotiate status = %d body=%s", resp.StatusCode, b)
	}
	var nr NegotiateResponse
	_ = json.NewDecoder(resp.Body).Decode(&nr)
	resp.Body.Close()
	if nr.Token == "" || nr.JTI == "" {
		t.Fatal("empty token/jti")
	}

	// 3. verify (pass)
	body, _ = json.Marshal(VerifyRequest{Token: nr.Token})
	resp, err = http.Post(srv.URL+"/a2a/verify", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var vr VerifyResponse
	_ = json.NewDecoder(resp.Body).Decode(&vr)
	resp.Body.Close()
	if !vr.OK {
		t.Errorf("verify should pass: %s", vr.Error)
	}
	if vr.Claims == nil || vr.Claims.AuditID != "audit-001" {
		t.Errorf("claims: %+v", vr.Claims)
	}

	// 4. list
	body, _ = json.Marshal(ListRequest{Subject: "did:agentid:alice"})
	resp, err = http.Post(srv.URL+"/a2a/list", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var lr ListResponse
	_ = json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	if lr.Count < 1 {
		t.Errorf("list count = %d", lr.Count)
	}

	// 5. revoke
	body, _ = json.Marshal(RevokeRequest{JTI: nr.JTI, Reason: "test"})
	resp, err = http.Post(srv.URL+"/a2a/revoke", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// 6. verify (fail because revoked)
	body, _ = json.Marshal(VerifyRequest{Token: nr.Token})
	resp, err = http.Post(srv.URL+"/a2a/verify", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&vr)
	resp.Body.Close()
	if vr.OK {
		t.Error("verify should fail after revoke")
	}
	if !strings.Contains(vr.Error, "revoked") {
		t.Errorf("error = %q", vr.Error)
	}
}

// TestA2A_NegotiateMany 颁发多个 token 后 list 应全部包含。
func TestA2A_NegotiateMany(t *testing.T) {
	h, _ := newTestA2AHandler(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/a2a/negotiate", h.Negotiate)
	mux.HandleFunc("/a2a/list", h.List)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	const N = 5
	subject := "did:agentid:bulk"
	for i := 0; i < N; i++ {
		body, _ := json.Marshal(NegotiateRequest{Subject: subject, Audience: "aud"})
		resp, err := http.Post(srv.URL+"/a2a/negotiate", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	body, _ := json.Marshal(ListRequest{Subject: subject})
	resp, err := http.Post(srv.URL+"/a2a/list", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var lr ListResponse
	_ = json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	if lr.Count != N {
		t.Errorf("count = %d, want %d", lr.Count, N)
	}
}

// TestA2A_JWKS_ExposesCorrectKey 验证 JWKS 暴露的公钥可正确用于验签。
func TestA2A_JWKS_ExposesCorrectKey(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	h := &A2AHandler{
		Issuer: func() *a2a.Issuer {
			is, _ := a2a.NewIssuer(a2a.IssuerConfig{DomainKey: priv, Issuer: "iss", KeyID: "kid"})
			return is
		}(),
		Verifier: func() *a2a.Verifier {
			v, _ := a2a.NewVerifier(a2a.VerifierConfig{
				ExpectedIssuer:   "iss",
				ExpectedAudience: "aud",
				Resolver:         &a2a.StaticKeyResolver{PublicKey: pub},
			})
			return v
		}(),
		Revoker: func() *a2a.Revoker {
			mr := miniredis.RunT(t)
			r, _ := a2a.NewRevoker(a2a.RevokerConfig{Cache: cache.NewMiniredis(mr)})
			return r
		}(),
		PublicKey: pub,
		KeyID:     "kid",
		DefaultTTL: 1 * time.Minute,
	}
	_ = h

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", h.JWKSHandler)
	mux.HandleFunc("/a2a/negotiate", h.Negotiate)
	mux.HandleFunc("/a2a/verify", h.Verify)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 拿 JWKS
	resp, _ := http.Get(srv.URL + "/.well-known/jwks.json")
	var jwks a2a.JWKS
	_ = json.NewDecoder(resp.Body).Decode(&jwks)
	resp.Body.Close()

	// 用公钥构造 resolver → 校验 token
	if len(jwks.Keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(jwks.Keys))
	}
	resolver, err := a2a.ResolverFromJWKS(&jwks)
	if err != nil {
		t.Fatal(err)
	}
	v2, _ := a2a.NewVerifier(a2a.VerifierConfig{
		ExpectedIssuer:   "iss",
		ExpectedAudience: "aud",
		Resolver:         resolver,
	})

	// negotiate + verify
	body, _ := json.Marshal(NegotiateRequest{Subject: "s", Audience: "aud"})
	resp, _ = http.Post(srv.URL+"/a2a/negotiate", "application/json", bytes.NewReader(body))
	var nr NegotiateResponse
	_ = json.NewDecoder(resp.Body).Decode(&nr)
	resp.Body.Close()

	claims, err := v2.Verify(nr.Token)
	if err != nil {
		t.Fatalf("verify with JWKS-derived key: %v", err)
	}
	if claims.Subject != "s" {
		t.Errorf("subject = %q", claims.Subject)
	}
}
