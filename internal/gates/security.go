package gates

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// NewSecurityGate 构造安全扫描门控。
//
// 行为：
//   - 依次执行 gosec 和 govulncheck
//   - 任一工具未安装 → 错误（不静默）
//   - 任一发现高危问题 → 错误
type SecurityGate struct {
	strict bool
}

// NewSecurityGate 构造 security 门。
func NewSecurityGate() *SecurityGate {
	return &SecurityGate{strict: true}
}

func (g *SecurityGate) Name() string        { return "security" }
func (g *SecurityGate) Severity() Severity  { return SeverityMandatory }
func (g *SecurityGate) Required() bool      { return true }
func (g *SecurityGate) Description() string { return "gosec + govulncheck must pass with no high-severity findings" }

// Check 执行安全扫描。
func (g *SecurityGate) Check(ctx context.Context) error {
	if err := g.runGosec(ctx); err != nil {
		return err
	}
	if err := g.runGovulncheck(ctx); err != nil {
		return err
	}
	return nil
}

func (g *SecurityGate) runGosec(ctx context.Context) error {
	binary, err := exec.LookPath("gosec")
	if err != nil {
		return fmt.Errorf("gosec not installed: 'go install github.com/securego/gosec/v2/cmd/gosec@latest'")
	}
	// -no-fail=false: 任何 issue 都退出非零
	// -severity=high: 只看高危
	cmd := exec.CommandContext(ctx, binary, "-no-fail=false", "-severity=high", "-quiet", "./...")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gosec found high-severity issues: %w\n%s", err, truncate(errOut.String(), 3000))
	}
	return nil
}

func (g *SecurityGate) runGovulncheck(ctx context.Context) error {
	binary, err := exec.LookPath("govulncheck")
	if err != nil {
		return fmt.Errorf("govulncheck not installed: 'go install golang.org/x/vuln/cmd/govulncheck@latest'")
	}
	// -mode=source: 静态扫描源码调用链
	cmd := exec.CommandContext(ctx, binary, "-mode=source", "./...")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("govulncheck found vulnerabilities: %w\n%s", err, truncate(errOut.String(), 3000))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n... (truncated)"
}
