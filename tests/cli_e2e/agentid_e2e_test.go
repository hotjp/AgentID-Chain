// Package cli_e2e 跑 agentid 二进制可执行文件的端到端测试。
//
// 策略：把当前仓库编译成 /tmp/agentid-bin，然后用 os/exec 跑它的子命令。
// 不依赖运行中的 gateway / 外部服务；用 Mode=local + Backend=mock。
//
// 用 `go test -tags e2e ./tests/cli_e2e/...` 触发；CI 在 P15 测试基础设施就位后接入。
//go:build e2e

package cli_e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildAgentID 编译当前仓库的 agentid 命令，返回二进制路径。
func buildAgentID(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "agentid")
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/agentid")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build agentid: %v", err)
	}
	return bin
}

func runAgentID(t *testing.T, bin, home string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestE2E_HelpAndVersion(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()

	stdout, _, err := runAgentID(t, bin, home, "--help")
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	for _, sub := range []string{"register", "info", "upgrade", "ban", "unban", "unregister", "batch-register", "audit", "migrate", "local", "config", "prompt", "aap", "version"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestE2E_Version(t *testing.T) {
	bin := buildAgentID(t)
	stdout, _, err := runAgentID(t, bin, t.TempDir(), "version")
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if !strings.HasPrefix(stdout, "agentid ") {
		t.Errorf("version output unexpected: %q", stdout)
	}
}

func TestE2E_LocalInit(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()

	stdout, _, err := runAgentID(t, bin, home, "local", "init", "--home", home)
	if err != nil {
		t.Fatalf("local init failed: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("ok = %v, want true", resp["ok"])
	}
	// 验证文件
	if _, err := os.Stat(filepath.Join(home, "config.yaml")); err != nil {
		t.Errorf("config.yaml not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "data")); err != nil {
		t.Errorf("data dir not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "outbox")); err != nil {
		t.Errorf("outbox dir not created: %v", err)
	}
}

func TestE2E_ConfigShowAndSet(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()

	// init
	if _, _, err := runAgentID(t, bin, home, "local", "init", "--home", home); err != nil {
		t.Fatalf("local init: %v", err)
	}

	// set mode
	cfgPath := filepath.Join(home, "config.yaml")
	if _, _, err := runAgentID(t, bin, home, "--config", cfgPath, "config", "set", "mode", "remote"); err != nil {
		t.Fatalf("config set: %v", err)
	}

	// show
	stdout, _, err := runAgentID(t, bin, home, "--config", cfgPath, "config", "show")
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	if !strings.Contains(stdout, "mode: remote") {
		t.Errorf("config show output: %q", stdout)
	}
}

func TestE2E_RegisterLocal(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()

	stdout, _, err := runAgentID(t, bin, home,
		"register",
		"--owner", "did:agentid:alice",
		"--level", "1",
		"--public-key", "pk_alice",
		"--format", "json",
	)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	var cred map[string]any
	if err := json.Unmarshal([]byte(stdout), &cred); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, stdout)
	}
	if uuid, _ := cred["uuid"].(string); uuid == "" {
		t.Errorf("no uuid in cred: %s", stdout)
	}
	if state, _ := cred["state"].(string); state != "active" {
		t.Errorf("state = %q, want active", state)
	}
}

func TestE2E_RegisterMissingArgs(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()
	_, _, err := runAgentID(t, bin, home, "register")
	if err == nil {
		t.Error("expected error for missing required flags")
	}
}

func TestE2E_PromptParse(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()

	stdout, _, err := runAgentID(t, bin, home, "prompt", "register an agent for bob at level 2")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}
	if !strings.Contains(stdout, "intent: register") {
		t.Errorf("prompt output: %q", stdout)
	}
	if !strings.Contains(stdout, "did:agentid:bob") {
		t.Errorf("prompt output missing owner: %q", stdout)
	}
}

func TestE2E_PromptExec(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()

	stdout, _, err := runAgentID(t, bin, home,
		"prompt", "--exec", "register an agent for carol at level 3",
	)
	if err != nil {
		t.Fatalf("prompt --exec failed: %v", err)
	}
	var cred map[string]any
	if err := json.Unmarshal([]byte(stdout), &cred); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, stdout)
	}
	if owner, _ := cred["owner"].(string); owner != "did:agentid:carol" {
		t.Errorf("owner = %q, want did:agentid:carol", owner)
	}
}

func TestE2E_BatchRegister(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()

	// 准备 CSV
	csvPath := filepath.Join(t.TempDir(), "agents.csv")
	csvBody := "owner,level,permission,public_key\n" +
		"did:agentid:u1,1,255,pk1\n" +
		"did:agentid:u2,2,4095,pk2\n"
	if err := os.WriteFile(csvPath, []byte(csvBody), 0o600); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "creds.json")
	stdout, stderr, err := runAgentID(t, bin, home,
		"batch-register", "--file", csvPath, "--output", outPath, "--format", "json",
	)
	if err != nil {
		t.Fatalf("batch-register failed: %v\nstderr=%s", err, stderr)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var results []map[string]any
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, string(data))
	}
	if len(results) != 2 {
		t.Errorf("results = %d, want 2", len(results))
	}
	if !strings.Contains(stdout, "ok=2") && !strings.Contains(stderr, "ok=2") {
		t.Errorf("no ok=2 summary in output")
	}
}

func TestE2E_AuditMissingUUID(t *testing.T) {
	bin := buildAgentID(t)
	home := t.TempDir()
	_, _, err := runAgentID(t, bin, home, "audit")
	if err == nil {
		t.Error("expected error for missing --uuid")
	}
}
