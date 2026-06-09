package gates

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubGate 用于测试 Runner 行为。
type stubGate struct {
	name     string
	severity Severity
	required bool
	err      error
}

func (s *stubGate) Name() string        { return s.name }
func (s *stubGate) Severity() Severity  { return s.severity }
func (s *stubGate) Required() bool      { return s.required }
func (s *stubGate) Description() string { return "stub" }
func (s *stubGate) Check(ctx context.Context) error { return s.err }

func TestRunner_RunAll_AllPass(t *testing.T) {
	gates := []Gate{
		&stubGate{name: "a", severity: SeverityMandatory, required: true, err: nil},
		&stubGate{name: "b", severity: SeverityBestPractice, required: false, err: nil},
	}
	r := NewRunner(gates, 5*time.Second)
	results, err := r.RunAll(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("gate %q should pass", r.Name)
		}
	}
}

func TestRunner_RunAll_FailFastOnNonNegotiable(t *testing.T) {
	gates := []Gate{
		&stubGate{name: "a", severity: SeverityMandatory, required: true, err: nil},
		&stubGate{name: "b", severity: SeverityNonNegotiable, required: true, err: os.ErrNotExist},
		&stubGate{name: "c", severity: SeverityMandatory, required: true, err: nil},
	}
	r := NewRunner(gates, 5*time.Second)
	results, err := r.RunAll(context.Background())
	if err == nil {
		t.Fatal("expected error from NON_NEGOTIABLE failure, got nil")
	}
	if !strings.Contains(err.Error(), "NON_NEGOTIABLE") {
		t.Errorf("error should mention NON_NEGOTIABLE: %v", err)
	}
	// fail-fast：c 不应被执行
	if len(results) != 2 {
		t.Errorf("expected 2 results (fail-fast), got %d", len(results))
	}
}

func TestRunner_RunAll_ContinueOnMandatory(t *testing.T) {
	gates := []Gate{
		&stubGate{name: "a", severity: SeverityMandatory, required: true, err: os.ErrInvalid},
		&stubGate{name: "b", severity: SeverityMandatory, required: true, err: nil},
	}
	r := NewRunner(gates, 5*time.Second)
	results, err := r.RunAll(context.Background())
	if err != nil {
		t.Fatalf("MANDATORY failure should not error from Runner: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].Passed {
		t.Error("a should be failed")
	}
	if !results[1].Passed {
		t.Error("b should be passed")
	}
}

func TestRunner_RunOne_NotFound(t *testing.T) {
	r := NewRunner(nil, 5*time.Second)
	_, err := r.RunOne(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error for missing gate")
	}
}

func TestRunner_RunOne_Found(t *testing.T) {
	gates := []Gate{
		&stubGate{name: "x", severity: SeverityMandatory, required: true, err: nil},
	}
	r := NewRunner(gates, 5*time.Second)
	res, err := r.RunOne(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Name != "x" || !res.Passed {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestRunner_AllGates(t *testing.T) {
	gates := []Gate{
		&stubGate{name: "a"},
		&stubGate{name: "b"},
	}
	r := NewRunner(gates, 5*time.Second)
	names := r.AllGates()
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("AllGates() = %v", names)
	}
}

func TestRunner_Default(t *testing.T) {
	gates := Default()
	if len(gates) < 5 {
		t.Errorf("Default() returned %d gates, expected ≥ 5", len(gates))
	}
	names := make(map[string]bool)
	for _, g := range gates {
		names[g.Name()] = true
	}
	for _, want := range []string{"coverage", "lint", "security", "docker_build", "schema_migration", "docs_completeness", "skill_validation"} {
		if !names[want] {
			t.Errorf("Default() missing gate: %s", want)
		}
	}
}

func TestRunner_Timeout(t *testing.T) {
	slow := &slowGate{}
	r := NewRunner([]Gate{slow}, 50*time.Millisecond)
	results, err := r.RunAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}
	if results[0].Passed {
		t.Error("slow gate should have failed due to timeout")
	}
	if !strings.Contains(results[0].Message, "context") && !strings.Contains(results[0].Message, "deadline") {
		t.Logf("timeout message: %s", results[0].Message)
	}
}

type slowGate struct{}

func (s *slowGate) Name() string        { return "slow" }
func (s *slowGate) Severity() Severity  { return SeverityMandatory }
func (s *slowGate) Required() bool      { return true }
func (s *slowGate) Description() string { return "always times out" }
func (s *slowGate) Check(ctx context.Context) error {
	select {
	case <-time.After(500 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestCoverageGate_ThresholdBounds(t *testing.T) {
	// 负数 / >1 → 默认 0.70
	g := NewCoverageGate(-1)
	if g.threshold != 0.70 {
		t.Errorf("expected 0.70, got %f", g.threshold)
	}
	g = NewCoverageGate(2.0)
	if g.threshold != 0.70 {
		t.Errorf("expected 0.70, got %f", g.threshold)
	}
	g = NewCoverageGate(0.85)
	if g.threshold != 0.85 {
		t.Errorf("expected 0.85, got %f", g.threshold)
	}
}

func TestCoverageGate_Description(t *testing.T) {
	g := NewCoverageGate(0.80)
	desc := g.Description()
	if !strings.Contains(desc, "80") {
		t.Errorf("Description should contain 80: %s", desc)
	}
}

func TestLintGate_MissingTool(t *testing.T) {
	// 不假设 golangci-lint 是否安装；如未安装则返回特定错误
	g := NewLintGate()
	err := g.Check(context.Background())
	if err != nil {
		if !strings.Contains(err.Error(), "golangci-lint") {
			t.Errorf("error should mention golangci-lint: %v", err)
		}
	}
}

func TestSecurityGate_Severity(t *testing.T) {
	g := NewSecurityGate()
	if g.Severity() != SeverityMandatory {
		t.Errorf("expected MANDATORY, got %s", g.Severity())
	}
}

func TestDockerBuildGate_Severity(t *testing.T) {
	g := NewDockerBuildGate()
	if g.Severity() != SeverityMandatory {
		t.Errorf("expected MANDATORY, got %s", g.Severity())
	}
}

func TestSchemaMigrationGate_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	g := &SchemaMigrationGate{dirs: []string{dir}}
	err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for empty migration dir")
	}
}

func TestSchemaMigrationGate_Paired(t *testing.T) {
	dir := t.TempDir()
	must := func(p, content string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("001_init.up.sql", "CREATE TABLE x (id INT);")
	must("001_init.down.sql", "DROP TABLE x;")
	must("002_add_y.up.sql", "ALTER TABLE x ADD COLUMN y INT;")
	must("002_add_y.down.sql", "ALTER TABLE x DROP COLUMN y;")

	g := &SchemaMigrationGate{dirs: []string{dir}}
	if err := g.Check(context.Background()); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestSchemaMigrationGate_MissingDown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_init.up.sql"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &SchemaMigrationGate{dirs: []string{dir}}
	err := g.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "down") {
		t.Errorf("expected missing down error, got %v", err)
	}
}

func TestSchemaMigrationGate_GapInVersions(t *testing.T) {
	dir := t.TempDir()
	must := func(p string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("001_init.up.sql")
	must("001_init.down.sql")
	must("003_skip.up.sql") // gap at 002
	must("003_skip.down.sql")

	g := &SchemaMigrationGate{dirs: []string{dir}}
	err := g.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "gap") {
		t.Errorf("expected gap error, got %v", err)
	}
}

func TestSkillValidationGate_Valid(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	sk := `---
name: demo
version: 1.0.0
description: a demo skill
---

# demo
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(sk), 0o644); err != nil {
		t.Fatal(err)
	}
	schema := `{"name":"demo","parameters":{"type":"object"}}`
	if err := os.WriteFile(filepath.Join(skillDir, "schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &SkillValidationGate{dirs: []string{dir}}
	if err := g.Check(context.Background()); err != nil {
		t.Errorf("expected pass, got %v", err)
	}
}

func TestSkillValidationGate_MissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# no frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "schema.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := &SkillValidationGate{dirs: []string{dir}}
	err := g.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Errorf("expected frontmatter error, got %v", err)
	}
}

func TestSkillValidationGate_InvalidSchema(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	sk := `---
name: d
version: 1
description: x
---
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(sk), 0o644)
	os.WriteFile(filepath.Join(skillDir, "schema.json"), []byte("not json"), 0o644)

	g := &SkillValidationGate{dirs: []string{dir}}
	err := g.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected JSON error, got %v", err)
	}
}

func TestDocsCompletenessGate_RequiredFiles(t *testing.T) {
	// 临时目录中无任何文件 → 应失败
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldwd)

	g := NewDocsCompletenessGate()
	err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for missing docs")
	}
	if !strings.Contains(err.Error(), "README.md") {
		t.Errorf("error should mention README.md: %v", err)
	}
}

func TestDocsCompletenessGate_NoDocsDir(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldwd)
	os.WriteFile("README.md", []byte("# Test\n## 快速开始\nfoo"), 0o644)
	os.WriteFile("CHANGELOG.md", []byte("# Changelog"), 0o644)
	os.WriteFile("LICENSE", []byte("Apache-2.0"), 0o644)

	g := NewDocsCompletenessGate()
	err := g.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for missing docs/")
	}
}

func TestValidateFrontmatter(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "---\nname: a\nversion: 1\ndescription: x\n---\nbody", false},
		{"no fence", "name: a\nversion: 1\ndescription: x\n", true},
		{"missing desc", "---\nname: a\nversion: 1\n---\n", true},
		{"empty", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateFrontmatter(c.input)
			if (err != nil) != c.wantErr {
				t.Errorf("validateFrontmatter err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

func TestHasSection(t *testing.T) {
	if !hasSection("# T\n## 快速开始\nfoo", "快速开始|Quick Start") {
		t.Error("should match 快速开始")
	}
	if hasSection("# T\n## 其他\n", "快速开始|Quick Start") {
		t.Error("should not match")
	}
}

func TestParseCoverageTotal_BadFile(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.out")
	os.WriteFile(bad, []byte("garbage"), 0o644)
	_, err := parseCoverageTotal(bad)
	if err == nil {
		t.Error("expected error for garbage file")
	}
}
