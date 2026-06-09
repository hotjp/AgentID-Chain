package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentid-chain/agentid-chain/core/backend"
	"github.com/agentid-chain/agentid-chain/core/chain_adapter/mock"
)

// newBackendForTest 构造一个 mock backend（用于 tool handler 测试）。
func newBackendForTest() backend.IdentityBackend {
	be, _ := backend.NewBackend(backend.Config{Type: backend.TypeMock})
	return be
}

func TestRegisterAgentIDTools_AllRegistered(t *testing.T) {
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, newBackendForTest())

	want := []string{
		"agentid_register",
		"agentid_get_info",
		"agentid_upgrade",
		"agentid_check_permission",
		"agentid_audit_logs",
		"agentid_batch_register",
		"agentid_ban",
		"agentid_unban",
	}
	got := map[string]bool{}
	for _, tool := range s.Tools() {
		got[tool.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestTool_RegisterAndGetInfo(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	// 找 register handler
	var regH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_register" {
			regH = td.Handler
		}
	}
	if regH == nil {
		t.Fatal("register handler not found")
	}

	// 调 register
	args, _ := json.Marshal(map[string]any{
		"owner":      "did:agentid:alice",
		"level":      1,
		"public_key": "pk_alice",
	})
	out, err := regH(ctx, args)
	if err != nil {
		t.Fatalf("register err = %v", err)
	}
	credMap, ok := out.(*backend.AgentCredential)
	if !ok {
		t.Fatalf("register output type = %T", out)
	}
	uuid := credMap.UUID
	if uuid == "" {
		t.Fatal("empty uuid")
	}

	// 调 get_info
	var infoH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_get_info" {
			infoH = td.Handler
		}
	}
	infoArgs, _ := json.Marshal(map[string]any{"uuid": uuid})
	infoOut, err := infoH(ctx, infoArgs)
	if err != nil {
		t.Fatalf("get_info err = %v", err)
	}
	info, ok := infoOut.(*backend.AgentInfo)
	if !ok {
		t.Fatalf("info type = %T", infoOut)
	}
	if info.UUID != uuid {
		t.Errorf("info.UUID = %q, want %q", info.UUID, uuid)
	}
	if info.State != "active" {
		t.Errorf("info.State = %q, want active", info.State)
	}
}

func TestTool_Upgrade(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	// 先注册
	cred, _ := be.RegisterAgent(ctx, &backend.RegisterRequest{
		Owner: "did:agentid:bob", Level: 1, Permission: 0xFF, PublicKey: "pk_bob",
	})
	uuid := cred.UUID

	var upgradeH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_upgrade" {
			upgradeH = td.Handler
		}
	}
	args, _ := json.Marshal(map[string]any{"uuid": uuid, "target_level": 2, "reason": "test"})
	out, err := upgradeH(ctx, args)
	if err != nil {
		t.Fatalf("upgrade err = %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("upgrade output type = %T", out)
	}
	if m["ok"] != true {
		t.Errorf("upgrade result = %v", m)
	}
}

func TestTool_CheckPermission(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	cred, _ := be.RegisterAgent(ctx, &backend.RegisterRequest{
		Owner: "did:agentid:carol", Level: 1, Permission: 0xFF, PublicKey: "pk",
	})
	uuid := cred.UUID

	var permH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_check_permission" {
			permH = td.Handler
		}
	}

	// 有权限
	args, _ := json.Marshal(map[string]any{"uuid": uuid, "permission": 0x0F})
	out, err := permH(ctx, args)
	if err != nil {
		t.Fatalf("check_perm err = %v", err)
	}
	m := out.(map[string]any)
	if m["granted"] != true {
		t.Errorf("granted = %v, want true", m["granted"])
	}

	// 无权限（请求 bit 16 但 permission 是 255 = 0xFF）
	args, _ = json.Marshal(map[string]any{"uuid": uuid, "permission": uint64(1) << 16})
	out, err = permH(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	m = out.(map[string]any)
	if m["granted"] != false {
		t.Errorf("granted = %v, want false", m["granted"])
	}
}

func TestTool_BanUnban(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	cred, _ := be.RegisterAgent(ctx, &backend.RegisterRequest{
		Owner: "did:agentid:dave", Level: 1, Permission: 0xFF, PublicKey: "pk",
	})
	uuid := cred.UUID

	var banH, unbanH ToolHandler
	for _, td := range allToolHandlers(s) {
		switch td.Tool.Name {
		case "agentid_ban":
			banH = td.Handler
		case "agentid_unban":
			unbanH = td.Handler
		}
	}
	args, _ := json.Marshal(map[string]any{"uuid": uuid, "reason": "test"})
	if _, err := banH(ctx, args); err != nil {
		t.Fatal(err)
	}
	info, _ := be.GetAgentInfo(ctx, uuid)
	if info.State != "banned" {
		t.Errorf("after ban: state = %q, want banned", info.State)
	}

	unbanArgs, _ := json.Marshal(map[string]any{"uuid": uuid})
	if _, err := unbanH(ctx, unbanArgs); err != nil {
		t.Fatal(err)
	}
	info, _ = be.GetAgentInfo(ctx, uuid)
	if info.State != "active" {
		t.Errorf("after unban: state = %q, want active", info.State)
	}
}

func TestTool_AuditLogs(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	cred, _ := be.RegisterAgent(ctx, &backend.RegisterRequest{
		Owner: "did:agentid:eve", Level: 1, Permission: 0xFF, PublicKey: "pk",
	})
	uuid := cred.UUID
	_ = be.BanAgent(ctx, uuid, "test")

	var auditH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_audit_logs" {
			auditH = td.Handler
		}
	}
	args, _ := json.Marshal(map[string]any{"uuid": uuid, "limit": 10})
	out, err := auditH(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	logs := m["logs"].([]backend.ChangeLog)
	if len(logs) < 2 {
		t.Errorf("logs = %d, want >= 2 (register + ban)", len(logs))
	}
}

func TestTool_BatchRegister(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	var batchH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_batch_register" {
			batchH = td.Handler
		}
	}
	args, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"owner": "did:agentid:a1", "level": 1, "public_key": "pk1"},
			{"owner": "did:agentid:a2", "level": 1, "public_key": "pk2"},
		},
	})
	out, err := batchH(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	m := out.(map[string]any)
	if m["count"].(int) != 2 {
		t.Errorf("count = %v, want 2", m["count"])
	}
}

func TestTool_BatchRegister_Empty(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	var batchH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_batch_register" {
			batchH = td.Handler
		}
	}
	args, _ := json.Marshal(map[string]any{"items": []any{}})
	_, err := batchH(ctx, args)
	if err == nil {
		t.Error("expected error for empty items")
	}
	if !strings.Contains(err.Error(), "non-empty") {
		t.Errorf("err = %v", err)
	}
}

func TestTool_InvalidArgs(t *testing.T) {
	be := newBackendForTest()
	s := NewServer(ServerInfo{Name: "t"}, "")
	RegisterAgentIDTools(s, be)
	ctx := context.Background()

	var regH ToolHandler
	for _, td := range allToolHandlers(s) {
		if td.Tool.Name == "agentid_register" {
			regH = td.Handler
		}
	}
	// 缺 owner + public_key
	_, err := regH(ctx, []byte(`{"level":1}`))
	if err == nil {
		t.Error("expected error for missing required fields")
	}
}

// allToolHandlers 内部辅助：返回所有工具描述符。
func allToolHandlers(s *Server) []*ToolDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*ToolDescriptor, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t)
	}
	return out
}

// 避免 unused import（mock 包在新 backend 实例化时被引用）
var _ = mock.NewMockAdapter
