// Command constitution-gates 执行 AgentID-Chain 项目的质量宪法门控。
//
// 用法：
//
//	constitution-gates                       # 执行所有默认门
//	constitution-gates -gate coverage        # 仅执行单个门
//	constitution-gates -format json          # JSON 输出
//	constitution-gates -threshold 0.80       # 自定义覆盖率阈值
//	constitution-gates -timeout 10m          # 单门超时
//	constitution-gates -workdir .            # 工作目录
//
// 退出码：
//	0 — 全部通过
//	1 — 任一必须门失败
//	2 — NON_NEGOTIABLE 门失败
//	3 — 工具缺失 / 不可恢复错误
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/gates"
)

func main() {
	var (
		gateName  = flag.String("gate", "", "run only this gate (empty = all)")
		format    = flag.String("format", "text", "output format: text|json")
		threshold = flag.Float64("threshold", 0.70, "coverage threshold (0-1)")
		timeout   = flag.Duration("timeout", 5*time.Minute, "per-gate timeout")
		workdir   = flag.String("workdir", "", "working directory (default: current)")
		verbose   = flag.Bool("v", false, "verbose output")
	)
	flag.Parse()

	if *workdir != "" {
		if err := os.Chdir(*workdir); err != nil {
			fmt.Fprintf(os.Stderr, "chdir: %v\n", err)
			os.Exit(3)
		}
	}

	// 构造门列表
	all := []gates.Gate{
		gates.NewCoverageGate(*threshold),
		gates.NewLintGate(),
		gates.NewSecurityGate(),
		gates.NewDockerBuildGate(),
		gates.NewSchemaMigrationGate(),
		gates.NewDocsCompletenessGate(),
		gates.NewSkillValidationGate(),
	}

	selected := all
	if *gateName != "" {
		selected = nil
		for _, g := range all {
			if g.Name() == *gateName {
				selected = append(selected, g)
				break
			}
		}
		if len(selected) == 0 {
			fmt.Fprintf(os.Stderr, "gate %q not found (available: %v)\n", *gateName, gateNames(all))
			os.Exit(3)
		}
	}

	// 上下文：响应信号
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	runner := gates.NewRunner(selected, *timeout)
	results, err := runner.RunAll(ctx)

	// 输出
	if *format == "json" {
		out, _ := json.MarshalIndent(struct {
			Timestamp time.Time      `json:"timestamp"`
			Workdir   string        `json:"workdir"`
			Results   []gates.Result `json:"results"`
			Error     string        `json:"error,omitempty"`
		}{
			Timestamp: time.Now(),
			Workdir:   mustGetwd(),
			Results:   results,
			Error:     errMsg(err),
		}, "", "  ")
		fmt.Println(string(out))
	} else {
		printText(results, err, *verbose)
	}

	// 退出码
	if err != nil {
		os.Exit(2) // NON_NEGOTIABLE failed
	}
	for _, r := range results {
		if r.Required && !r.Passed {
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func printText(results []gates.Result, err error, verbose bool) {
	fmt.Println("┌─ Constitution Gates ─────────────────────────────────────")
	for _, r := range results {
		icon := "✅"
		status := "PASS"
		if !r.Passed {
			icon = "❌"
			status = "FAIL"
		} else if !r.Required {
			icon = "⚠️ "
			status = "SKIP"
		}
		fmt.Printf("│ %s [%s] %-22s %s  %s\n",
			icon, r.Severity, r.Name, status, r.Duration.Round(time.Millisecond))
		if verbose || !r.Passed {
			if r.Message != "" && r.Message != "passed" {
				fmt.Printf("│     └─ %s\n", r.Message)
			}
		}
	}
	fmt.Println("└──────────────────────────────────────────────────────────")
	if err != nil {
		fmt.Printf("\n❌ Runner error: %v\n", err)
	} else {
		failed := 0
		for _, r := range results {
			if !r.Passed && r.Required {
				failed++
			}
		}
		if failed == 0 {
			fmt.Println("\n✅ All required gates passed.")
		} else {
			fmt.Printf("\n❌ %d required gate(s) failed.\n", failed)
		}
	}
}

func gateNames(gs []gates.Gate) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Name()
	}
	return out
}

func mustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
