# 用户旅程

> 5 个典型场景的端到端示例

## 📋 旅程索引

| 旅程 | 场景 | 难度 |
|------|------|------|
| [1. 个人开发者注册](#journey-1) | 单 Agent + 本地存储 | ⭐ |
| [2. 企业批量接入](#journey-2) | 1000+ Agent | ⭐⭐ |
| [3. LLM 通过 MCP 调用](#journey-3) | Claude / GPT 集成 | ⭐⭐ |
| [4. Agent 间互信调用](#journey-4) | A2A 完整流程 | ⭐⭐⭐ |
| [5. 链上审计验证](#journey-5) | hybrid 模式 | ⭐⭐ |

---

## Journey 1: 个人开发者注册

**场景**：Alice 想给自己的脚本分配一个稳定身份。

**前置**：
- 服务已启动
- CLI 已编译

**步骤**：

```bash
# 1. 一次性：AAP 鉴权
go run ./cmd/agentid auth init
# 生成 ~/.agentid/keys/{public,private}.b64
# 输出 JWT 到 ~/.agentid/jwt

# 2. 注册
go run ./cmd/agentid register \
  --owner alice \
  --level test \
  --backend local
# 输出: 0190a3b4-7c8d-7def-9abc-def012345678

# 3. 后续使用
go run ./cmd/agentid get 0190a3b4-7c8d-7def-9abc-def012345678
```

**耗时**：~ 2 min（含编译）

---

## Journey 2: 企业批量接入

**场景**：某公司需要为 1000 个内部微服务分配 Agent ID。

**前置**：
- 服务已启动
- CSV / YAML 列表

**步骤**：

```yaml
# agents.yaml
agents:
  - owner: team-a
    level: prod
    metadata:
      service: payment-api
      env: prod
  - owner: team-a
    level: prod
    metadata:
      service: order-api
      env: prod
  # ... 1000 行
```

```bash
# 批量注册
go run ./cmd/agentid batch \
  --file agents.yaml \
  --concurrency 10 \
  --output result.json
```

**输出**：
```json
[
  {"agent_id": "...", "owner": "team-a", "status": "active"},
  {"agent_id": "...", "owner": "team-a", "status": "active"},
  ...
]
```

**性能**：~ 11μs/register（见 [register-benchmark.md](../perf/register-benchmark.md)）
- 1000 Agent 串行：~ 11ms
- 1000 Agent 并发 10：~ 1ms（实际受限于 AAP）

**AAP 优化**：批量场景下，使用**长时 JWT**（如 24h），避免每请求都做 EdDSA 验签。

---

## Journey 3: LLM 通过 MCP 调用

**场景**：用户在 Claude Desktop 中通过自然语言管理 Agent。

**前置**：
- 服务已启动
- Claude Desktop 已安装

**步骤**：

#### 3.1 配置 MCP Server

`~/.config/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "agentid-chain": {
      "url": "http://localhost:8080/mcp/v1/rpc",
      "headers": {
        "Authorization": "Bearer <aap-jwt>"
      }
    }
  }
}
```

#### 3.2 启动 Claude Desktop

```
用户: 帮我列出 alice 的所有 test 等级 agent
```

Claude 内部：
```
→ tool_call: list_agents({ owner: "alice", level: "test" })
→ POST /mcp/v1/rpc
→ 返回 agents 列表
```

#### 3.3 用户执行操作

```
用户: 撤销 agent 0190a3b4-7c8d-7def-9abc-def012345678
```

Claude 内部：
```
→ tool_call: revoke_agent({ uuid: "...", reason: "user requested" })
→ POST /mcp/v1/rpc
→ 204 No Content
```

详见 [api/mcp.md](../api/mcp.md)

---

## Journey 4: Agent 间互信调用

**场景**：Agent A 需要调用 Agent B 的服务，且 B 需要验证 A 的身份。

**前置**：
- A 和 B 都有 Agent ID
- A 的 owner 有 AAP 权限

**步骤**：

```
┌─────────┐                ┌──────────────┐                ┌─────────┐
│ Agent A │                │ AgentID-     │                │ Agent B │
│         │                │ Chain        │                │         │
└────┬────┘                └──────┬───────┘                └────┬────┘
     │                            │                             │
     │  1. 协商 A2A Token          │                             │
     │───────────────────────────>│                             │
     │  { agent_id, scope }        │                             │
     │                             │                             │
     │  2. 颁发 Token + 临时密钥   │                             │
     │<───────────────────────────│                             │
     │  { token, public_key,       │                             │
     │    private_key_encrypted }  │                             │
     │                            │                             │
     │  3. 用 A2A 私钥签名请求     │                             │
     │  (Header: X-A2A-Token,      │                             │
     │   X-A2A-Signature)          │                             │
     │                                                       │
     │─────────────────────────────────────────────────────────>│
     │                                                       │
     │                                                       │ 4. 验签
     │                                                       │ 5. 检查
     │                                                       │    Redis
     │                                                       │    撤销集
     │                                                       │
     │<────────────────────────────────────────────────────────│
     │  6. 响应                                                │
     │                                                       │
```

**代码示例**：

A 端：
```go
// 1. 协商
tok, _ := a2a.Negotiate(apID, []string{"read"})

// 2. 解密 A2A 私钥
a2aPriv, _ := decryptPrivKey(tok.PrivateKeyEnc, aapJWT)

// 3. 签名请求
req := buildRequest(...)
sig := ed25519.Sign(a2aPriv, req.Body)
req.Header.Set("X-A2A-Token", tok.Token)
req.Header.Set("X-A2A-Signature", base64.StdEncoding.EncodeToString(sig))

http.DefaultClient.Do(req)
```

B 端：
```go
// 4. 验签
pub, _ := getPubKeyFromToken(tok.Token)  // 从 token 解析
sig, _ := base64.StdEncoding.DecodeString(req.Header.Get("X-A2A-Signature"))
if !ed25519.Verify(pub, req.Body, sig) {
    return 401
}

// 5. 检查撤销
if redis.SIsMember("a2a:revoked", extractJTI(tok.Token)) {
    return 401
}

// 6. 业务处理
return handle(req)
```

详见 [api/a2a.md](../api/a2a.md)

---

## Journey 5: 链上审计验证

**场景**：合规审计员需要验证某个 Agent 注册记录的真实性。

**前置**：
- hybrid 模式部署
- 链上合约已部署

**步骤**：

#### 5.1 用户查询（PG）

```bash
curl -s http://localhost:8080/v1/agents/$AGENT_ID \
  -H "Authorization: Bearer $AAP_TOKEN" | jq .
```

返回：
```json
{
  "agent_id": "...",
  "owner": "alice",
  "level": "test",
  "status": "active",
  "chain_tx_hash": "0xabc123..."  // 链上交易哈希
}
```

#### 5.2 审计员验证（链上）

```bash
# 1. 获取合约 ABI
# https://polygonscan.com/address/0xCONTRACT#code

# 2. 查询链上记录
cast call 0xCONTRACT "getAgent(bytes32)(address,uint8,uint256)" \
  0x$(echo $AGENT_ID | tr -d '-') \
  --rpc-url https://polygon-rpc.com
```

返回：
```
0xALICE_ADDRESS  1 (level=test)  1717938896 (timestamp)
```

#### 5.3 比对

| 字段 | PG (业务) | 链上 (审计) | 一致？ |
|------|-----------|------------|--------|
| owner | alice | 0xALICE... | ✅ (可解析) |
| level | test | 1 | ✅ |
| created_at | 2026-06-09T... | 1717938896 | ✅ |

**结论**：链上记录与业务库一致，不可篡改验证通过。

#### 5.4 历史变更追踪

```bash
# 链上事件
cast logs --rpc-url https://polygon-rpc.com \
  --from-block 0xLATEST \
  --address 0xCONTRACT \
  --topic "Registered(bytes32,address,uint8)"
```

返回所有 `Registered` 事件，可还原完整操作历史。

详见 [architecture/storage.md](../architecture/storage.md)
