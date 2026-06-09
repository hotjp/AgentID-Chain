package config

import (
	"strings"
	"testing"
)

func TestValidate_Default(t *testing.T) {
	cfg := New()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate(default) error = %v", err)
	}
}

func TestValidate_InvalidBackend(t *testing.T) {
	cfg := New()
	cfg.Backend.Type = "unknown"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid backend type")
	}
	if !strings.Contains(err.Error(), "backend.type") {
		t.Errorf("error should mention backend.type, got: %v", err)
	}
}

func TestValidate_InvalidRole(t *testing.T) {
	cfg := New()
	cfg.Service.Role = "hacker"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "service.role") {
		t.Errorf("error should mention service.role, got: %v", err)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := New()
	cfg.Log.Level = "verbose"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestValidate_InvalidLogFormat(t *testing.T) {
	cfg := New()
	cfg.Log.Format = "yaml"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid log format")
	}
}

func TestValidate_EmptyDSN(t *testing.T) {
	cfg := New()
	cfg.Storage.DB.DSN = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
}

func TestValidate_BadDSN(t *testing.T) {
	cfg := New()
	cfg.Storage.DB.DSN = "not a url with spaces"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for malformed DSN")
	}
}

func TestValidate_NegativeMaxOpen(t *testing.T) {
	cfg := New()
	cfg.Storage.DB.MaxOpen = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for MaxOpen <= 0")
	}
}

func TestValidate_OutboxMissingKey(t *testing.T) {
	cfg := New()
	cfg.Storage.Outbox.Enabled = true
	cfg.Storage.Outbox.StreamKey = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing outbox stream_key")
	}
}

func TestValidate_AuditRetention(t *testing.T) {
	cfg := New()
	cfg.Storage.Audit.RetentionDays = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for retention_days <= 0")
	}
}

func TestValidate_DefaultLevelOutOfRange(t *testing.T) {
	cfg := New()
	cfg.Authz.DefaultLevel = 99
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for default_level out of range")
	}
}

func TestValidate_ChainDriver(t *testing.T) {
	cfg := New()
	cfg.Backend.Type = "onchain"
	cfg.Chain.Driver = "ethereum" // invalid
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid chain driver")
	}
}

func TestValidate_ChainMissingRPC(t *testing.T) {
	cfg := New()
	cfg.Backend.Type = "onchain"
	cfg.Chain.Driver = "polygon"
	cfg.Chain.RPCURL = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing rpc_url")
	}
}

func TestValidate_LocalSkipsChain(t *testing.T) {
	cfg := New()
	cfg.Backend.Type = "local"
	cfg.Chain.Driver = "garbage" // should be ignored when local
	if err := cfg.Validate(); err != nil {
		t.Errorf("local should skip chain validation, got: %v", err)
	}
}

func TestValidate_TelemetrySampleRate(t *testing.T) {
	cfg := New()
	cfg.Telemetry.SampleRate = 1.5
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for sample_rate > 1")
	}
	cfg.Telemetry.SampleRate = -0.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for sample_rate < 0")
	}
}

func TestValidate_InvalidAddr(t *testing.T) {
	tests := []struct {
		addr string
		ok   bool
	}{
		{":8080", true},
		{"0.0.0.0:8080", true},
		{"localhost:8080", true},
		{"no-port", false},
		{":", false},
		{"", false},
		{":abc", false},
	}
	for _, tt := range tests {
		if got := validAddr(tt.addr); got != tt.ok {
			t.Errorf("validAddr(%q) = %v, want %v", tt.addr, got, tt.ok)
		}
	}
}

func TestValidationError_Error(t *testing.T) {
	e := &ValidationError{Field: "f", Message: "m"}
	got := e.Error()
	if !strings.Contains(got, "f") || !strings.Contains(got, "m") {
		t.Errorf("Error() = %q, want both f and m", got)
	}
}

func TestMustValidate_PanicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustValidate should panic on error")
		}
	}()
	cfg := New()
	cfg.Backend.Type = "invalid"
	cfg.MustValidate()
}

func TestValidRole(t *testing.T) {
	for _, r := range []string{"gateway", "auth-center", "tag-sense", "mcp", "migration", "cli"} {
		if !validRole(r) {
			t.Errorf("validRole(%q) = false, want true", r)
		}
	}
	if validRole("nope") {
		t.Error("validRole(nope) should be false")
	}
}

func TestValidLogLevel(t *testing.T) {
	for _, l := range []string{"debug", "info", "warn", "error"} {
		if !validLogLevel(l) {
			t.Errorf("validLogLevel(%q) = false, want true", l)
		}
	}
	if validLogLevel("trace") {
		t.Error("validLogLevel(trace) should be false")
	}
}

func TestValidLogFormat(t *testing.T) {
	for _, f := range []string{"json", "text"} {
		if !validLogFormat(f) {
			t.Errorf("validLogFormat(%q) = false, want true", f)
		}
	}
	if validLogFormat("xml") {
		t.Error("validLogFormat(xml) should be false")
	}
}
