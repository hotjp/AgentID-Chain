package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_Generated(t *testing.T) {
	rec := httptest.NewRecorder()
	var gotID string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = RequestIDFromContext(r.Context())
	}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if gotID == "" {
		t.Error("expected generated request_id")
	}
	if rec.Header().Get(HeaderRequestID) == "" {
		t.Error("expected X-Request-ID in response")
	}
	if rec.Header().Get(HeaderRequestID) != gotID {
		t.Error("response header != context value")
	}
}

func TestRequestID_Passthrough(t *testing.T) {
	rec := httptest.NewRecorder()
	want := "client-supplied-id-123"
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderRequestID, want)
	var gotID string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = RequestIDFromContext(r.Context())
	}))
	h.ServeHTTP(rec, req)
	if gotID != want {
		t.Errorf("ctx id = %q, want %q", gotID, want)
	}
	if rec.Header().Get(HeaderRequestID) != want {
		t.Errorf("response header = %q, want %q", rec.Header().Get(HeaderRequestID), want)
	}
}

func TestRequestID_Empty(t *testing.T) {
	//lint:ignore SA1012 intentional nil ctx test
	if id := RequestIDFromContext(nil); id != "" {
		t.Errorf("nil ctx = %q", id)
	}
}
