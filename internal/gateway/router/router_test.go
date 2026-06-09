package router

import (
	"net/http/httptest"
	"testing"

	"github.com/agentid-chain/agentid-chain/internal/gateway/handler"
)

func TestRouter_Registers(t *testing.T) {
	health := handler.NewHealthHandler(nil)
	r := New(nil, health)
	mux := r.Mux()
	if mux == nil {
		t.Fatal("nil mux")
	}
	// 验证 healthz 端点已注册
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Errorf("/healthz = %d", rec.Code)
	}
	// /live
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/live", nil))
	if rec.Code != 200 {
		t.Errorf("/live = %d", rec.Code)
	}
	// /ready
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 200 {
		t.Errorf("/readyz = %d", rec.Code)
	}
}

func TestRouter_HealthReadyFails(t *testing.T) {
	health := handler.NewHealthHandler(func() error {
		return nil // first test
	})
	r := New(nil, health)
	mux := r.Mux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 200 {
		t.Errorf("readyz = %d", rec.Code)
	}
}
