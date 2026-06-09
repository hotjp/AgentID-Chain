package rbac

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

const sampleYAML = `
version: "1"
updated_at: "2026-06-08T10:00:00Z"
level_max_permissions:
  test: "0xFFFF"
  basic: "0xFFFFFFFF"
  advanced: "0xFFFFFFFFFFFF"
  pro: "0xFFFFFFFFFFFFFFFF"
  reserved4: "0xFFFFFFFFFFFFFFFF"
`

func TestParseLevels_OK(t *testing.T) {
	cfg, err := ParseLevels([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("ParseLevels: %v", err)
	}
	if cfg.Version != "1" {
		t.Errorf("Version = %q, want 1", cfg.Version)
	}
	if len(cfg.LevelMaxPermissions) != 5 {
		t.Errorf("got %d levels, want 5", len(cfg.LevelMaxPermissions))
	}
}

func TestParseLevels_NoLevels(t *testing.T) {
	_, err := ParseLevels([]byte(`version: "1"`))
	if err == nil {
		t.Fatal("expected error for missing level_max_permissions")
	}
	if !errors.Is(err, ErrLevelsConfigInvalid) {
		t.Errorf("err should wrap ErrLevelsConfigInvalid: %v", err)
	}
}

func TestParseLevels_InvalidYAML(t *testing.T) {
	_, err := ParseLevels([]byte("not: valid: yaml: :::"))
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestBuildTemplate_OK(t *testing.T) {
	cfg, err := ParseLevels([]byte(sampleYAML))
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := BuildTemplate(cfg)
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if tpl.Max(domain.LevelTest) != 0xFFFF {
		t.Errorf("LevelTest = %#x", tpl.Max(domain.LevelTest))
	}
	if tpl.Max(domain.LevelBasic) != 0xFFFFFFFF {
		t.Errorf("LevelBasic = %#x", tpl.Max(domain.LevelBasic))
	}
	if tpl.Max(domain.LevelPro) != 0xFFFFFFFFFFFFFFFF {
		t.Errorf("LevelPro = %#x", tpl.Max(domain.LevelPro))
	}
	// reserved5/6/7 未配置 → 走 domain 默认
	if tpl.Max(domain.LevelReserved5) != domain.LevelReserved5.DefaultMaxPermissions() {
		t.Errorf("LevelReserved5 fallback failed")
	}
}

func TestBuildTemplate_UnknownLevel(t *testing.T) {
	cfg := &LevelsConfig{
		LevelMaxPermissions: map[string]string{
			"unknown": "0xFF",
		},
	}
	_, err := BuildTemplate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidLevel) {
		t.Errorf("err should wrap ErrInvalidLevel: %v", err)
	}
}

func TestBuildTemplate_InvalidHex(t *testing.T) {
	cfg := &LevelsConfig{
		LevelMaxPermissions: map[string]string{
			"basic": "not-hex",
		},
	}
	_, err := BuildTemplate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildTemplate_NilConfig(t *testing.T) {
	_, err := BuildTemplate(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseHexMask(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"0", 0, true},
		{"0x0", 0, true},
		{"0X0", 0, true},
		{"FF", 0xFF, true},
		{"0xFF", 0xFF, true},
		{"0XFF", 0xFF, true},
		{"ffff", 0xFFFF, true},
		{"FFFFFFFFFFFFFFFF", 0xFFFFFFFFFFFFFFFF, true},
		{"0xFFFFFFFFFFFFFFFF", 0xFFFFFFFFFFFFFFFF, true},
		{"", 0, false},
		{"0x", 0, false},
		{"GG", 0, false},
		// 17 hex chars (超过 64 位)
		{"10000000000000000", 0, false},
	}
	for _, c := range cases {
		got, err := parseHexMask(c.in)
		if c.ok {
			if err != nil {
				t.Errorf("parseHexMask(%q) err=%v", c.in, err)
				continue
			}
			if got != c.want {
				t.Errorf("parseHexMask(%q) = %#x, want %#x", c.in, got, c.want)
			}
		} else {
			if err == nil {
				t.Errorf("parseHexMask(%q) expected error, got %#x", c.in, got)
			}
		}
	}
}

func TestParseLevelName(t *testing.T) {
	cases := []struct {
		in      string
		want    domain.LevelType
		ok      bool
	}{
		{"test", domain.LevelTest, true},
		{"basic", domain.LevelBasic, true},
		{"advanced", domain.LevelAdvanced, true},
		{"pro", domain.LevelPro, true},
		{"reserved4", domain.LevelReserved4, true},
		{"reserved5", domain.LevelReserved5, true},
		{"reserved6", domain.LevelReserved6, true},
		{"reserved7", domain.LevelReserved7, true},
		{"platform", domain.LevelReserved7, true},
		{"0", domain.LevelTest, true},
		{"1", domain.LevelBasic, true},
		{"7", domain.LevelReserved7, true},
		{"level3", domain.LevelPro, true},
		{"unknown", 0, false},
		{"", 0, false},
		{"8", 0, false},
	}
	for _, c := range cases {
		got, ok := parseLevelName(c.in)
		if ok != c.ok {
			t.Errorf("parseLevelName(%q) ok=%v, want %v", c.in, ok, c.ok)
			continue
		}
		if c.ok && got != c.want {
			t.Errorf("parseLevelName(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestLoadLevelsFromFile_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_level.yaml")
	if err := os.WriteFile(path, []byte(sampleYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadLevelsFromFile(path)
	if err != nil {
		t.Fatalf("LoadLevelsFromFile: %v", err)
	}
	if cfg.Version != "1" {
		t.Errorf("Version = %q", cfg.Version)
	}
}

func TestLoadLevelsFromFile_EmptyPath(t *testing.T) {
	_, err := LoadLevelsFromFile("")
	if !errors.Is(err, ErrEmptyPath) {
		t.Errorf("err = %v, want ErrEmptyPath", err)
	}
}

func TestLoadLevelsFromFile_NotExist(t *testing.T) {
	_, err := LoadLevelsFromFile("/nonexistent/path/agent_level.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewLoader_EmptyPath(t *testing.T) {
	l, err := NewLoader("")
	if err != nil {
		t.Fatal(err)
	}
	if l.Template() == nil {
		t.Fatal("nil template")
	}
	// 默认模板：LevelBasic → 0xFFFFFFFF
	if l.Template().Max(domain.LevelBasic) != domain.LevelBasic.DefaultMaxPermissions() {
		t.Error("default template mismatch")
	}
}

func TestNewLoader_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_level.yaml")
	if err := os.WriteFile(path, []byte(sampleYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := NewLoader(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.Path() != path {
		t.Errorf("Path = %q, want %q", l.Path(), path)
	}
	if l.Template().Max(domain.LevelBasic) != 0xFFFFFFFF {
		t.Errorf("template not loaded")
	}
}

func TestNewLoader_FileNotExist(t *testing.T) {
	_, err := NewLoader("/nonexistent/path/agent_level.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoader_Reload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_level.yaml")
	if err := os.WriteFile(path, []byte(sampleYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := NewLoader(path)
	if err != nil {
		t.Fatal(err)
	}
	loadedAt1 := l.LoadedAt()

	// 改文件
	time.Sleep(2 * time.Millisecond)
	newYAML := `
version: "2"
level_max_permissions:
  test: "0xFF"
  basic: "0xFFFF"
  advanced: "0xFFFFFFFF"
  pro: "0xFFFFFFFFFFFF"
`
	if err := os.WriteFile(path, []byte(newYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	if l.Template().Max(domain.LevelBasic) != 0xFFFF {
		t.Errorf("reload did not pick up new config")
	}
	if !l.LoadedAt().After(loadedAt1) {
		t.Error("LoadedAt not updated after Reload")
	}
}

func TestLoader_Reload_FailureKeepsOldTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_level.yaml")
	if err := os.WriteFile(path, []byte(sampleYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := NewLoader(path)
	if err != nil {
		t.Fatal(err)
	}
	oldTpl := l.Template()
	oldMax := oldTpl.Max(domain.LevelBasic)

	// 写一个非法 YAML 进去
	if err := os.WriteFile(path, []byte("not: valid: yaml: :::"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.Reload(); err == nil {
		t.Fatal("expected error")
	}
	// 旧 template 必须保留
	if l.Template().Max(domain.LevelBasic) != oldMax {
		t.Error("template should not change on Reload failure")
	}
}

func TestLoader_SetPath(t *testing.T) {
	l, err := NewLoader("")
	if err != nil {
		t.Fatal(err)
	}
	l.SetPath("/tmp/test.yaml")
	if l.Path() != "/tmp/test.yaml" {
		t.Errorf("Path = %q", l.Path())
	}
}

func TestLoader_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_level.yaml")
	if err := os.WriteFile(path, []byte(sampleYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := NewLoader(path)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = l.Template().Max(domain.LevelBasic)
			}
		}()
	}
	wg.Wait()
	// 仅验证不 race：r.Load 是只读
}
