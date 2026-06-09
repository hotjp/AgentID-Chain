package aap

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestMiddleware(t *testing.T) (*Middleware, *ProofSigner) {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, err := NewProofSigner(priv, "agentid-chain")
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewMiddleware(MiddlewareConfig{Signer: s})
	if err != nil {
		t.Fatal(err)
	}
	return m, s
}

func newRequest(t *testing.T, headerVal string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	if headerVal != "" {
		r.Header.Set("X-AAP-Proof", headerVal)
	}
	return r
}

func signToken(t *testing.T, s *ProofSigner, ttl time.Duration) string {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	tok, err := s.Sign(SignInput{
		AgentUUID:   "01234567-89ab-cdef-0123-456789abcdef",
		AgentPubKey: priv.Public().(ed25519.PublicKey),
		JTI:         "test-jti",
		TTL:         ttl,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestNewMiddleware_NilSigner(t *testing.T) {
	_, err := NewMiddleware(MiddlewareConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewMiddleware_DefaultHeader(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "test")
	m, err := NewMiddleware(MiddlewareConfig{Signer: s})
	if err != nil {
		t.Fatal(err)
	}
	if m.cfg.HeaderName != "X-AAP-Proof" {
		t.Errorf("HeaderName = %s", m.cfg.HeaderName)
	}
}

func TestNewMiddleware_CustomHeader(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "test")
	m, err := NewMiddleware(MiddlewareConfig{Signer: s, HeaderName: "X-Token"})
	if err != nil {
		t.Fatal(err)
	}
	if m.cfg.HeaderName != "X-Token" {
		t.Errorf("HeaderName = %s", m.cfg.HeaderName)
	}
}

func TestMiddleware_HappyPath(t *testing.T) {
	m, s := newTestMiddleware(t)
	tok := signToken(t, s, 5*time.Minute)

	var got *UserContext
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := m.Wrap(next)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, newRequest(t, tok))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got == nil {
		t.Fatal("user context not injected")
	}
	if got.AgentUUID != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Errorf("AgentUUID = %s", got.AgentUUID)
	}
	if got.JTI != "test-jti" {
		t.Errorf("JTI = %s", got.JTI)
	}
}

func TestMiddleware_MissingHeader(t *testing.T) {
	m, _ := newTestMiddleware(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := m.Wrap(next)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, newRequest(t, ""))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if called {
		t.Error("next should not be called")
	}
	// 验证 body 是 JSON
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Errorf("body not JSON: %v", err)
	}
}

func TestMiddleware_BadToken(t *testing.T) {
	m, _ := newTestMiddleware(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := m.Wrap(next)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, newRequest(t, "garbage.token.here"))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "agentid-chain")
	// 签一个 1h 前过期的 token
	fixed := time.Now().Add(-2 * time.Hour)
	s.SetClock(func() time.Time { return fixed })
	tok := signToken(t, s, time.Hour)
	// 现在切到真实时间
	s.SetClock(time.Now)

	m, _ := NewMiddleware(MiddlewareConfig{Signer: s})
	rr := httptest.NewRecorder()
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, newRequest(t, tok))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "expired") {
		t.Errorf("body = %s, want contains 'expired'", rr.Body.String())
	}
}

func TestMiddleware_RequireHTTPS_NoTLS(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "agentid-chain")
	tok := signToken(t, s, 5*time.Minute)

	m, _ := NewMiddleware(MiddlewareConfig{Signer: s, RequireHTTPS: true})
	rr := httptest.NewRecorder()
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, newRequest(t, tok))

	if rr.Code != http.StatusUpgradeRequired {
		t.Errorf("status = %d, want 426", rr.Code)
	}
}

func TestMiddleware_RequireHTTPS_ForwardedProto(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "agentid-chain")
	tok := signToken(t, s, 5*time.Minute)

	m, _ := NewMiddleware(MiddlewareConfig{Signer: s, RequireHTTPS: true})
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	r := newRequest(t, tok)
	r.Header.Set("X-Forwarded-Proto", "https")
	handler.ServeHTTP(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestMiddleware_RequireHTTPS_ForwardedProtoLower(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "agentid-chain")
	tok := signToken(t, s, 5*time.Minute)

	m, _ := NewMiddleware(MiddlewareConfig{Signer: s, RequireHTTPS: true})
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	r := newRequest(t, tok)
	r.Header.Set("X-Forwarded-Proto", "HTTPS")
	handler.ServeHTTP(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestMiddleware_Skipper(t *testing.T) {
	m, _ := newTestMiddleware(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	skip := func(r *http.Request) bool { return r.URL.Path == "/healthz" }
	m.cfg.Skipper = skip
	handler := m.Wrap(next)
	rr := httptest.NewRecorder()
	r := newRequest(t, "")
	r.URL.Path = "/healthz"
	handler.ServeHTTP(rr, r)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !called {
		t.Error("next should be called when skipped")
	}
}

func TestMiddleware_CustomErrorHandler(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "agentid-chain")
	m, err := NewMiddleware(MiddlewareConfig{
		Signer: s,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte("custom error: " + err.Error()))
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, newRequest(t, ""))
	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rr.Code)
	}
	if !strings.HasPrefix(rr.Body.String(), "custom error:") {
		t.Errorf("body = %s", rr.Body.String())
	}
}

func TestMiddleware_ConnectUnary(t *testing.T) {
	m, s := newTestMiddleware(t)
	tok := signToken(t, s, 5*time.Minute)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := m.ConnectUnary(next)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, newRequest(t, tok))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !called {
		t.Error("next should be called")
	}
}

func TestFromContext_NilContext(t *testing.T) {
	if got := FromContext(nil); got != nil {
		t.Errorf("FromContext(nil) = %v, want nil", got)
	}
}

func TestFromContext_NoValue(t *testing.T) {
	ctx := context.Background()
	if got := FromContext(ctx); got != nil {
		t.Errorf("FromContext(empty) = %v, want nil", got)
	}
}

func TestWithUserContext_NilUC(t *testing.T) {
	ctx := WithUserContext(context.Background(), nil)
	if got := FromContext(ctx); got != nil {
		t.Errorf("FromContext(nil injected) = %v", got)
	}
}

func TestWithUserContext_OK(t *testing.T) {
	uc := &UserContext{AgentUUID: "abc"}
	ctx := WithUserContext(context.Background(), uc)
	got := FromContext(ctx)
	if got == nil || got.AgentUUID != "abc" {
		t.Error("injection failed")
	}
}

func TestIsForwardedHTTPS(t *testing.T) {
	cases := []struct {
		proto string
		want  bool
	}{
		{"", false},
		{"http", false},
		{"https", true},
		{"HTTPS", true},
		{"HtTpS", true},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		if c.proto != "" {
			r.Header.Set("X-Forwarded-Proto", c.proto)
		}
		if got := isForwardedHTTPS(r); got != c.want {
			t.Errorf("proto=%q: got %v, want %v", c.proto, got, c.want)
		}
	}
}

func TestIntToStr(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{123, "123"},
		{401, "401"},
		{426, "426"},
		{-1, "-1"},
		{-100, "-100"},
	}
	for _, c := range cases {
		if got := intToStr(c.in); got != c.want {
			t.Errorf("intToStr(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultErrorHandler_StatusCodes(t *testing.T) {
	cases := []struct {
		err        error
		wantStatus int
	}{
		{ErrHTTPSRequired, http.StatusUpgradeRequired},
		{ErrMissingProof, http.StatusUnauthorized},
		{ErrResponseExpired, http.StatusUnauthorized},
		{ErrSignatureInvalid, http.StatusUnauthorized},
		{ErrProofIssuerMismatch, http.StatusUnauthorized},
		{ErrProofUnsupportedAlg, http.StatusUnauthorized},
		{ErrProofMalformed, http.StatusUnauthorized},
	}
	for _, c := range cases {
		rr := httptest.NewRecorder()
		defaultErrorHandler(rr, httptest.NewRequest("GET", "/", nil), c.err)
		if rr.Code != c.wantStatus {
			t.Errorf("err=%v: status=%d, want %d", c.err, rr.Code, c.wantStatus)
		}
	}
}

func TestUserContext_Fields(t *testing.T) {
	// 验证 UserContext 字段传递
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewProofSigner(priv, "agentid-chain")
	tok := signToken(t, s, 5*time.Minute)

	m, _ := NewMiddleware(MiddlewareConfig{Signer: s})
	var got *UserContext
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromContext(r.Context())
	})
	handler := m.Wrap(next)
	handler.ServeHTTP(httptest.NewRecorder(), newRequest(t, tok))

	if got.Proof != tok {
		t.Error("Proof field not set")
	}
	if got.Issuer != "agentid-chain" {
		t.Errorf("Issuer = %s", got.Issuer)
	}
	if got.ExpiresAt.Before(got.IssuedAt) {
		t.Error("ExpiresAt before IssuedAt")
	}
}
