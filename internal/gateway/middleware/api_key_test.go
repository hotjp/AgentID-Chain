package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKey_Valid(t *testing.T) {
	cfg := APIKeyConfig{Keys: []string{"secret-1", "secret-2"}}
	rec := httptest.NewRecorder()
	called := false
	h := APIKey(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set("X-API-Key", "secret-2")
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("valid key should pass")
	}
}

func TestAPIKey_Missing(t *testing.T) {
	cfg := APIKeyConfig{Keys: []string{"secret"}}
	rec := httptest.NewRecorder()
	h := APIKey(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v2/test", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPIKey_Invalid(t *testing.T) {
	cfg := APIKeyConfig{Keys: []string{"secret"}}
	rec := httptest.NewRecorder()
	h := APIKey(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set("X-API-Key", "wrong")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPIKey_SkipPath(t *testing.T) {
	cfg := APIKeyConfig{Keys: []string{"secret"}, SkipPaths: []string{"/healthz"}}
	rec := httptest.NewRecorder()
	called := false
	h := APIKey(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if !called {
		t.Error("skip path should bypass")
	}
}

func TestAPIKey_CustomHeader(t *testing.T) {
	cfg := APIKeyConfig{Keys: []string{"k"}, Header: "X-Auth-Token"}
	rec := httptest.NewRecorder()
	called := false
	h := APIKey(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set("X-Auth-Token", "k")
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("custom header should work")
	}
}
