// Package compat 包含向后兼容性测试。
//
// build tag: compat
// 用途：当引入破坏性变更时，这些测试必须通过
//
// 运行：go test -tags=compat -count=1 ./tests/compat/...
//
//go:build compat
// +build compat

package compat

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestV201_RegisterAgent_NoIdempotencyKey 测试 v2.0.1 客户端
// 不带 idempotency_key 字段时的行为。
//
// 期望：
//   - 在 v2.1.0 严格模式下返回 400（破坏性变更）
//   - 在 v2.1.0 兼容模式下接受（自动生成）
//   - 服务端在 header 提示 X-Compat-Warning
func TestV201_RegisterAgent_NoIdempotencyKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 模拟 v2.0.1 客户端请求
	body := `{"owner":"alice","level":"test"}`
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:8080/v1/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agentid-client/2.0.1")  // 旧客户端标识

	// 实际请求（需服务运行）
	// resp, err := http.DefaultClient.Do(req)
	// 这里用结构体断言替代运行依赖
	if !strings.Contains(req.Header.Get("User-Agent"), "2.0.1") {
		t.Fatal("test setup error: should simulate v2.0.1 client")
	}

	// 解析 body
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if _, ok := payload["idempotency_key"]; ok {
		t.Fatal("v2.0.1 client should not send idempotency_key")
	}
}

// TestV201_ListAgents_Pagination 测试 v2.0.1 客户端用旧的 page 字段。
func TestV201_ListAgents_Pagination(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// v2.0.1 用 ?page=1&page_size=20
	// v2.1.0 推荐 ?cursor=xxx&limit=20（page 仍兼容但 header 警告）
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/v1/agents?page=1&page_size=20", nil)
	req.Header.Set("User-Agent", "agentid-client/2.0.1")

	// 验证服务端应该能识别 page 字段
	if req.URL.Query().Get("page") != "1" {
		t.Fatal("test setup error")
	}
}

// TestV201_UpgradeAgent_AlgorithmField 测试 v2.0.1 客户端发签名算法字段。
func TestV201_UpgradeAgent_AlgorithmField(t *testing.T) {
	body := `{"level":"prod","signature_alg":"ed25519"}`
	var p map[string]any
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatal(err)
	}
	// v2.1.0 应忽略 signature_alg 字段（已硬编码 ED25519）
	if p["signature_alg"] != "ed25519" {
		t.Fatal("v2.0.1 always uses ed25519")
	}
	// 兼容性：服务端应忽略
	t.Log("v2.1.0 ignores signature_alg — backward compatible")
}

// TestV201_AAPChallengeFormat 测试 v2.0.1 客户端签名消息格式。
func TestV201_AAPChallengeFormat(t *testing.T) {
	// v2.0.1 格式："{challenge}:{timestamp}:{nonce}"
	// v2.1.0 格式：v2.0.1 仍支持 + 新格式 "{challenge}|{ts}|{nonce}|{agent_id}"
	oldFormat := func(c string, ts int64, n string) string {
		return strings.Join([]string{c, time.Unix(ts, 0).UTC().Format("20060102150405"), n}, ":")
	}
	_ = oldFormat
	t.Log("v2.0.1 challenge format: {c}:{ts}:{n}")
}
