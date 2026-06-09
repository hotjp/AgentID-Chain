package gates

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// NewLintGate 构造 golangci-lint 门控。
//
// 行为：
//   - 执行 `golangci-lint run --timeout=5m ./...`
//   - 工具缺失时返回明确错误（不静默通过）
//   - 任何 issue 视为失败
type LintGate struct {
	timeout string
}

// NewLintGate 构造 lint 门。
func NewLintGate() *LintGate {
	return &LintGate{timeout: "5m"}
}

func (g *LintGate) Name() string             { return "lint" }
func (g *LintGate) Severity() Severity       { return SeverityMandatory }
func (g *LintGate) Required() bool           { return true }
func (g *LintGate) Description() string      { return "golangci-lint must pass with zero issues" }

// Check 执行 lint 检查。
func (g *LintGate) Check(ctx context.Context) error {
	binary, err := exec.LookPath("golangci-lint")
	if err != nil {
		return fmt.Errorf("golangci-lint not installed: install via 'go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest'")
	}

	args := []string{"run", "--timeout=" + g.timeout, "./..."}
	cmd := exec.CommandContext(ctx, binary, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		// 截取前 50 行避免信息爆炸
		stderr := errOut.String()
		if len(stderr) > 4000 {
			stderr = stderr[:4000] + "\n... (truncated)"
		}
		return fmt.Errorf("golangci-lint failed: %w\n%s", err, stderr)
	}
	return nil
}

// helper: strings import guard
var _ = strings.TrimSpace
