package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUABlock_Blocked(t *testing.T) {
	cfg := UAConfig{BlockPatterns: []string{"sqlmap"}, BlockEmpty: true}
	rec := httptest.NewRecorder()
	h := UABlock(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set("User-Agent", "sqlmap/1.0")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestUABlock_Allowed(t *testing.T) {
	cfg := UAConfig{BlockPatterns: []string{"sqlmap"}, BlockEmpty: true}
	rec := httptest.NewRecorder()
	called := false
	h := UABlock(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestUABlock_Empty(t *testing.T) {
	cfg := UAConfig{BlockEmpty: true}
	rec := httptest.NewRecorder()
	h := UABlock(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v2/test", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("empty UA status = %d", rec.Code)
	}
}

func TestUABlock_ProbePathsExempt(t *testing.T) {
	cfg := UAConfig{BlockEmpty: true, BlockPatterns: []string{"sqlmap"}}
	rec := httptest.NewRecorder()
	called := false
	h := UABlock(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if !called {
		t.Error("probe path should bypass UA block")
	}
}

func TestUABlock_AllowList(t *testing.T) {
	cfg := UAConfig{AllowList: []string{"mybot"}, BlockEmpty: true}
	rec := httptest.NewRecorder()
	called := false
	h := UABlock(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set("User-Agent", "mybot/1.0")
	h.ServeHTTP(rec, req)
	if !called {
		t.Error("allowlisted UA should pass")
	}
}

func TestUABlock_DefaultConfig(t *testing.T) {
	h := UABlock(UAConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set("User-Agent", "sqlmap")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("default should block sqlmap, got %d", rec.Code)
	}
}
