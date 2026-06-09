package gates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// NewSchemaMigrationGate 构造 schema migration 门控。
//
// 行为：
//   - 扫描 migrations/ 或 ent/migrate/migrations/ 目录
//   - 验证文件命名：NNN_name.up.sql / NNN_name.down.sql 配对
//   - 验证版本号严格递增
//   - 验证 down 与 up 一一对应
type SchemaMigrationGate struct {
	dirs []string
}

// NewSchemaMigrationGate 默认扫描项目常见 migration 目录。
func NewSchemaMigrationGate() *SchemaMigrationGate {
	return &SchemaMigrationGate{
		dirs: []string{
			"migrations",
			"ent/migrate/migrations",
			"db/migrations",
		},
	}
}

func (g *SchemaMigrationGate) Name() string        { return "schema_migration" }
func (g *SchemaMigrationGate) Severity() Severity  { return SeverityMandatory }
func (g *SchemaMigrationGate) Required() bool      { return true }
func (g *SchemaMigrationGate) Description() string { return "schema migrations must be well-formed, ordered, and paired (up/down)" }

// Check 校验 migration 文件。
func (g *SchemaMigrationGate) Check(ctx context.Context) error {
	// 至少应存在一个 migration 目录
	found := false
	var errs []string
	for _, d := range g.dirs {
		if _, err := os.Stat(d); err != nil {
			continue
		}
		found = true
		if err := g.checkDir(d); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", d, err))
		}
	}
	if !found {
		return fmt.Errorf("no migration directory found (looked in: %s)", strings.Join(g.dirs, ", "))
	}
	if len(errs) > 0 {
		return fmt.Errorf("migration validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

var migrationRe = regexp.MustCompile(`^(\d{3,})_([a-z0-9_]+)\.(up|down)\.sql$`)

func (g *SchemaMigrationGate) checkDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	type pair struct {
		version int
		name    string
		up      string
		down    string
	}
	pairs := map[int]*pair{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		ver, _ := strconv.Atoi(m[1])
		name := m[2]
		kind := m[3]
		p, ok := pairs[ver]
		if !ok {
			p = &pair{version: ver, name: name}
			pairs[ver] = p
		}
		if p.name != name {
			return fmt.Errorf("version %03d name mismatch: %q vs %q", ver, p.name, name)
		}
		switch kind {
		case "up":
			p.up = e.Name()
		case "down":
			p.down = e.Name()
		}
	}

	if len(pairs) == 0 {
		return fmt.Errorf("no valid migration files in %s (expected NNN_name.up.sql/.down.sql)", dir)
	}

	versions := make([]int, 0, len(pairs))
	for v := range pairs {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	// 顺序检查
	for i := 1; i < len(versions); i++ {
		if versions[i] != versions[i-1]+1 {
			return fmt.Errorf("non-sequential versions: %03d → %03d (gap)", versions[i-1], versions[i])
		}
	}

	// 配对检查
	for _, v := range versions {
		p := pairs[v]
		if p.up == "" {
			return fmt.Errorf("version %03d missing .up.sql", v)
		}
		if p.down == "" {
			return fmt.Errorf("version %03d missing .down.sql", v)
		}
	}
	return nil
}

// helper
var _ = filepath.Base
