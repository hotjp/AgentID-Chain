// Package main prompt 子命令实现（基于 internal/prompt Pipeline）。
//
// 用途：自然语言（NL）→ agentid 子命令的路由器。
//
// 用法：
//
//	agentid prompt "register an agent for alice at level 2"
//	agentid prompt --exec "ban <uuid> for spam"
//	agentid prompt --format json "..."
//	agentid prompt --explain "..."            # 打印 intent + slots + cmd
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/agentid-chain/agentid-chain/internal/prompt"
	"github.com/spf13/cobra"
)

var promptFlagVals = struct {
	exec    bool
	dry     bool
	quiet   bool
	format  string
	explain bool
}{}

var promptCmdImpl = &cobra.Command{
	Use:   "prompt <text>",
	Short: "自然语言 → agentid 子命令路由器",
	Long: `把自然语言请求解析为 agentid 子命令（deterministic；不依赖 LLM）。

支持的 intent（6 类）：
  register / upgrade / query / batch / config / audit

Examples:
  agentid prompt "register an agent for alice at level 2"
  agentid prompt --exec "ban <uuid> for spam"
  agentid prompt --explain "audit <uuid> limit 10"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runPrompt,
}

func init() {
	promptCmdImpl.Flags().BoolVar(&promptFlagVals.exec, "exec", false, "在本进程内直接执行解析出的子命令")
	promptCmdImpl.Flags().BoolVar(&promptFlagVals.dry, "dry", false, "只解析不执行（打印 resolved cmd）")
	promptCmdImpl.Flags().BoolVar(&promptFlagVals.quiet, "quiet", false, "执行时抑制子命令输出")
	promptCmdImpl.Flags().StringVar(&promptFlagVals.format, "format", "", "输出格式 (json|table)")
	promptCmdImpl.Flags().BoolVar(&promptFlagVals.explain, "explain", false, "打印 intent + slots + cmd 的解释")
}

func runPrompt(_ *cobra.Command, args []string) error {
	text := strings.Join(args, " ")
	pipeline := prompt.NewPipeline()
	res, err := pipeline.Process(text)
	if err != nil {
		return fmt.Errorf("prompt: %w (intent=%s)", err, intentNameFromErr(pipeline, text))
	}

	if promptFlagVals.explain {
		return printExplain(res)
	}
	if promptFlagVals.dry {
		fmt.Fprintf(os.Stdout, "%s %s\n", res.Cmd, strings.Join(res.Args, " "))
		return nil
	}
	if !promptFlagVals.exec {
		return printExplain(res)
	}

	// 派发到 rootCmd
	all := append([]string{res.Cmd}, res.Args...)
	rootCmd.SetArgs(all)
	if promptFlagVals.quiet {
		old := os.Stdout
		os.Stdout, _ = os.Open(os.DevNull)
		defer func() { os.Stdout = old }()
	}
	return rootCmd.Execute()
}

func printExplain(res *prompt.Result) error {
	if promptFlagVals.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	fmt.Fprintf(os.Stdout, "intent: %s\n", res.Intent)
	fmt.Fprintf(os.Stdout, "score:  %.2f\n", res.Score)
	if len(res.Slots) > 0 {
		fmt.Fprintf(os.Stdout, "slots:\n")
		for k, v := range res.Slots {
			fmt.Fprintf(os.Stdout, "  %s = %s\n", k, v)
		}
	}
	fmt.Fprintf(os.Stdout, "cmd:    %s %s\n", res.Cmd, strings.Join(res.Args, " "))
	return nil
}

func intentNameFromErr(p *prompt.Pipeline, text string) string {
	intent, _ := p.Classifier.Confidence(text)
	return string(intent)
}
