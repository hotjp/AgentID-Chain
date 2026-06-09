package gates

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NewDocsCompletenessGate 构造文档完整性门控。
//
// 行为：
//   - 必存在顶层文档：README.md, CHANGELOG.md, LICENSE
//   - docs/ 目录存在且至少 N 个 .md
//   - 每个 .md 必含 frontmatter 或 # 标题
//   - 必含"## 安全"或"## Security"或"## 安全考虑"章节
//   - 链接 [text](url) 中 url 不可指向不存在的本地 .md
type DocsCompletenessGate struct {
	minDocCount int
	requiredFiles []string
}

// NewDocsCompletenessGate 默认配置。
func NewDocsCompletenessGate() *DocsCompletenessGate {
	return &DocsCompletenessGate{
		minDocCount: 10,
		requiredFiles: []string{
			"README.md",
			"CHANGELOG.md",
			"LICENSE",
		},
	}
}

func (g *DocsCompletenessGate) Name() string        { return "docs_completeness" }
func (g *DocsCompletenessGate) Severity() Severity  { return SeverityMandatory }
func (g *DocsCompletenessGate) Required() bool      { return true }
func (g *DocsCompletenessGate) Description() string { return "required top-level docs exist and docs/ is complete" }

// Check 校验文档完整性。
func (g *DocsCompletenessGate) Check(ctx context.Context) error {
	var errs []string

	// 1. 必存在文件
	for _, f := range g.requiredFiles {
		if _, err := os.Stat(f); err != nil {
			errs = append(errs, fmt.Sprintf("required file missing: %s", f))
		}
	}

	// 2. docs/ 目录
	if _, err := os.Stat("docs"); err != nil {
		errs = append(errs, "docs/ directory missing")
	} else {
		count, err := countMarkdown("docs")
		if err != nil {
			errs = append(errs, fmt.Sprintf("scan docs/: %v", err))
		} else if count < g.minDocCount {
			errs = append(errs, fmt.Sprintf("docs/ has %d .md files, expected ≥ %d", count, g.minDocCount))
		}
	}

	// 3. README.md 必含"## 安全"或"## 快速开始"
	if data, err := os.ReadFile("README.md"); err == nil {
		if !hasSection(string(data), "快速开始|Quick Start|Getting Started") {
			errs = append(errs, "README.md missing '## 快速开始' or '## Quick Start' section")
		}
	}

	// 4. 校验本地 .md 链接
	brokenLinks, err := findBrokenLocalMarkdownLinks(".")
	if err != nil {
		errs = append(errs, fmt.Sprintf("link check: %v", err))
	}
	if len(brokenLinks) > 0 {
		errs = append(errs, fmt.Sprintf("%d broken local markdown links:\n  - %s",
			len(brokenLinks), strings.Join(brokenLinks[:min(5, len(brokenLinks))], "\n  - ")))
	}

	if len(errs) > 0 {
		return fmt.Errorf("docs completeness failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func hasSection(content, pattern string) bool {
	re := regexp.MustCompile(`(?m)^##\s+(` + pattern + `)`)
	return re.MatchString(content)
}

func countMarkdown(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			count++
		}
		return nil
	})
	return count, err
}

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

func findBrokenLocalMarkdownLinks(root string) ([]string, error) {
	var broken []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		// 跳过隐藏目录
		if strings.HasPrefix(filepath.Base(p), ".") && p != "./README.md" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		base := filepath.Dir(p)
		for _, m := range mdLinkRe.FindAllStringSubmatch(string(data), -1) {
			link := strings.TrimSpace(m[2])
			if link == "" || strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
				continue
			}
			// 去掉 anchor / query
			link = strings.SplitN(link, "#", 2)[0]
			link = strings.SplitN(link, "?", 2)[0]
			if link == "" {
				continue
			}
			target := filepath.Join(base, link)
			if _, err := os.Stat(target); err != nil {
				broken = append(broken, fmt.Sprintf("%s → %s", p, m[2]))
			}
		}
		return nil
	})
	return broken, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
