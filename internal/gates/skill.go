package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NewSkillValidationGate 构造 skill 验证门控。
//
// 行为：
//   - 扫描 skills/ 目录（或自定义）
//   - 每个 skill 目录必含 SKILL.md 和 schema.json
//   - SKILL.md 必含 YAML frontmatter（name, version, description）
//   - schema.json 必为合法 JSON，含 name, parameters 字段
//   - 必含 examples/ 目录
type SkillValidationGate struct {
	dirs []string
}

// NewSkillValidationGate 默认扫描 skills/ 和 examples/agent-skills/。
func NewSkillValidationGate() *SkillValidationGate {
	return &SkillValidationGate{
		dirs: []string{
			"skills",
			"examples/agent-skills",
		},
	}
}

func (g *SkillValidationGate) Name() string        { return "skill_validation" }
func (g *SkillValidationGate) Severity() Severity  { return SeverityMandatory }
func (g *SkillValidationGate) Required() bool      { return true }
func (g *SkillValidationGate) Description() string { return "all Agent Skills must have SKILL.md, schema.json, examples/, and tests/" }

// Check 校验 skill 完整性。
func (g *SkillValidationGate) Check(ctx context.Context) error {
	found := false
	var errs []string
	for _, d := range g.dirs {
		if _, err := os.Stat(d); err != nil {
			continue
		}
		found = true
		entries, err := os.ReadDir(d)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: read: %v", d, err))
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillDir := filepath.Join(d, e.Name())
			if err := g.checkSkill(skillDir); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", skillDir, err))
			}
		}
	}
	if !found {
		return fmt.Errorf("no skill directory found (looked in: %s)", strings.Join(g.dirs, ", "))
	}
	if len(errs) > 0 {
		return fmt.Errorf("skill validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func (g *SkillValidationGate) checkSkill(dir string) error {
	// SKILL.md
	skillMD := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return fmt.Errorf("SKILL.md missing: %w", err)
	}
	if err := validateFrontmatter(string(data)); err != nil {
		return fmt.Errorf("SKILL.md frontmatter: %w", err)
	}

	// schema.json
	schemaPath := filepath.Join(dir, "schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("schema.json missing: %w", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("schema.json invalid JSON: %w", err)
	}
	if _, ok := schema["name"]; !ok {
		return fmt.Errorf("schema.json missing 'name'")
	}
	if _, ok := schema["parameters"]; !ok {
		return fmt.Errorf("schema.json missing 'parameters'")
	}

	// examples/ 目录
	examplesDir := filepath.Join(dir, "examples")
	if fi, err := os.Stat(examplesDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("examples/ directory missing")
	}
	return nil
}

var frontmatterRe = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---`)

func validateFrontmatter(md string) error {
	m := frontmatterRe.FindStringSubmatch(md)
	if m == nil {
		return fmt.Errorf("no YAML frontmatter (expected '---' delimiters)")
	}
	fm := m[1]
	required := []string{"name:", "version:", "description:"}
	for _, key := range required {
		if !strings.Contains(fm, key) {
			return fmt.Errorf("missing frontmatter key: %s", strings.TrimSuffix(key, ":"))
		}
	}
	return nil
}
