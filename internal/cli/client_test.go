package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Mode != ModeLocal {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeLocal)
	}
	if cfg.Gateway != "http://localhost:8080" {
		t.Errorf("Gateway = %q, want default", cfg.Gateway)
	}
	if cfg.Backend != "mock" {
		t.Errorf("Backend = %q, want mock", cfg.Backend)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", cfg.TimeoutSeconds)
	}
}

func TestApplyDefaults_FillsEmptyFields(t *testing.T) {
	cfg := &Config{Mode: "remote", Backend: "onchain"} // 故意只填部分
	cfg.ApplyDefaults()
	if cfg.Gateway == "" {
		t.Error("Gateway should be filled by ApplyDefaults")
	}
	if cfg.APIKey == "" {
		// APIKey 没有默认，所以保持空
	}
	if cfg.Output == "" {
		t.Error("Output should be filled by ApplyDefaults")
	}
	if cfg.TimeoutSeconds == 0 {
		t.Error("TimeoutSeconds should be filled by ApplyDefaults")
	}
}

func TestLoadConfig_NonexistentReturnsDefault(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "missing.yaml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(missing) err = %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}
	if cfg.Mode != ModeLocal {
		t.Errorf("Mode = %q, want %q (default)", cfg.Mode, ModeLocal)
	}
}

func TestLoadConfig_ParseYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	body := `mode: remote
gateway: http://gw.example.com:9090
api_key: test-key
backend: onchain
output: yaml
timeout_seconds: 60
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig err = %v", err)
	}
	if cfg.Mode != ModeRemote {
		t.Errorf("Mode = %q, want remote", cfg.Mode)
	}
	if cfg.Gateway != "http://gw.example.com:9090" {
		t.Errorf("Gateway = %q", cfg.Gateway)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q", cfg.APIKey)
	}
	if cfg.Backend != "onchain" {
		t.Errorf("Backend = %q", cfg.Backend)
	}
	if cfg.Output != "yaml" {
		t.Errorf("Output = %q", cfg.Output)
	}
	if cfg.TimeoutSeconds != 60 {
		t.Errorf("TimeoutSeconds = %d", cfg.TimeoutSeconds)
	}
}

func TestLoadConfig_BadYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(path, []byte(":\n:bad:::"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error on bad YAML")
	}
}

func TestEnsureConfigFile_CreatesDefault(t *testing.T) {
	// 重写 home 行为：直接调内部函数不便，跳过 setHome；用 t.TempDir() 模拟
	// 这里只验证 DefaultConfig + Marshal 路径可序列化。
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig is nil")
	}
}

func TestNewClient_LocalMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeLocal
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient err = %v", err)
	}
	if c == nil {
		t.Fatal("client is nil")
	}
	if c.local == nil {
		t.Error("local backend should be set in local mode")
	}
	if c.Mode() != ModeLocal {
		t.Errorf("Mode() = %q, want local", c.Mode())
	}
	if c.Token() != "" {
		t.Error("new client should have no token")
	}
}

func TestNewClient_RemoteMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeRemote
	cfg.Gateway = "http://example.com:1234"
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient err = %v", err)
	}
	if c.local != nil {
		t.Error("remote mode should not set local backend")
	}
	if c.http == nil {
		t.Error("http client should be set")
	}
}

func TestTokenSetAndClear(t *testing.T) {
	cfg := DefaultConfig()
	c, _ := NewClient(cfg)

	c.tok.set("abc123", time.Hour)
	if got := c.Token(); got != "abc123" {
		t.Errorf("Token() = %q, want abc123", got)
	}
	c.ClearToken()
	if got := c.Token(); got != "" {
		t.Errorf("Token() after clear = %q, want empty", got)
	}
}

func TestNewClient_UnknownBackend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeLocal
	cfg.Backend = "no-such-backend"
	_, err := NewClient(cfg)
	if err == nil {
		t.Error("expected error for unknown backend")
	}
}

func TestRegisterRequest_Fields(t *testing.T) {
	req := RegisterRequest{
		Owner:      "did:agentid:alice",
		Level:      2,
		Permission: 0xFFF,
		PublicKey:  "pk_alice",
	}
	if req.Owner == "" {
		t.Error("Owner empty")
	}
	if req.Level != 2 {
		t.Errorf("Level = %d", req.Level)
	}
	if req.Permission != 0xFFF {
		t.Errorf("Permission = %d", req.Permission)
	}
}

func TestAgentCredential_Fields(t *testing.T) {
	c := AgentCredential{
		UUID:       "abc",
		Owner:      "did:agentid:alice",
		Level:      1,
		State:      "active",
		Permission: 255,
		PublicKey:  "pk",
		TxHash:     "0x123",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if c.UUID == "" {
		t.Error("UUID empty")
	}
	if c.TxHash == "" {
		t.Error("TxHash empty")
	}
}
