// Package main prompt 子命令实现。
//
// 用途：自然语言（NL）→ agentid 子命令的轻量路由器。
// 这是一个 deterministic pattern matcher（不依赖 LLM），用于 P9 阶段的入口。
// 真实 LLM 路由将在 P21（Agent Skills）阶段替换/扩展。
//
// 用法：
//
//	agentid prompt "register an agent for alice at level 2"
//	agentid prompt "ban agent <uuid> for spam"
//	agentid prompt "show info for <uuid>"
//
// 解析成功后，会把内部命令和参数打印到 stdout，可由上层包装器执行；
// 用 --exec 则在本进程内直接派发到对应 cobra 子命令。
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var promptFlagVals = struct {
	exec  bool
	dry   bool
	quiet bool
}{}

var promptCmdImpl = &cobra.Command{
	Use:   "prompt <text>",
	Short: "自然语言 → agentid 子命令的轻量路由器",
	Long: `把自然语言请求解析为 agentid 子命令（deterministic pattern matcher）。

支持的句式（示例）：
  "register an agent for alice"                   → register --owner did:agentid:alice --public-key pk_alice
  "register alice at level 2"                     → register --owner did:agentid:alice --level 2 --public-key pk_alice
  "info for <uuid>" / "show <uuid>"                → info --uuid <uuid>
  "audit <uuid>" / "logs of <uuid>"                → audit --uuid <uuid>
  "upgrade <uuid> to level 3"                     → upgrade --uuid <uuid> --target-level 3
  "ban <uuid> for <reason>"                       → ban --uuid <uuid> --reason <reason>
  "unban <uuid>"                                  → unban --uuid <uuid>
  "unregister <uuid>"                             → unregister --uuid <uuid>

Examples:
  agentid prompt "register an agent for alice at level 2"
  agentid prompt --exec "ban <uuid> for spam"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runPrompt,
}

func init() {
	promptCmdImpl.Flags().BoolVar(&promptFlagVals.exec, "exec", false, "在本进程内直接执行解析出的子命令")
	promptCmdImpl.Flags().BoolVar(&promptFlagVals.dry, "dry", false, "只解析不打印（隐含 --exec 时无效）")
	promptCmdImpl.Flags().BoolVar(&promptFlagVals.quiet, "quiet", false, "执行时抑制子命令输出")
}

var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func runPrompt(_ *cobra.Command, args []string) error {
	text := strings.Join(args, " ")
	cmd, parsed, err := parsePrompt(text)
	if err != nil {
		return err
	}

	if promptFlagVals.dry {
		fmt.Fprintf(os.Stdout, "%s %s\n", cmd, strings.Join(parsed, " "))
		return nil
	}
	if !promptFlagVals.exec {
		// 仅打印解析结果
		fmt.Fprintf(os.Stdout, "intent: %s\n", cmd)
		fmt.Fprintf(os.Stdout, "args:   %s\n", strings.Join(parsed, " "))
		fmt.Fprintln(os.Stdout, "use --exec to dispatch, or --dry to print the resolved command line")
		return nil
	}

	// --exec: 在 rootCmd 上调用对应子命令
	all := append([]string{cmd}, parsed...)
	rootCmd.SetArgs(all)
	if promptFlagVals.quiet {
		old := os.Stdout
		os.Stdout, _ = os.Open(os.DevNull)
		defer func() { os.Stdout = old }()
	}
	return rootCmd.Execute()
}

// parsePrompt 把 NL 文本解析为 (subcommand, args)。
func parsePrompt(text string) (string, []string, error) {
	low := strings.ToLower(text)
	uuid := ""
	if m := uuidRe.FindString(text); m != "" {
		uuid = m
	}
	// 提取 owner（did:agentid:<name>）
	ownerRe := regexp.MustCompile(`did:agentid:([A-Za-z0-9_\-]+)`)
	owner := ""
	if m := ownerRe.FindStringSubmatch(text); len(m) == 2 {
		owner = "did:agentid:" + m[1]
	}

	switch {
	case strings.Contains(low, "unregister") || strings.Contains(low, "deactivate"):
		if uuid == "" {
			return "", nil, fmt.Errorf("unregister: no uuid in input")
		}
		return "unregister", []string{"--uuid", uuid}, nil

	case strings.Contains(low, "register"):
		args := []string{}
		if owner != "" {
			args = append(args, "--owner", owner)
		} else if name := extractNameFor(low); name != "" {
			args = append(args, "--owner", "did:agentid:"+name)
		}
		if lvl := extractLevel(low); lvl > 0 {
			args = append(args, "--level", fmt.Sprintf("%d", lvl))
		}
		args = append(args, "--public-key", "pk_default")
		return "register", args, nil

	case strings.Contains(low, "info") || strings.HasPrefix(low, "show ") || strings.Contains(low, "get info"):
		if uuid == "" {
			return "", nil, fmt.Errorf("info: no uuid in input")
		}
		return "info", []string{"--uuid", uuid}, nil

	case strings.Contains(low, "audit") || strings.Contains(low, "log"):
		if uuid == "" {
			return "", nil, fmt.Errorf("audit: no uuid in input")
		}
		return "audit", []string{"--uuid", uuid}, nil

	case strings.Contains(low, "upgrade") || strings.Contains(low, "promote"):
		if uuid == "" {
			return "", nil, fmt.Errorf("upgrade: no uuid in input")
		}
		lvl := extractLevel(low)
		if lvl == 0 {
			lvl = 2
		}
		return "upgrade", []string{"--uuid", uuid, "--target-level", fmt.Sprintf("%d", lvl), "--reason", "prompt"}, nil

	case strings.Contains(low, "ban") && !strings.Contains(low, "unban"):
		if uuid == "" {
			return "", nil, fmt.Errorf("ban: no uuid in input")
		}
		reason := extractReason(text)
		if reason == "" {
			reason = "policy"
		}
		return "ban", []string{"--uuid", uuid, "--reason", reason}, nil

	case strings.Contains(low, "unban"):
		if uuid == "" {
			return "", nil, fmt.Errorf("unban: no uuid in input")
		}
		return "unban", []string{"--uuid", uuid}, nil
	}
	return "", nil, fmt.Errorf("no matching intent for: %q", text)
}

func extractNameFor(low string) string {
	// "register an agent for alice" / "register alice"
	for _, p := range []string{"for ", "agent "} {
		if i := strings.Index(low, p); i >= 0 {
			rest := low[i+len(p):]
			words := strings.Fields(rest)
			if len(words) > 0 {
				w := strings.Trim(words[0], " \t\n,.")
				return w
			}
		}
	}
	// "register alice ..."
	parts := strings.Fields(low)
	if len(parts) >= 2 && parts[0] == "register" {
		w := strings.Trim(parts[1], " \t\n,.")
		if !isStopWord(w) {
			return w
		}
	}
	return ""
}

func isStopWord(w string) bool {
	switch w {
	case "an", "a", "the", "agent", "new":
		return true
	}
	return false
}

func extractLevel(low string) int {
	// "at level N" / "to level N" / "level N"
	re := regexp.MustCompile(`level\s+(\d+)`)
	if m := re.FindStringSubmatch(low); len(m) == 2 {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		return n
	}
	return 0
}

func extractReason(text string) string {
	// "for <reason>" at end
	low := strings.ToLower(text)
	for _, p := range []string{"for ", "because "} {
		if i := strings.LastIndex(low, p); i >= 0 {
			rest := text[i+len(p):]
			rest = strings.TrimSpace(rest)
			rest = strings.Trim(rest, " \t\n,.\"'`")
			if rest != "" {
				return rest
			}
		}
	}
	return ""
}
