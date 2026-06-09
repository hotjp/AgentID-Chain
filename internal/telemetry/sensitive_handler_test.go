// Package telemetry: Sensitive Handler 测试。
package telemetry

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func newTestLogger(cfg *SensitiveConfig) (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	base := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(NewSensitiveHandler(base, cfg)), buf
}

func parseLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json parse: %v\nraw: %s", err, buf.String())
	}
	return m
}

func TestSensitiveHandler_RedactsPassword(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("login", slog.String("username", "alice"), slog.String("password", "super-secret-123"))
	m := parseLog(t, buf)
	if m["password"] == "super-secret-123" {
		t.Fatalf("password not redacted: %v", m)
	}
	if !strings.Contains(m["password"].(string), "***") {
		t.Fatalf("expected *** in redacted password, got %v", m["password"])
	}
}

func TestSensitiveHandler_RedactsToken(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("api", slog.String("access_token", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbGljZSJ9.signature"))
	m := parseLog(t, buf)
	if strings.Contains(m["access_token"].(string), "eyJ") {
		t.Fatalf("JWT not redacted: %v", m["access_token"])
	}
}

func TestSensitiveHandler_RedactsAPIKey(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("req", slog.String("api_key", "AKIAIOSFODNN7EXAMPLE"))
	m := parseLog(t, buf)
	v := m["api_key"].(string)
	if strings.Contains(v, "AKIA") {
		t.Fatalf("AWS key not redacted: %v", v)
	}
}

func TestSensitiveHandler_RedactsDSN(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("connect", slog.String("dsn", "postgres://user:devpass@localhost:5432/db"))
	m := parseLog(t, buf)
	v := m["dsn"].(string)
	if strings.Contains(v, "devpass") {
		t.Fatalf("DSN password not redacted: %v", v)
	}
}

func TestSensitiveHandler_RedactsNested(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("req", slog.Group("auth", slog.String("token", "secret-token-123"), slog.String("user", "alice")))
	m := parseLog(t, buf)
	auth := m["auth"].(map[string]any)
	if !strings.Contains(auth["token"].(string), "***") {
		t.Fatalf("nested token not redacted: %v", auth["token"])
	}
	if auth["user"] != "alice" {
		t.Fatalf("user should not be redacted: %v", auth["user"])
	}
}

func TestSensitiveHandler_NonSensitiveKey(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("req", slog.String("request_id", "abc-123"), slog.String("username", "alice"))
	m := parseLog(t, buf)
	if m["request_id"] != "abc-123" {
		t.Fatalf("request_id should not be redacted: %v", m["request_id"])
	}
	if m["username"] != "alice" {
		t.Fatalf("username should not be redacted: %v", m["username"])
	}
}

func TestSensitiveHandler_ExtraKeys(t *testing.T) {
	cfg := &SensitiveConfig{
		ExtraKeys:    []string{"custom_field"},
		Replacement:  "***",
	}
	log, buf := newTestLogger(cfg)
	log.Info("x", slog.String("custom_field", "shhh"), slog.String("other", "ok"))
	m := parseLog(t, buf)
	if !strings.Contains(m["custom_field"].(string), "***") {
		t.Fatalf("custom_field not redacted: %v", m["custom_field"])
	}
	if m["other"] != "ok" {
		t.Fatalf("other should not be redacted: %v", m["other"])
	}
}

func TestSensitiveHandler_Disabled(t *testing.T) {
	cfg := &SensitiveConfig{Disabled: true}
	log, buf := newTestLogger(cfg)
	log.Info("x", slog.String("password", "leaked"))
	m := parseLog(t, buf)
	if m["password"] != "leaked" {
		t.Fatalf("disabled mode should not redact: %v", m["password"])
	}
}

func TestSensitiveHandler_PreservesLength(t *testing.T) {
	cfg := &SensitiveConfig{PreserveLength: true}
	log, buf := newTestLogger(cfg)
	log.Info("x", slog.String("password", "abcdefghij"))
	m := parseLog(t, buf)
	v := m["password"].(string)
	if len(v) != len("abcdefghij") {
		t.Fatalf("preserve length failed: %q (len=%d, want %d)", v, len(v), len("abcdefghij"))
	}
	if v[0] != 'a' || v[len(v)-1] != 'j' {
		t.Fatalf("first/last char not preserved: %q", v)
	}
}

func TestSensitiveHandler_PEMKey(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("x", slog.String("private_key", "-----BEGIN OPENSSH PRIVATE KEY-----\nxxx"))
	m := parseLog(t, buf)
	v := m["private_key"].(string)
	if strings.Contains(v, "PRIVATE KEY") {
		t.Fatalf("PEM marker not redacted: %v", v)
	}
}

func TestSensitiveHandler_NonStringValue(t *testing.T) {
	log, buf := newTestLogger(nil)
	log.Info("x", slog.Int("secret_level", 5))
	m := parseLog(t, buf)
	if m["secret_level"] != "***" {
		t.Fatalf("non-string sensitive value not replaced: %v", m["secret_level"])
	}
}

func TestMaskMiddle(t *testing.T) {
	tests := []struct {
		in       string
		cfg      SensitiveConfig
		expected string
	}{
		{"abc", SensitiveConfig{Replacement: "***"}, "***"},
		{"abcd", SensitiveConfig{Replacement: "***"}, "***"},
		{"abcde", SensitiveConfig{Replacement: "***"}, "a***e"},
		{"abcdefgh", SensitiveConfig{Replacement: "***"}, "a***h"},
		{"abcdef", SensitiveConfig{Replacement: "***", PreserveLength: true}, "a****f"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := maskMiddle(tt.in, tt.cfg); got != tt.expected {
				t.Fatalf("maskMiddle(%q) = %q, want %q", tt.in, got, tt.expected)
			}
		})
	}
}

func TestSensitiveHandler_WithAttrs(t *testing.T) {
	log, buf := newTestLogger(nil)
	// 在 WithAttrs 阶段就预置敏感 attr
	scoped := log.With(slog.String("authorization", "Bearer secret"))
	scoped.Info("hello", slog.String("user", "bob"))
	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Fatalf("authorization in WithAttrs not redacted: %s", out)
	}
	if !strings.Contains(out, "bob") {
		t.Fatalf("user not preserved: %s", out)
	}
}
