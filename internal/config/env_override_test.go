package config

import (
	"reflect"
	"testing"
	"time"
)

func TestEnvOverride_Apply_BasicMapping(t *testing.T) {
	e := DefaultEnvOverride()
	env := map[string]string{
		"AGENTID_SERVICE_NAME":  "env-svc",
		"AGENTID_BACKEND_TYPE":  "onchain",
		"AGENTID_STORAGE_DB_DSN": "postgres://env:5432/db",
	}
	overrides := e.Apply(env)
	if len(overrides) != 3 {
		t.Fatalf("Apply() got %d overrides, want 3", len(overrides))
	}

	// 验证转换
	got := make(map[string]Override)
	for _, o := range overrides {
		got[o.EnvKey] = o
	}
	if got["AGENTID_SERVICE_NAME"].ConfigKey != "service.name" {
		t.Errorf("ConfigKey = %q, want %q", got["AGENTID_SERVICE_NAME"].ConfigKey, "service.name")
	}
	if got["AGENTID_BACKEND_TYPE"].ConfigKey != "backend.type" {
		t.Errorf("ConfigKey = %q, want %q", got["AGENTID_BACKEND_TYPE"].ConfigKey, "backend.type")
	}
	if got["AGENTID_STORAGE_DB_DSN"].ConfigKey != "storage.db.dsn" {
		t.Errorf("ConfigKey = %q, want %q", got["AGENTID_STORAGE_DB_DSN"].ConfigKey, "storage.db.dsn")
	}
}

func TestEnvOverride_Apply_NonMatchingPrefix(t *testing.T) {
	e := DefaultEnvOverride()
	env := map[string]string{
		"PATH":                  "/usr/bin",
		"OTHER_VAR":             "ignored",
		"AGENTID_SERVICE_NAME":  "env-svc",
	}
	overrides := e.Apply(env)
	if len(overrides) != 1 {
		t.Errorf("Apply() got %d overrides, want 1", len(overrides))
	}
}

func TestEnvOverride_Apply_SkipList(t *testing.T) {
	e := EnvOverride{
		Prefix:  EnvPrefix,
		Delim:   EnvDelim,
		Skipped: []string{"AGENTID_SKIPPED"},
	}
	env := map[string]string{
		"AGENTID_SKIPPED":      "skip",
		"AGENTID_SERVICE_NAME": "env-svc",
	}
	overrides := e.Apply(env)
	if len(overrides) != 1 {
		t.Errorf("Apply() got %d overrides, want 1", len(overrides))
	}
	if overrides[0].EnvKey != "AGENTID_SERVICE_NAME" {
		t.Errorf("EnvKey = %q, want %q", overrides[0].EnvKey, "AGENTID_SERVICE_NAME")
	}
}

func TestInferType_Bool(t *testing.T) {
	tests := []struct {
		in   string
		want any
	}{
		{"true", true},
		{"false", false},
		{"TRUE", true},
		{"False", false},
	}
	for _, tt := range tests {
		got := inferType(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("inferType(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestInferType_Int(t *testing.T) {
	tests := []struct {
		in   string
		want any
	}{
		{"42", 42},
		{"0", 0},
		{"-10", -10},
	}
	for _, tt := range tests {
		got := inferType(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("inferType(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestInferType_Duration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"500ms", 500 * time.Millisecond},
	}
	for _, tt := range tests {
		got := inferType(tt.in)
		d, ok := got.(time.Duration)
		if !ok {
			t.Errorf("inferType(%q) = %v, want Duration", tt.in, got)
			continue
		}
		if d != tt.want {
			t.Errorf("inferType(%q) = %v, want %v", tt.in, d, tt.want)
		}
	}
}

func TestInferType_String(t *testing.T) {
	got := inferType("hello-world")
	if s, ok := got.(string); !ok || s != "hello-world" {
		t.Errorf("inferType(hello-world) = %v, want string", got)
	}
}

func TestListEnv(t *testing.T) {
	t.Setenv("AGENTID_TEST_VAR", "test-value")
	t.Setenv("AGENTID_OTHER", "other-value")
	t.Setenv("UNRELATED", "should-not-appear")

	envs := ListEnv("AGENTID_")
	if len(envs) != 2 {
		t.Errorf("ListEnv() got %d envs, want 2", len(envs))
	}
	if envs["AGENTID_TEST_VAR"] != "test-value" {
		t.Errorf("AGENTID_TEST_VAR = %q, want %q", envs["AGENTID_TEST_VAR"], "test-value")
	}
}

func TestExpandEnv(t *testing.T) {
	t.Setenv("AGENTID_TEST_HOST", "example.com")
	got := ExpandEnv("postgres://user@${AGENTID_TEST_HOST}:5432/db")
	want := "postgres://user@example.com:5432/db"
	if got != want {
		t.Errorf("ExpandEnv() = %q, want %q", got, want)
	}

	// 找不到的变量展开为空字符串（os.Expand 默认行为）
	got2 := ExpandEnv("value-${UNDEFINED_VAR}-end")
	want2 := "value--end"
	if got2 != want2 {
		t.Errorf("ExpandEnv(undefined) = %q, want %q", got2, want2)
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"SERVICE_NAME", "service.name"},
		{"service.name", "service.name"},
		{"Service-Name", "service-name"},
		{".SERVICE_NAME.", "service.name"},
		{"DB_DSN", "db.dsn"},
	}
	for _, tt := range tests {
		got := NormalizeKey(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOverride_String(t *testing.T) {
	o := Override{
		EnvKey:    "AGENTID_DB_DSN",
		ConfigKey: "db.dsn",
		RawValue:  "postgres://...",
		TypedValue: "postgres://...",
	}
	got := o.String()
	if got == "" {
		t.Error("Override.String() should not be empty")
	}
	if !contains(got, "AGENTID_DB_DSN") {
		t.Errorf("String() should contain env key, got %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
