// Package middleware: TLS 中间件测试。
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTLSReq(path, scheme, xfp string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = "agentid.example.com"
	if xfp != "" {
		r.Header.Set("X-Forwarded-Proto", xfp)
	}
	return r
}

func TestTLS_Disabled_PassThrough(t *testing.T) {
	cfg := TLSConfig{Enabled: false}
	h := TLS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newTLSReq("/api/x", "http", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTLS_HTTP_Redirects(t *testing.T) {
	cfg := DefaultTLSConfig()
	cfg.Enabled = true
	cfg.Port = 443
	h := TLS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream should not be called")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newTLSReq("/api/v1/agents", "http", ""))
	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected 308, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	want := "https://agentid.example.com/api/v1/agents"
	if loc != want {
		t.Fatalf("expected Location=%q, got %q", want, loc)
	}
	if hsts := rec.Header().Get("Strict-Transport-Security"); hsts == "" {
		t.Fatal("expected HSTS header on redirect")
	}
}

func TestTLS_HTTP_Redirect_WithCustomPort(t *testing.T) {
	cfg := DefaultTLSConfig()
	cfg.Port = 8443
	h := TLS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream should not be called")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newTLSReq("/api/v1/agents", "http", ""))
	loc := rec.Header().Get("Location")
	want := "https://agentid.example.com:8443/api/v1/agents"
	if loc != want {
		t.Fatalf("expected %q, got %q", want, loc)
	}
}

func TestTLS_ExemptPath(t *testing.T) {
	cfg := DefaultTLSConfig()
	called := false
	h := TLS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newTLSReq("/healthz", "http", ""))
	if !called {
		t.Fatal("expected downstream to be called for /healthz")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if hsts := rec.Header().Get("Strict-Transport-Security"); hsts == "" {
		t.Fatal("expected HSTS header even on exempt path")
	}
}

func TestTLS_TrustProxy_HTTPS(t *testing.T) {
	cfg := DefaultTLSConfig()
	cfg.TrustProxy = true
	called := false
	h := TLS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newTLSReq("/api/v1/agents", "http", "https"))
	if !called {
		t.Fatal("expected downstream to be called when XFP=https")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTLS_TrustProxy_Disabled_HTTPStillRedirects(t *testing.T) {
	cfg := DefaultTLSConfig()
	cfg.TrustProxy = false
	called := false
	h := TLS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newTLSReq("/api/v1/agents", "http", "https"))
	if called {
		t.Fatal("downstream should not be called when TrustProxy=false")
	}
	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected 308, got %d", rec.Code)
	}
}

func TestBuildHSTS(t *testing.T) {
	tests := []struct {
		name string
		cfg  TLSConfig
		want string
	}{
		{"basic", TLSConfig{HSTSMaxAge: 31536000}, "max-age=31536000"},
		{"includeSub", TLSConfig{HSTSMaxAge: 31536000, HSTSIncludeSubdomains: true}, "max-age=31536000; includeSubDomains"},
		{"preload", TLSConfig{HSTSMaxAge: 31536000, HSTSPreload: true}, "max-age=31536000; preload"},
		{"all", TLSConfig{HSTSMaxAge: 63072000, HSTSIncludeSubdomains: true, HSTSPreload: true}, "max-age=63072000; includeSubDomains; preload"},
		{"zero", TLSConfig{HSTSMaxAge: 0}, "max-age=0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildHSTS(tt.cfg); got != tt.want {
				t.Fatalf("buildHSTS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		in, host, port string
	}{
		{"example.com:8080", "example.com", "8080"},
		{"example.com", "example.com", ""},
		{"[::1]:8080", "[::1]", "8080"},
		{"[::1]", "[::1]", ""},
		{"localhost:443", "localhost", "443"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			h, p, _ := splitHostPort(tt.in)
			if h != tt.host || p != tt.port {
				t.Fatalf("splitHostPort(%q) = (%q, %q), want (%q, %q)", tt.in, h, p, tt.host, tt.port)
			}
		})
	}
}
