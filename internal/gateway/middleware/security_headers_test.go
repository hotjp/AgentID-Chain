// Package middleware: Security Headers 中间件测试。
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_Disabled_PassThrough(t *testing.T) {
	cfg := SecurityHeadersConfig{Enabled: false}
	h := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil))
	if got := rec.Header().Get("X-Content-Type-Options"); got != "" {
		t.Fatalf("expected no headers, got X-Content-Type-Options=%q", got)
	}
}

func TestSecurityHeaders_Default_InjectsAll(t *testing.T) {
	cfg := DefaultSecurityHeadersConfig()
	h := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil))

	expected := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"Referrer-Policy":                   "strict-origin-when-cross-origin",
		"Cross-Origin-Opener-Policy":        "same-origin",
		"Cross-Origin-Embedder-Policy":      "require-corp",
		"Cross-Origin-Resource-Policy":      "same-origin",
		"X-Permitted-Cross-Domain-Policies": "none",
		"Content-Security-Policy":           "default-src 'self';",
		"Strict-Transport-Security":         "max-age=31536000; includeSubDomains; preload",
	}
	for k, want := range expected {
		if got := rec.Header().Get(k); !contains(got, want) {
			t.Errorf("header %q = %q, want contains %q", k, got, want)
		}
	}
}

func TestSecurityHeaders_NoCachePaths(t *testing.T) {
	cfg := DefaultSecurityHeadersConfig()
	h := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		path        string
		wantNoStore bool
	}{
		{"/api/v1/agents", true},
		{"/v1/health", true},
		{"/aap/handshake", true},
		{"/auth/login", true},
		{"/static/file.js", false},
		{"/", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			cc := rec.Header().Get("Cache-Control")
			if tt.wantNoStore {
				if cc == "" || !contains(cc, "no-store") {
					t.Errorf("path %q: expected no-store, got %q", tt.path, cc)
				}
			} else {
				if cc != "" {
					t.Errorf("path %q: expected no Cache-Control, got %q", tt.path, cc)
				}
			}
		})
	}
}

func TestSecurityHeaders_PassesThroughDownstream(t *testing.T) {
	cfg := DefaultSecurityHeadersConfig()
	called := false
	h := SecurityHeaders(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil))
	if !called {
		t.Fatal("expected downstream to be called")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", rec.Code)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
