//go:build integration

// Package backend contract test: OnchainBackend 远端 mock-chain 联调（P8.14）。
//
// 流程：
//  1. 启动 mock-chain HTTP server（cmd/mock-chain/main.go 程序）作为独立子进程
//  2. 通过 HTTPAdapter 调用其 /api/v1/agents/* 端点
//  3. 包装成 OnchainBackend
//  4. 端到端：注册 → 查询 → 升级 → 封禁 → 解封 → 注销
//
// 运行：
//
//	go test -tags=integration -run Contract ./core/backend/...
package backend

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	httpadapter "github.com/agentid-chain/agentid-chain/core/chain_adapter/http"
)

const contractTestAddrBase = "127.0.0.1:"

// startMockChain 启动 mock-chain 子进程（无 mock-chain 二进制则编译）。
func startMockChain(t *testing.T) (cleanup func(), baseURL string) {
	t.Helper()

	// 1. 找空闲端口（OS 分配：0 = let kernel pick）
	ln, err := net.Listen("tcp", contractTestAddrBase+"0")
	if err != nil {
		t.Skipf("no free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	// 2. 找 mock-chain 二进制（PATH 或 ./bin）
	binary, err := findMockChainBinary()
	if err != nil {
		t.Skipf("mock-chain binary not found: %v (run: go build -o ./bin/mock-chain ./cmd/mock-chain)", err)
	}

	// 3. 启动子进程
	cmd := exec.Command(binary, "-addr", addr, "-log-level", "error")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mock-chain: %v", err)
	}

	// 4. 等待 healthz 通过
	baseURL = "http://" + addr
	cleanup = func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		}
		_ = cmd.Wait()
	}
	if err := waitHealthy(baseURL+"/healthz", 5*time.Second); err != nil {
		cleanup()
		t.Skipf("mock-chain not healthy: %v", err)
	}
	return cleanup, baseURL
}

func findMockChainBinary() (string, error) {
	candidates := []string{
		filepath.Join("bin", "mock-chain"),
		filepath.Join("..", "..", "bin", "mock-chain"),
		filepath.Join(os.Getenv("GOPATH"), "bin", "mock-chain"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs, nil
		}
	}
	return "", fmt.Errorf("not in PATH or ./bin/")
}

func waitHealthy(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

// =============================================================================
// Contract Tests
// =============================================================================

func TestContract_OnchainBackend_WithMockChain_Register(t *testing.T) {
	cleanup, baseURL := startMockChain(t)
	defer cleanup()

	adapter := httpadapter.NewHTTPAdapter(baseURL)
	be, err := NewBackend(Config{Type: TypeOnchain, ChainAdapter: adapter})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	cred, err := be.RegisterAgent(ctx, &RegisterRequest{
		Owner: "alice", Level: 1, Permission: 0xFF, PublicKey: "pk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cred.UUID == "" {
		t.Error("empty uuid")
	}
	if cred.TxHash == "" {
		t.Error("empty tx hash")
	}
}

func TestContract_OnchainBackend_WithMockChain_FullLifecycle(t *testing.T) {
	cleanup, baseURL := startMockChain(t)
	defer cleanup()

	adapter := httpadapter.NewHTTPAdapter(baseURL)
	be, err := NewBackend(Config{Type: TypeOnchain, ChainAdapter: adapter})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	// 1. 注册
	cred, err := be.RegisterAgent(ctx, &RegisterRequest{
		Owner: "bob", Level: 1, Permission: 0xFF, PublicKey: "pk",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 查询
	info, err := be.GetAgentInfo(ctx, cred.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if info.State != StateActive {
		t.Errorf("state = %q, want active", info.State)
	}

	// 3. 升级
	if err := be.UpdateAgentLevel(ctx, cred.UUID, 2, "test upgrade"); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(ctx, cred.UUID)
	if info.Level != 2 {
		t.Errorf("level = %d, want 2", info.Level)
	}

	// 4. 封禁
	if err := be.BanAgent(ctx, cred.UUID, "policy"); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(ctx, cred.UUID)
	if info.State != StateBanned {
		t.Errorf("state = %q, want banned", info.State)
	}

	// 5. 解封
	if err := be.UnbanAgent(ctx, cred.UUID); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(ctx, cred.UUID)
	if info.State != StateActive {
		t.Errorf("state = %q, want active", info.State)
	}

	// 6. 注销
	if err := be.UnregisterAgent(ctx, cred.UUID); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(ctx, cred.UUID)
	if info.State != StateUnregistered {
		t.Errorf("state = %q, want unregistered", info.State)
	}
}

func TestContract_OnchainBackend_WithMockChain_NotFound(t *testing.T) {
	cleanup, baseURL := startMockChain(t)
	defer cleanup()

	adapter := httpadapter.NewHTTPAdapter(baseURL)
	be, _ := NewBackend(Config{Type: TypeOnchain, ChainAdapter: adapter})

	_, err := be.GetAgentInfo(context.Background(), "missing-uuid")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestContract_OnchainBackend_WithMockChain_BatchGet(t *testing.T) {
	cleanup, baseURL := startMockChain(t)
	defer cleanup()

	adapter := httpadapter.NewHTTPAdapter(baseURL)
	be, _ := NewBackend(Config{Type: TypeOnchain, ChainAdapter: adapter})

	ctx := context.Background()
	uuids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		cred, err := be.RegisterAgent(ctx, &RegisterRequest{
			Owner: "u", Level: 1, Permission: 0xFF, PublicKey: "pk",
		})
		if err != nil {
			t.Fatal(err)
		}
		uuids = append(uuids, cred.UUID)
	}

	infos, err := be.BatchGetAgentInfo(ctx, uuids)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 3 {
		t.Errorf("len = %d, want 3", len(infos))
	}
}

// =============================================================================
// 共享 http client（none required; tests use http.Client inline）
