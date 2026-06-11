package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeAAPProof(t *testing.T) string {
	t.Helper()
	body, _ := json.Marshal(AAPProofHeader{
		AgentUUID:   "11111111-2222-3333-4444-555555555555",
		ChallengeID: "challenge-xyz",
		PubKey:      base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
		Sig:         base64.RawURLEncoding.EncodeToString(make([]byte, 64)),
	})
	return base64.RawURLEncoding.EncodeToString(body)
}

func TestAAP_MissingHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	h := AAP()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v2/test", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAAP_BadBase64(t *testing.T) {
	rec := httptest.NewRecorder()
	h := AAP()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set(HeaderAAPProof, "!!!not-base64!!!")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAAP_HappyPath(t *testing.T) {
	rec := httptest.NewRecorder()
	var gotUUID string
	h := AAP()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUUID = AgentUUIDFromContext(r.Context())
	}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set(HeaderAAPProof, makeAAPProof(t))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if gotUUID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("uuid = %q", gotUUID)
	}
}

func TestAAP_ProbePathExempt(t *testing.T) {
	rec := httptest.NewRecorder()
	called := false
	h := AAP()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if !called {
		t.Error("probe path should bypass AAP")
	}
}

func TestAAP_VersionPrefix(t *testing.T) {
	rec := httptest.NewRecorder()
	h := AAP()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/api/v2/test", nil)
	req.Header.Set(HeaderAAPProof, "v1:"+makeAAPProof(t))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("v1 prefix status = %d", rec.Code)
	}
}

func TestAAP_AgentUUIDEmptyContext(t *testing.T) {
	//lint:ignore SA1012 intentional nil ctx test
	if uuid := AgentUUIDFromContext(nil); uuid != "" {
		t.Errorf("nil ctx = %q", uuid)
	}
}
