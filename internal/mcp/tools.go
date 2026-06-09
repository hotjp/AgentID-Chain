// Package mcp AgentID-Chain 的 8 个 MCP 工具实现。
//
// 工具列表（与 docs §4.2 MCP 接入对齐）：
//   - agentid_register         注册新 Agent
//   - agentid_get_info         查询 Agent
//   - agentid_upgrade          升级 Level
//   - agentid_check_permission 校验 Permission
//   - agentid_audit_logs       审计日志
//   - agentid_batch_register   批量注册
//   - agentid_ban              封禁
//   - agentid_unban            解封
//
// 工具通过 Server.RegisterTool 注册；Handler 接受原始 JSON 参数，
// 反序列化为内部 DTO 后调用 backend.IdentityBackend。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentid-chain/agentid-chain/core/backend"
)

// =============================================================================
// 通用 DTO
// =============================================================================

// agentRef 通用 agent 引用参数。
type agentRef struct {
	UUID string `json:"uuid"`
}

// registerArgs 注册参数。
type registerArgs struct {
	Owner      string            `json:"owner"`
	Level      uint8             `json:"level"`
	Permission uint64            `json:"permission"`
	PublicKey  string            `json:"public_key"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// upgradeArgs 升级参数。
type upgradeArgs struct {
	UUID        string `json:"uuid"`
	TargetLevel uint8  `json:"target_level"`
	Reason      string `json:"reason,omitempty"`
}

// checkPermArgs 权限校验参数。
type checkPermArgs struct {
	UUID       string `json:"uuid"`
	Permission uint64 `json:"permission"`
}

// auditArgs 审计日志参数。
type auditArgs struct {
	UUID  string `json:"uuid"`
	Limit int    `json:"limit,omitempty"`
}

// batchArgs 批量注册参数。
type batchArgs struct {
	Items []registerArgs `json:"items"`
}

// banArgs 封禁参数。
type banArgs struct {
	UUID   string `json:"uuid"`
	Reason string `json:"reason,omitempty"`
}

// =============================================================================
// 工具注册
// =============================================================================

// RegisterAgentIDTools 把 8 个工具注册到 server。
//
// be 不能为 nil；底层调用 backend.IdentityBackend。
func RegisterAgentIDTools(s *Server, be backend.IdentityBackend) {
	// 1. agentid_register
	s.RegisterTool(
		"agentid_register",
		"Register a new agent. Returns credential with uuid, level, state, tx_hash.",
		objectSchema(map[string]string{
			"owner":      "string (DID, e.g. did:agentid:alice)",
			"level":      "integer 1-3",
			"permission": "integer bitmask (default 255)",
			"public_key": "string (base64)",
		}, []string{"owner", "level", "public_key"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a registerArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if a.Owner == "" || a.PublicKey == "" {
				return nil, fmt.Errorf("owner and public_key are required")
			}
			if a.Level == 0 {
				a.Level = 1
			}
			if a.Permission == 0 {
				a.Permission = 0xFF
			}
			return be.RegisterAgent(ctx, &backend.RegisterRequest{
				Owner:      a.Owner,
				Level:      a.Level,
				Permission: a.Permission,
				PublicKey:  a.PublicKey,
				Metadata:   a.Metadata,
			})
		},
	)

	// 2. agentid_get_info
	s.RegisterTool(
		"agentid_get_info",
		"Get agent full info by uuid.",
		objectSchema(map[string]string{
			"uuid": "string (uuid v7)",
		}, []string{"uuid"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a agentRef
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if a.UUID == "" {
				return nil, fmt.Errorf("uuid is required")
			}
			return be.GetAgentInfo(ctx, a.UUID)
		},
	)

	// 3. agentid_upgrade
	s.RegisterTool(
		"agentid_upgrade",
		"Upgrade agent level (newLevel must be > currentLevel).",
		objectSchema(map[string]string{
			"uuid":         "string (uuid v7)",
			"target_level": "integer 2-3",
			"reason":       "string (optional)",
		}, []string{"uuid", "target_level"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a upgradeArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if a.UUID == "" || a.TargetLevel == 0 {
				return nil, fmt.Errorf("uuid and target_level are required")
			}
			if err := be.UpdateAgentLevel(ctx, a.UUID, a.TargetLevel, a.Reason); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "uuid": a.UUID, "new_level": a.TargetLevel}, nil
		},
	)

	// 4. agentid_check_permission
	s.RegisterTool(
		"agentid_check_permission",
		"Check whether agent has all bits in the given permission bitmask.",
		objectSchema(map[string]string{
			"uuid":       "string (uuid v7)",
			"permission": "integer bitmask",
		}, []string{"uuid", "permission"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a checkPermArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if a.UUID == "" {
				return nil, fmt.Errorf("uuid is required")
			}
			info, err := be.GetAgentInfo(ctx, a.UUID)
			if err != nil {
				return nil, err
			}
			has := (info.Permission & a.Permission) == a.Permission
			return map[string]any{
				"uuid":          a.UUID,
				"required":      a.Permission,
				"actual":        info.Permission,
				"granted":       has,
				"missing_bits":  a.Permission &^ info.Permission,
			}, nil
		},
	)

	// 5. agentid_audit_logs
	s.RegisterTool(
		"agentid_audit_logs",
		"Get change logs (register/upgrade/ban/unban/unregister) for an agent.",
		objectSchema(map[string]string{
			"uuid":  "string (uuid v7)",
			"limit": "integer (optional, default 50)",
		}, []string{"uuid"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a auditArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if a.UUID == "" {
				return nil, fmt.Errorf("uuid is required")
			}
			logs, err := be.GetChangeLogs(ctx, a.UUID)
			if err != nil {
				return nil, err
			}
			if a.Limit > 0 && len(logs) > a.Limit {
				logs = logs[len(logs)-a.Limit:]
			}
			return map[string]any{"logs": logs, "count": len(logs)}, nil
		},
	)

	// 6. agentid_batch_register
	s.RegisterTool(
		"agentid_batch_register",
		"Register multiple agents in one call. Returns per-item credentials.",
		objectSchema(map[string]string{
			"items": "array of {owner,level,permission,public_key}",
		}, []string{"items"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a batchArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if len(a.Items) == 0 {
				return nil, fmt.Errorf("items must be non-empty")
			}
			if len(a.Items) > 100 {
				return nil, fmt.Errorf("items length must be <= 100")
			}
			results := make([]map[string]any, 0, len(a.Items))
			for i, it := range a.Items {
				if it.Level == 0 {
					it.Level = 1
				}
				if it.Permission == 0 {
					it.Permission = 0xFF
				}
				cred, err := be.RegisterAgent(ctx, &backend.RegisterRequest{
					Owner: it.Owner, Level: it.Level,
					Permission: it.Permission, PublicKey: it.PublicKey,
				})
				entry := map[string]any{"index": i, "owner": it.Owner}
				if err != nil {
					entry["error"] = err.Error()
				} else if cred != nil {
					entry["uuid"] = cred.UUID
					entry["state"] = cred.State
					entry["tx_hash"] = cred.TxHash
				}
				results = append(results, entry)
			}
			return map[string]any{"results": results, "count": len(results)}, nil
		},
	)

	// 7. agentid_ban
	s.RegisterTool(
		"agentid_ban",
		"Ban an agent (state -> banned; idempotent).",
		objectSchema(map[string]string{
			"uuid":   "string (uuid v7)",
			"reason": "string (optional, default 'policy')",
		}, []string{"uuid"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a banArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if a.UUID == "" {
				return nil, fmt.Errorf("uuid is required")
			}
			if a.Reason == "" {
				a.Reason = "policy"
			}
			if err := be.BanAgent(ctx, a.UUID, a.Reason); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "uuid": a.UUID, "action": "banned"}, nil
		},
	)

	// 8. agentid_unban
	s.RegisterTool(
		"agentid_unban",
		"Unban an agent (state banned -> active; idempotent).",
		objectSchema(map[string]string{
			"uuid": "string (uuid v7)",
		}, []string{"uuid"}),
		func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a agentRef
			if err := json.Unmarshal(raw, &a); err != nil {
				return nil, fmt.Errorf("invalid args: %w", err)
			}
			if a.UUID == "" {
				return nil, fmt.Errorf("uuid is required")
			}
			if err := be.UnbanAgent(ctx, a.UUID); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "uuid": a.UUID, "action": "unbanned"}, nil
		},
	)
}

// objectSchema 生成简单的 JSON Schema 对象。
func objectSchema(props map[string]string, required []string) any {
	schemaProps := make(map[string]any, len(props))
	for k, desc := range props {
		t := "string"
		// 简单启发式：含 "integer" 描述 → integer
		if containsAny(desc, "integer", "bitmask") {
			t = "integer"
		}
		schemaProps[k] = map[string]any{"type": t, "description": desc}
	}
	return map[string]any{
		"type":       "object",
		"properties": schemaProps,
		"required":   required,
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
