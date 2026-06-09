package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth_LiveAlwaysOK(t *testing.T) {
	h := NewHealthHandler(nil)
	rec := httptest.NewRecorder()
	h.Live(rec, httptest.NewRequest("GET", "/live", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestHealth_ReadyNoFn(t *testing.T) {
	h := NewHealthHandler(nil)
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestHealth_ReadyFnOK(t *testing.T) {
	h := NewHealthHandler(func() error { return nil })
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestHealth_ReadyFnFail(t *testing.T) {
	h := NewHealthHandler(func() error { return errors.New("redis down") })
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestHealth_HealthzFnFail(t *testing.T) {
	h := NewHealthHandler(func() error { return errors.New("db down") })
	rec := httptest.NewRecorder()
	h.Healthz(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d", rec.Code)
	}
}
