package gates

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// NewCoverageGate 构造覆盖率门控。
//
// 行为：
//   - 在 workdir 中执行 `go test -coverprofile=cover.out -covermode=atomic ./...`
//   - 用 `go tool cover -func=cover.out` 解析 total 覆盖率
//   - 与 threshold 比较；未达即返回错误
//
// 退出码策略：
//   - 工具缺失 / 编译错误 / 解析失败 → 错误（fail）
//   - 覆盖率达标 → 通过
//   - 覆盖率不足 → 错误（fail）
type CoverageGate struct {
	threshold float64
	pkgs      string
}

// NewCoverageGate 构造覆盖率门（threshold ∈ [0,1]）。
func NewCoverageGate(threshold float64) *CoverageGate {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.70
	}
	return &CoverageGate{threshold: threshold, pkgs: "./..."}
}

// NewCoverageGateWithPkgs 构造覆盖率门（自定义包范围）。
func NewCoverageGateWithPkgs(threshold float64, pkgs string) *CoverageGate {
	g := NewCoverageGate(threshold)
	if pkgs != "" {
		g.pkgs = pkgs
	}
	return g
}

func (g *CoverageGate) Name() string { return "coverage" }
func (g *CoverageGate) Severity() Severity {
	return SeverityNonNegotiable
}
func (g *CoverageGate) Required() bool { return true }
func (g *CoverageGate) Description() string {
	return fmt.Sprintf("Go test coverage must be ≥ %.0f%% (package scope)", g.threshold*100)
}

// Check 执行覆盖率检查。
func (g *CoverageGate) Check(ctx context.Context) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go toolchain not found in PATH: %w", err)
	}

	// Step 1: 生成 coverage profile
	if err := os.MkdirAll("build", 0o755); err != nil {
		return fmt.Errorf("create build dir: %w", err)
	}
	profile := filepath.Join("build", "coverage.out")
	args := []string{"test", "-coverprofile=" + profile, "-covermode=atomic", g.pkgs}
	cmd := exec.CommandContext(ctx, "go", args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		// 区分"无测试包"与"测试失败"
		if strings.Contains(errOut.String(), "no test files") {
			return fmt.Errorf("coverage gate failed: no test files in %s (need ≥%.0f%% but 0%% reported)", g.pkgs, g.threshold*100)
		}
		return fmt.Errorf("coverage gate failed during go test: %w\nstderr: %s", err, errOut.String())
	}

	// Step 2: 解析 total 覆盖率
	coverage, err := parseCoverageTotal(profile)
	if err != nil {
		return fmt.Errorf("coverage gate failed parsing: %w", err)
	}

	// Step 3: 比较
	if coverage < g.threshold {
		return fmt.Errorf("coverage %.2f%% < required %.2f%% (threshold)", coverage*100, g.threshold*100)
	}
	return nil
}

// parseCoverageTotal 从 coverage profile 中解析 total 百分比。
//
// 兼容两种输入：
//   - cover.out 原始 profile（含 "mode: atomic" 头）：调用 go tool cover -func
//   - go tool cover -func 输出文本（含 "total:		(statements)	XX.X%"）
var totalRe = regexp.MustCompile(`total:\s+\(statements\)\s+([\d.]+)%`)

func parseCoverageTotal(profile string) (float64, error) {
	// 优先：go tool cover -func（结构化输出）
	cmd := exec.Command("go", "tool", "cover", "-func="+profile)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		if m := totalRe.FindStringSubmatch(out.String()); len(m) == 2 {
			return strconv.ParseFloat(m[1], 64)
		}
	}

	// 回退：正则扫描原 profile 末尾
	data, err := readFile(profile)
	if err != nil {
		return 0, fmt.Errorf("read profile: %w", err)
	}
	if m := totalRe.FindStringSubmatch(string(data)); len(m) == 2 {
		return strconv.ParseFloat(m[1], 64)
	}
	return 0, fmt.Errorf("total coverage not found in %s", profile)
}
