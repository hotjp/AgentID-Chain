// Package gates: 项目质量门控执行器 (P23)。
//
// 实现 constitution.yaml 中声明的所有 NON_NEGOTIABLE / MANDATORY 门控。
// 每个 Gate 都有：
//   - Name: 唯一名称
//   - Type: NON_NEGOTIABLE | MANDATORY | CONFIGURABLE | BEST_PRACTICE
//   - Check(ctx) error: 实际执行检查
//   - Required: 是否阻塞流程
//
// 设计原则：
//   - 每个 Gate 独立可执行（go test ./internal/gates/...）
//   - 错误信息明确指出修复方法
//   - 支持 timeout 防止挂死
//   - 输出结构化结果（JSON / text）
package gates

import (
	"context"
	"fmt"
	"time"
)

// Severity 严重度。
type Severity string

const (
	SeverityNonNegotiable Severity = "NON_NEGOTIABLE"
	SeverityMandatory     Severity = "MANDATORY"
	SeverityConfigurable  Severity = "CONFIGURABLE"
	SeverityBestPractice  Severity = "BEST_PRACTICE"
)

// Result 单个 Gate 的执行结果。
type Result struct {
	Name      string        `json:"name"`
	Severity  Severity      `json:"severity"`
	Passed    bool          `json:"passed"`
	Required  bool          `json:"required"`
	Message   string        `json:"message"`
	Duration  time.Duration `json:"duration_ns"`
	Timestamp time.Time     `json:"timestamp"`
}

// Gate 单个质量门。
type Gate interface {
	Name() string
	Severity() Severity
	Description() string
	Required() bool
	Check(ctx context.Context) error
}

// Runner 批量执行 Gate 的引擎。
type Runner struct {
	gates   []Gate
	timeout time.Duration
}

// NewRunner 构造 Runner。
func NewRunner(gates []Gate, timeout time.Duration) *Runner {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Runner{gates: gates, timeout: timeout}
}

// RunAll 顺序执行所有 Gate；NON_NEGOTIABLE 失败立即返回。
func (r *Runner) RunAll(ctx context.Context) ([]Result, error) {
	results := make([]Result, 0, len(r.gates))
	for _, g := range r.gates {
		ctxGate, cancel := context.WithTimeout(ctx, r.timeout)
		start := time.Now()
		err := g.Check(ctxGate)
		cancel()

		res := Result{
			Name:      g.Name(),
			Severity:  g.Severity(),
			Required:  g.Required(),
			Passed:    err == nil,
			Duration:  time.Since(start),
			Timestamp: start,
		}
		if err != nil {
			res.Message = err.Error()
		} else {
			res.Message = "passed"
		}
		results = append(results, res)

		// NON_NEGOTIABLE 失败：fail-fast
		if err != nil && g.Severity() == SeverityNonNegotiable && g.Required() {
			return results, fmt.Errorf("NON_NEGOTIABLE gate %q failed: %w", g.Name(), err)
		}
	}
	return results, nil
}

// RunOne 执行单个 Gate。
func (r *Runner) RunOne(ctx context.Context, name string) (Result, error) {
	for _, g := range r.gates {
		if g.Name() == name {
			ctxGate, cancel := context.WithTimeout(ctx, r.timeout)
			defer cancel()
			start := time.Now()
			err := g.Check(ctxGate)
			res := Result{
				Name:      g.Name(),
				Severity:  g.Severity(),
				Required:  g.Required(),
				Passed:    err == nil,
				Duration:  time.Since(start),
				Timestamp: start,
			}
			if err != nil {
				res.Message = err.Error()
			}
			return res, err
		}
	}
	return Result{}, fmt.Errorf("gate %q not found", name)
}

// AllGates 返回注册的 Gate 列表（用于 introspection）。
func (r *Runner) AllGates() []string {
	names := make([]string, len(r.gates))
	for i, g := range r.gates {
		names[i] = g.Name()
	}
	return names
}

// Default 返回所有默认 Gate。
func Default() []Gate {
	return []Gate{
		NewCoverageGate(0.70),
		NewLintGate(),
		NewSecurityGate(),
		NewDockerBuildGate(),
		NewSchemaMigrationGate(),
		NewDocsCompletenessGate(),
		NewSkillValidationGate(),
	}
}