// Package telemetry: Logger 测试。
package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestNewServiceLogger_DefaultLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{
		ServiceName:    "test-svc",
		ServiceVersion: "1.0.0",
		Level:          "",
		Format:         "json",
	}
	logger := NewServiceLoggerTo(cfg, buf)
	logger.Info("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Fatal("expected log entry")
	}
	// 默认 level 是 info
	if strings.Contains(buf.String(), "debug") {
		t.Fatal("debug should not appear at info level")
	}
}

func TestNewServiceLogger_DebugLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{
		ServiceName: "test-svc",
		Level:       "debug",
		Format:      "json",
	}
	logger := NewServiceLoggerTo(cfg, buf)
	logger.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Fatal("expected debug message")
	}
}

func TestNewServiceLogger_ServiceFields(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{
		ServiceName:      "test-svc",
		ServiceVersion:   "2.0.1",
		ServiceNamespace: "prod",
		ServiceInstance:  "host-1",
		Level:            "info",
		Format:           "json",
	}
	logger := NewServiceLoggerTo(cfg, buf)
	logger.Info("test")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["service"] != "test-svc" {
		t.Fatalf("expected service=test-svc, got %v", m["service"])
	}
	if m["version"] != "2.0.1" {
		t.Fatalf("expected version=2.0.1, got %v", m["version"])
	}
	if m["namespace"] != "prod" {
		t.Fatalf("expected namespace=prod, got %v", m["namespace"])
	}
	if m["instance"] != "host-1" {
		t.Fatalf("expected instance=host-1, got %v", m["instance"])
	}
}

func TestNewServiceLogger_SensitiveRedaction(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{
		ServiceName:      "test-svc",
		Level:            "info",
		Format:           "json",
		SensitiveConfig:  &SensitiveConfig{},  // 显式启用脱敏
	}
	logger := NewServiceLoggerTo(cfg, buf)
	logger.Info("login", slog.String("username", "alice"), slog.String("password", "leaked"))
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["password"] == "leaked" {
		t.Fatalf("password should be redacted, got %v", m["password"])
	}
	if !strings.Contains(m["password"].(string), "***") {
		t.Fatalf("expected *** in redacted password, got %v", m["password"])
	}
}

func TestNewServiceLogger_TextFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{
		ServiceName: "test-svc",
		Format:      "text",
	}
	logger := NewServiceLoggerTo(cfg, buf)
	logger.Info("test")
	if !strings.Contains(buf.String(), "service=test-svc") {
		t.Fatalf("expected text format, got: %s", buf.String())
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error": slog.LevelError,
		"err":   slog.LevelError,
		"":      slog.LevelInfo, // default
		"foo":   slog.LevelInfo, // unknown → info
	}
	for in, want := range tests {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestWithRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{ServiceName: "t", Format: "json"}
	logger := NewServiceLoggerTo(cfg, buf)
	//lint:ignore SA1012 intentional nil ctx test
	scoped := WithRequestID(nil, logger, "req-123")
	scoped.Info("test")
	var m map[string]any
	_ = json.Unmarshal(buf.Bytes(), &m)
	if m["request_id"] != "req-123" {
		t.Fatalf("expected request_id=req-123, got %v", m["request_id"])
	}
}

func TestWithAgent(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{ServiceName: "t", Format: "json"}
	logger := NewServiceLoggerTo(cfg, buf)
	scoped := WithAgent(logger, "uuid-1", "alice", "2")
	scoped.Info("test")
	var m map[string]any
	_ = json.Unmarshal(buf.Bytes(), &m)
	if m["agent_id"] != "uuid-1" {
		t.Fatalf("expected agent_id=uuid-1, got %v", m["agent_id"])
	}
	if m["owner"] != "alice" {
		t.Fatalf("expected owner=alice, got %v", m["owner"])
	}
}

func TestWithError(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{ServiceName: "t", Format: "json"}
	logger := NewServiceLoggerTo(cfg, buf)
	scoped := WithError(logger, errors.New("boom"))
	scoped.Info("test")
	if !strings.Contains(buf.String(), "boom") {
		t.Fatalf("expected error message, got: %s", buf.String())
	}
}

func TestWithLatency(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := LogConfig{ServiceName: "t", Format: "json"}
	logger := NewServiceLoggerTo(cfg, buf)
	scoped := WithLatency(logger, 42.5)
	scoped.Info("test")
	var m map[string]any
	_ = json.Unmarshal(buf.Bytes(), &m)
	if m["latency_ms"] != 42.5 {
		t.Fatalf("expected latency_ms=42.5, got %v", m["latency_ms"])
	}
}
