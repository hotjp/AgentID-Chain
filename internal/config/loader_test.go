package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil cfg")
	}
	if cfg.Service.Name != "agentid-chain" {
		t.Errorf("Service.Name = %q, want %q", cfg.Service.Name, "agentid-chain")
	}
	if cfg.Backend.Type != "local" {
		t.Errorf("Backend.Type = %q, want %q", cfg.Backend.Type, "local")
	}
}

func TestLoad_YAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "test.yaml")
	content := []byte("service:\n  name: test-svc\n  http_addr: ':9999'\nbackend:\n  type: hybrid\n")
	if err := os.WriteFile(yamlPath, content, 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Service.Name != "test-svc" {
		t.Errorf("Service.Name = %q, want %q", cfg.Service.Name, "test-svc")
	}
	if cfg.Service.HTTPAddr != ":9999" {
		t.Errorf("Service.HTTPAddr = %q, want %q", cfg.Service.HTTPAddr, ":9999")
	}
	if cfg.Backend.Type != "hybrid" {
		t.Errorf("Backend.Type = %q, want %q", cfg.Backend.Type, "hybrid")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("AGENTID_SERVICE_NAME", "env-svc")
	t.Setenv("AGENTID_BACKEND_TYPE", "onchain")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Service.Name != "env-svc" {
		t.Errorf("Service.Name = %q, want %q", cfg.Service.Name, "env-svc")
	}
	if cfg.Backend.Type != "onchain" {
		t.Errorf("Backend.Type = %q, want %q", cfg.Backend.Type, "onchain")
	}
}

func TestLoad_EnvLowerUnderscore(t *testing.T) {
	// AGENTID_STORAGE_DB_DSN → storage.db.dsn
	t.Setenv("AGENTID_STORAGE_DB_DSN", "postgres://env:env@host:5432/db")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Storage.DB.DSN != "postgres://env:env@host:5432/db" {
		t.Errorf("Storage.DB.DSN = %q, want env-set", cfg.Storage.DB.DSN)
	}
}

func TestLoadString_Valid(t *testing.T) {
	resetKoanf()
	_ = koanfInstance()
	if err := LoadString("service.name=from-flag"); err != nil {
		t.Fatalf("LoadString() error = %v", err)
	}
}

func TestLoadString_Invalid(t *testing.T) {
	if err := LoadString("no-equals-sign"); err == nil {
		t.Fatal("LoadString() should return error for invalid input")
	}
}

func TestMustLoad_PanicsOnError(t *testing.T) {
	// MustLoad 应该 panic 当 YAML 解析失败时（无效的 YAML 内容）
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("invalid: yaml: : :"), 0o644); err != nil {
		t.Fatalf("write bad yaml: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustLoad should panic on bad YAML")
		}
	}()
	_ = MustLoad(badPath)
}

func TestSnapshot_NotEmpty(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	_ = cfg
	snap := Snapshot()
	if len(snap) == 0 {
		t.Fatal("Snapshot() returned empty map")
	}
}

func TestLoadFromEnv_OnlyEnv(t *testing.T) {
	t.Setenv("AGENTID_LOG_LEVEL", "debug")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestLoadWith_FlagOverrides(t *testing.T) {
	t.Setenv("AGENTID_SERVICE_NAME", "from-env")
	cfg, err := LoadWith("", []string{"service.name=from-flag"})
	if err != nil {
		t.Fatalf("LoadWith() error = %v", err)
	}
	// flag > env
	if cfg.Service.Name != "from-flag" {
		t.Errorf("Service.Name = %q, want %q", cfg.Service.Name, "from-flag")
	}
}
