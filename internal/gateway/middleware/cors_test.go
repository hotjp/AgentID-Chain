package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_Headers(t *testing.T) {
	rec := httptest.NewRecorder()
	h := CORS(CORSConfig{AllowOrigins: "https://example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("missing allow-methods")
	}
}

func TestCORS_Preflight(t *testing.T) {
	rec := httptest.NewRecorder()
	called := false
	h := CORS(DefaultCORSConfig())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	h.ServeHTTP(rec, httptest.NewRequest("OPTIONS", "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d", rec.Code)
	}
	if called {
		t.Error("downstream handler should not be called for preflight")
	}
}

func TestCORS_DefaultConfig(t *testing.T) {
	h := CORS(CORSConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("default origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
