# AgentID-Chain 技术文档 v2.0

> **文档定位**：面向开发者的全量技术方案，支持"给其他 Agent 直接开发"的认知对齐水平。  
> **核心变更**：v2.0 引入**混合存储架构**（链上/链下可配置）与**四种接入范式**（CLI / MCP / A2A / Prompt），实现同一身份体系下的多模态部署与跨 Agent 互认。

---

## 1. 项目概述

### 1.1 设计目标

AgentID-Chain 是专为 AI Agent 生态设计的分布式身份与权限网关。v2.0 版本在保持原有区块链可信存证能力的基础上，引入**可配置的后端存储模式**与**多协议接入层**，使同一套身份体系既能以公链/联盟链模式运行（开放生态），也能以纯本地/企业内模式运行（内部闭环），同时支持 CLI、MCP、A2A、Prompt 四种交互范式接入。

### 1.2 核心能力矩阵

| 能力维度 | v1.0 状态 | v2.0 状态 |
|---|---|---|
| 身份容量 | 百亿级 UUID | **万亿级 UUID**，架构层面无冲突 |
| 存储后端 | 仅区块链 | **链上/链下可配置**，企业内可零链部署 |
| Agent 体系 | 固定 4 级（含爬虫） | **可配置模板**，默认测试/普通两级，自由扩展 |
| 准入控制 | 反向验证 | **Agent 准入协议 + MoltCaptcha**，人类/爬虫永久拦截 |
| 接入方式 | HTTP API | **CLI / MCP / A2A / Prompt** 四种范式 |
| 互认协议 | 无 | **A2A 协议原生支持**，同体系 Agent 自动互信 |

### 1.3 适用场景

- **开放生态**：对接 Polygon/BSC 等公链，支撑跨平台 Agent 身份互认
- **企业内网**：配置为纯本地存储，零区块链依赖，内部 Agent 集群管理
- **混合模式**：核心身份链上存证，高频权限链下缓存，兼顾安全与性能
- **Agent 工具链**：通过 MCP 协议嵌入 Cursor/Cline/Claude Desktop 等工具
- **自动化脚本**：通过 CLI 完成批量注册、权限调整、审计导出
- **Prompt 工程**：通过自然语言配置完成身份初始化与权限申请

---

## 2. 整体架构

### 2.1 逻辑架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    接入层（Access Layer）                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │  CLI    │  │  MCP    │  │  A2A    │  │  Prompt Parser  │ │
│  │ Client  │  │ Server  │  │ Agent   │  │  (NL→Config)    │ │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────────┬────────┘ │
│       └─────────────┴─────────────┴────────────────┘          │
│                         │                                    │
│              ┌──────────▼──────────┐                       │
│              │   网关 & 路由分流     │                       │
│              │  UA拦截/限流/准入协议   │                       │
│              └──────────┬──────────┘                       │
└───────────────────────────┼─────────────────────────────────┘
                          │
┌───────────────────────────▼─────────────────────────────────┐
│                   核心服务层（Core Layer）                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  UUID 生成   │  │  MoltCaptcha│  │   Identity Backend  │  │
│  │  v4/v7 双模  │  │  & 准入协议  │  │   (存储抽象层)       │  │
│  └─────────────┘  └─────────────┘  └──────────┬──────────┘  │
│                                               │            │
│                          ┌────────────────────┼────────┐   │
│                          ▼                    ▼        ▼   │
│                   ┌────────────┐      ┌────────────┐      │
│                   │  链上后端   │      │  链下后端   │      │
│                   │ ChainAdapter│      │ LocalBackend│      │
│                   │  FISCO/Poly │      │ SQLite/Redis│      │
│                   └────────────┘      └────────────┘      │
│                                               │            │
│                              ┌────────────────┼────────┐   │
│                              ▼                ▼        ▼   │
│                       ┌──────────┐      ┌──────────┐       │
│                       │ 智能合约  │      │ 本地权限  │       │
│                       │ 身份&权限 │      │ 引擎&RBAC │       │
│                       └──────────┘      └──────────┘       │
└─────────────────────────────────────────────────────────────┘
                          │
┌───────────────────────────▼─────────────────────────────────┐
│                   数据层（Data Layer）                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  链上存证   │  │  链下缓存   │  │   审计日志库         │  │
│  │  核心身份   │  │  Redis/MySQL│  │   SQLite/PostgreSQL │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 模块职责

| 模块 | 职责 | 关键接口/文件 |
|---|---|---|
| **CLI Client** | 命令行工具，支持注册、查询、升级、导出 | `cmd/agentid/main.go` |
| **MCP Server** | 暴露 MCP 标准工具集，供 IDE/Agent 调用 | `mcp/server.go` |
| **A2A Agent** | A2A 协议端点，处理 Agent 间身份互认请求 | `a2a/handler.go` |
| **Prompt Parser** | 自然语言解析器，将需求转为配置指令 | `prompt/parser.go` |
| **网关层** | 流量拆分、UA 拦截、Agent 准入协议校验 | `gateway/` |
| **UUID 生成器** | 分布式 v4/v7 生成，全局唯一性保障 | `core/uuid_generator/` |
| **验证引擎** | MoltCaptcha + Agent 行为指纹双重校验 | `core/captcha/` |
| **Identity Backend** | 存储后端抽象，链上/链下统一接口 | `core/backend/` |
| **链适配器** | 多链插拔实现 | `core/chain_adapter/` |
| **本地后端** | 纯本地身份与权限管理 | `core/local_backend/` |
| **权限引擎** | RBAC + 位掩码 + 分级限流 | `core/auth/` |
| **缓存层** | Redis 权限缓存与失效策略 | `cache/` |

---

## 3. 核心设计详解

### 3.1 混合存储架构：链上/链下可配置

v2.0 引入 `IdentityBackend` 统一抽象接口，所有身份操作均通过该接口下发，业务层完全无感知存储后端类型。

#### 3.1.1 统一抽象接口

```go
// IdentityBackend 存储后端统一接口
// 链上/链下均实现此接口，业务层无感知切换
type IdentityBackend interface {
    // 注册 Agent，返回 UUID 与凭证
    RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error)

    // 查询 Agent 身份与权限
    GetAgentInfo(ctx context.Context, uuid string) (*AgentInfo, error)

    // 更新 Agent 等级/权限
    UpdateAgentLevel(ctx context.Context, uuid string, newLevel uint8, reason string) error

    // 封禁/解封 Agent
    BanAgent(ctx context.Context, uuid string, reason string) error
    UnbanAgent(ctx context.Context, uuid string) error

    // 注销 Agent
    UnregisterAgent(ctx context.Context, uuid string) error

    // 查询变更日志（审计）
    GetChangeLogs(ctx context.Context, uuid string) ([]ChangeLog, error)

    // 批量查询（用于 A2A 互认批量校验）
    BatchGetAgentInfo(ctx context.Context, uuids []string) (map[string]*AgentInfo, error)

    // 后端类型标识
    BackendType() BackendType // "onchain" | "local"
}
```

#### 3.1.2 链上后端（OnchainBackend）

- **实现文件**：`core/backend/onchain.go`
- **职责**：通过 `ChainAdapter` 调用智能合约，所有变更永久存证
- **适用场景**：开放生态、跨机构互信、需要审计追溯的场景
- **依赖**：区块链节点、合约地址、私钥管理

```go
type OnchainBackend struct {
    adapter chain_adapter.BaseChainAdapter
    cache   *cache.RedisCache
}

func (ob *OnchainBackend) RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
    // 1. 生成 UUID
    uuid := uuid_generator.GenerateV7()

    // 2. 调用链适配器上链注册
    txHash, err := ob.adapter.RegisterAgent(uuid, req.Owner, req.InitLevel, req.Permission)
    if err != nil {
        return nil, fmt.Errorf("onchain register failed: %w", err)
    }

    // 3. 生成链上签名凭证
    cred := &AgentCredential{
        UUID:      uuid,
        TxHash:    txHash,
        Signature: ob.signCredential(uuid, txHash),
    }

    // 4. 写入缓存加速
    ob.cache.SetAgentInfo(ctx, uuid, req.InitLevel, 15*time.Minute)

    return cred, nil
}
```

#### 3.1.3 链下后端（LocalBackend）

- **实现文件**：`core/backend/local.go`
- **职责**：基于 SQLite + Redis 的纯本地身份管理，零区块链依赖
- **适用场景**：企业内部 Agent 集群、开发测试、隐私合规要求高的场景
- **特性**：
  - 数据存储于本地 SQLite，支持加密存储
  - 权限变更通过本地 RBAC 引擎执行
  - 审计日志写入本地 PostgreSQL/SQLite，支持导出
  - 与链上后端保持**接口级兼容**，未来可无损迁移上链

```go
type LocalBackend struct {
    db    *sql.DB      // SQLite/PostgreSQL
    cache *cache.RedisCache
    rbac  *auth.RBACEngine
}

func (lb *LocalBackend) RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
    // 1. 生成 UUID
    uuid := uuid_generator.GenerateV7()

    // 2. 本地数据库写入身份记录
    _, err := lb.db.ExecContext(ctx, 
        `INSERT INTO agents (uuid, owner, level, permission, status, created_at) 
         VALUES (?, ?, ?, ?, ?, ?)`,
        uuid, req.Owner, req.InitLevel, req.Permission, 0, time.Now().Unix())
    if err != nil {
        return nil, fmt.Errorf("local register failed: %w", err)
    }

    // 3. 本地 RBAC 初始化权限
    lb.rbac.Grant(uuid, req.InitLevel, req.Permission)

    // 4. 生成本地凭证（无链上签名，使用 HMAC-SHA256）
    cred := &AgentCredential{
        UUID:      uuid,
        Signature: lb.signLocalCredential(uuid),
    }

    // 5. 写入缓存
    lb.cache.SetAgentInfo(ctx, uuid, req.InitLevel, 15*time.Minute)

    return cred, nil
}
```

#### 3.1.4 配置化切换

通过 `config/backend.yaml` 动态指定存储后端，无需修改业务代码：

```yaml
# 存储后端配置
backend:
  type: local           # 可选：onchain | local | hybrid

  # 当 type=onchain 或 hybrid 时生效
  onchain:
    driver: fisco-bcos  # 可选：fabric / polygon / bsc / mock
    rpc_url: "http://127.0.0.1:8545"
    contract_address: "0x..."
    operator_private_key: "${PRIVATE_KEY}"  # 支持环境变量注入
    gas_limit: 300000

  # 当 type=local 或 hybrid 时生效
  local:
    db_driver: sqlite     # 可选：sqlite / postgres / mysql
    dsn: "file:agentid.db?cache=shared&_fk=1"
    encrypt_at_rest: true   # 静态加密
    encryption_key: "${ENCRYPTION_KEY}"

  # 混合模式：核心身份上链，高频权限本地缓存
  hybrid:
    identity_onchain: true      # 注册/注销/等级变更上链
    permission_local: true       # 权限查询/限流走本地
    sync_interval: "5m"          # 链上→本地同步间隔
```

**混合模式（Hybrid）说明**：
- 注册、注销、等级变更等低频高危操作上链存证
- 权限查询、QPS 限流、缓存校验等高频操作走本地
- 后台定时同步链上状态到本地数据库，确保一致性
- 适用于"既要审计追溯，又要高并发性能"的企业场景

---

### 3.2 可配置 Agent 等级体系

v2.0 将固定等级改为**可配置模板**，通过 YAML 文件定义等级规则，系统启动时加载至内存与合约。

#### 3.2.1 默认配置（开箱即用）

```yaml
# config/agent_level.yaml
levels:
  - name: "测试 Agent"
    level: 1
    description: "内部测试与沙箱环境使用"
    permissions:
      apis: ["public.readonly"]
      qps: 2
      daily_quota: 1000
    upgrade_rules:
      auto: false
      requires_approval: true
      max_target_level: 2

  - name: "普通 Agent"
    level: 2
    description: "标准生产环境 Agent"
    permissions:
      apis: ["public.*", "data.read"]
      qps: 10
      daily_quota: 10000
    upgrade_rules:
      auto: false
      requires_approval: true
      max_target_level: 3

# 自定义扩展示例（取消注释即可生效）
#  - name: "高级 Agent"
#    level: 3
#    description: "高权限商业 Agent"
#    permissions:
#      apis: ["public.*", "data.*", "batch.submit"]
#      qps: 50
#      daily_quota: 100000
#    upgrade_rules:
#      auto: false
#      requires_approval: true
#      requires_multi_sig: true
```

#### 3.2.2 位掩码扩展

每个等级除基础模板外，支持位掩码叠加细粒度权限：

```go
const (
    PermDataRead     uint64 = 0x01  // 数据读取
    PermDataWrite    uint64 = 0x02  // 数据写入
    PermBatchSubmit  uint64 = 0x04  // 批量提交
    PermAdminAudit   uint64 = 0x08  // 审计查询
    PermA2AInterop   uint64 = 0x10  // A2A 跨 Agent 互认
)
```

---

### 3.3 准入控制：Agent 专属协议

v2.0 在 MoltCaptcha 基础上增加 **Agent 准入协议（Agent Admission Protocol, AAP）**，从协议层拒绝人类用户与普通爬虫。

#### 3.3.1 三层拦截模型

| 层级 | 拦截对象 | 机制 | 通过率 |
|---|---|---|---|
| L1 网关层 | 人类浏览器、低质量爬虫 | UA 黑名单、TLS 指纹、行为模式 | 拦截 90% 人类/爬虫 |
| L2 协议层 | 无法响应 Agent 协议的实体 | AAP 挑战-响应（Challenge-Response） | 拦截 99% 非 Agent |
| L3 验证层 | 恶意自动化脚本 | MoltCaptcha 反向验证 | 拦截剩余攻击 |

#### 3.3.2 Agent 准入协议（AAP）

AAP 是一种轻量级挑战-响应协议，注册前 Agent 必须完成协议握手：

```
Agent → 网关: 请求注册
网关 → Agent: 返回 Challenge (随机 nonce + 时间戳 + 域名签名)
Agent → 网关: 返回 Response (nonce + Agent 声明 + 能力指纹)
网关 → Agent: 验证通过，进入 MoltCaptcha 环节
```

**Agent 声明格式**：
```json
{
  "agent_version": "agentid-sdk/v2.0.1",
  "capabilities": ["a2a", "mcp", "streaming"],
  "runtime": "python3.11",
  "purpose": "data_pipeline",
  "owner_did": "did:agentid:0x1234..."
}
```

**能力指纹**：基于 Agent 声明的 HMAC-SHA256，防止声明伪造。

---

### 3.4 万亿级 UUID 设计

采用 UUID v7 为主、v4 为备的双模式策略，确保万亿级容量下无冲突、可排序、高性能。

#### 3.4.1 UUID v7 优势

- **时间排序**：前 48 位为 Unix 时间戳，天然支持按时间范围查询
- **分布式安全**：后 74 位随机数，冲突概率低于 10^-18
- **万亿级支撑**：理论空间 2^122 ≈ 5.3 × 10^36，远超万亿需求

#### 3.4.2 生成器实现

```go
package uuid_generator

import (
    "github.com/google/uuid"
    "sync/atomic"
)

var sequence uint32 = 0

// GenerateV7 生成时间排序 UUID，支持高并发
func GenerateV7() string {
    id, err := uuid.NewV7()
    if err != nil {
        // 降级到 v4
        return uuid.New().String()
    }
    return id.String()
}

// GenerateV4 纯随机 UUID，用于特殊场景
func GenerateV4() string {
    return uuid.New().String()
}

// BatchGenerate 批量生成，用于企业批量注册
func BatchGenerate(count int) []string {
    ids := make([]string, count)
    for i := 0; i < count; i++ {
        ids[i] = GenerateV7()
    }
    return ids
}
```

---

## 4. 四种接入范式

v2.0 提供 CLI、MCP、A2A、Prompt 四种接入方式，覆盖从开发者到终端用户的全场景。

### 4.1 CLI 客户端

#### 4.1.1 安装方式

```bash
# 方式一：Go install
go install github.com/agentid-chain/cli/cmd/agentid@latest

# 方式二：Homebrew
brew tap agentid-chain/tap
brew install agentid

# 方式三：Docker
docker pull agentid-chain/cli:latest
alias agentid='docker run --rm -v ~/.agentid:/root/.agentid agentid-chain/cli'
```

#### 4.1.2 配置文件

```yaml
# ~/.agentid/config.yaml
server:
  endpoint: "https://agentid.internal.company.com"
  timeout: 30s

auth:
  mode: "apikey"        # 可选：apikey / did / mcp
  api_key: "${AGENTID_API_KEY}"

backend:
  type: "local"         # 本地模式，零链依赖
```

#### 4.1.3 核心命令

```bash
# 注册新 Agent（自动完成 AAP 协议 + MoltCaptcha）
agentid register   --name "data-pipeline-01"   --level 2   --owner-did "did:agentid:0x1234..."   --output credential.json

# 查询 Agent 信息
agentid info --uuid "018f..."

# 申请升级
agentid upgrade --uuid "018f..." --target-level 3 --reason "Q3 业务扩容"

# 批量注册（企业场景）
agentid batch-register --file agents.csv --output credentials.json

# 导出审计日志
agentid audit --uuid "018f..." --format json --output audit.json

# 本地模式初始化（零链部署）
agentid local init --db-path ./agentid.db
```

#### 4.1.4 批量注册 CSV 格式

```csv
name,owner_did,level,capabilities
data-pipeline-01,did:agentid:0x1234...,2,"data.read,batch.submit"
search-agent-01,did:agentid:0x5678...,2,"public.readonly"
```

---

### 4.2 MCP Server（Model Context Protocol）

#### 4.2.1 设计定位

将 AgentID-Chain 的能力封装为 MCP 标准工具集，供 Claude Desktop、Cursor、Cline 等支持 MCP 的 IDE/Agent 直接调用。

#### 4.2.2 安装配置

```json
// Claude Desktop / Cursor 配置 ~/.cursor/mcp.json
{
  "mcpServers": {
    "agentid-chain": {
      "command": "npx",
      "args": ["-y", "@agentid-chain/mcp-server"],
      "env": {
        "AGENTID_ENDPOINT": "https://agentid.internal.company.com",
        "AGENTID_API_KEY": "sk-...",
        "AGENTID_BACKEND_TYPE": "local"
      }
    }
  }
}
```

#### 4.2.3 暴露的工具集（Tools）

| Tool | 描述 | 参数 |
|---|---|---|
| `agentid_register` | 注册新 Agent | `name`, `level`, `owner_did`, `capabilities` |
| `agentid_get_info` | 查询 Agent 信息 | `uuid` |
| `agentid_upgrade` | 申请权限升级 | `uuid`, `target_level`, `reason` |
| `agentid_check_permission` | 校验权限 | `uuid`, `required_permission` |
| `agentid_audit_logs` | 查询审计日志 | `uuid`, `limit` |
| `agentid_batch_register` | 批量注册 | `agents_json` |
| `agentid_ban` | 封禁 Agent | `uuid`, `reason` |

#### 4.2.4 调用示例（Claude 对话中）

```
User: 帮我注册一个名为 "search-agent" 的 Agent，等级为普通 Agent，具备数据读取能力

Claude: [调用 agentid_register]
结果：
{
  "uuid": "018f3a2b-4c5d-7e8f-9a0b-1c2d3e4f5a6b",
  "level": 2,
  "name": "search-agent",
  "credential": "eyJhbGciOiJIUzI1NiIs...",
  "backend_type": "local"
}
已为您完成注册。UUID 为 018f3a2b-...，凭证已保存至本地。
```

---

### 4.3 A2A 协议（Agent-to-Agent）

#### 4.3.1 设计定位

实现同一 AgentID-Chain 体系内的 Agent 自动互信。当两个 Agent 均注册于同一身份网关时，可通过 A2A 协议完成身份校验与权限协商，无需重复验证。

#### 4.3.2 A2A 身份互认流程

```mermaid
sequenceDiagram
    participant A as Agent A
    participant GW as AgentID Gateway
    participant B as Agent B

    A->>GW: 请求与 Agent B 建立 A2A 连接
    GW->>GW: 校验 A 的 UUID + 凭证
    GW->>GW: 校验 B 的 UUID + 凭证
    GW->>A: 返回 B 的公钥 + 权限摘要
    GW->>B: 返回 A 的公钥 + 权限摘要
    A->>B: 使用对方公钥建立加密通道
    B->>A: 确认通道建立，交换能力声明
    Note over A,B: 后续通信基于互信通道，无需重复校验
```

#### 4.3.3 A2A 端点实现

```go
// a2a/handler.go
package a2a

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

// A2AHandler 处理 Agent 间互认请求
type A2AHandler struct {
    backend core.IdentityBackend
}

// Negotiate 处理 A2A 协商请求
func (h *A2AHandler) Negotiate(c *gin.Context) {
    var req A2ANegotiateRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    // 1. 校验双方身份
    agentA, err := h.backend.GetAgentInfo(c, req.AgentA_UUID)
    if err != nil || agentA.Status != 0 {
        c.JSON(403, gin.H{"error": "agent A invalid"})
        return
    }

    agentB, err := h.backend.GetAgentInfo(c, req.AgentB_UUID)
    if err != nil || agentB.Status != 0 {
        c.JSON(403, gin.H{"error": "agent B invalid"})
        return
    }

    // 2. 校验 A2A 权限位
    if agentA.Permission&core.PermA2AInterop == 0 {
        c.JSON(403, gin.H{"error": "agent A lacks A2A permission"})
        return
    }

    // 3. 生成互信令牌（短期有效）
    token := generateA2AToken(agentA, agentB)

    c.JSON(200, A2ANegotiateResponse{
        Token:     token,
        ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
        AgentA_PermSummary: agentA.PermissionSummary(),
        AgentB_PermSummary: agentB.PermissionSummary(),
    })
}
```

#### 4.3.4 A2A 请求格式

```json
// POST /a2a/negotiate
{
  "agent_a_uuid": "018f3a2b-4c5d-7e8f-9a0b-1c2d3e4f5a6b",
  "agent_a_credential": "eyJhbGciOiJIUzI1NiIs...",
  "agent_b_uuid": "018f3a2b-4c5d-7e8f-9a0b-1c2d3e4f5a6c",
  "agent_b_credential": "eyJhbGciOiJIUzI1NiIs...",
  "purpose": "data_pipeline_collaboration",
  "requested_permissions": ["data.read"],
  "duration": "1h"
}

// Response
{
  "token": "a2a_tok_...",
  "expires_at": 1718000000,
  "trust_level": "full",
  "allowed_permissions": ["data.read"],
  "audit_log_id": "audit_..."
}
```

---

### 4.4 Prompt 接入（自然语言配置）

#### 4.4.1 设计定位

为不具备开发能力的运营人员或终端用户提供自然语言配置入口。通过 Prompt Parser 将自然语言需求转换为结构化配置指令。

#### 4.4.2 Prompt Parser 架构

```
User Prompt → Intent Classifier → Slot Filling → Config Generator → Validation → Execution
```

- **Intent Classifier**：识别用户意图（注册/查询/升级/封禁/配置）
- **Slot Filling**：提取关键参数（名称、等级、权限、原因）
- **Config Generator**：生成对应的 API 调用或配置文件
- **Validation**：校验参数合规性（等级范围、权限冲突等）
- **Execution**：调用后端执行

#### 4.4.3 支持的自然语言模板

| 意图 | 示例 Prompt | 生成的动作 |
|---|---|---|
| 注册 | "帮我注册一个测试 Agent，用于数据抓取" | `agentid_register` (level=1) |
| 升级 | "把 search-agent 升级到普通 Agent，因为 Q3 要用" | `agentid_upgrade` (target=2) |
| 查询 | "查一下 018f... 的状态和权限" | `agentid_get_info` |
| 批量 | "给团队批量注册 5 个普通 Agent，都做数据读取" | `agentid_batch_register` |
| 配置 | "把后端改成本地模式，不用上链" | 修改 `backend.yaml` |
| 审计 | "导出最近 30 天所有 Agent 的操作日志" | `agentid_audit_logs` |

#### 4.4.4 实现示例

```go
// prompt/parser.go
package prompt

import (
    "strings"
    "regexp"
)

type ParsedIntent struct {
    Action     string            `json:"action"`
    Parameters map[string]string `json:"parameters"`
    Confidence float64           `json:"confidence"`
}

// Parse 将自然语言解析为结构化意图
func Parse(input string) (*ParsedIntent, error) {
    input = strings.ToLower(input)

    // 意图匹配规则（可扩展为正则/LLM）
    switch {
    case containsAny(input, []string{"注册", "新建", "创建", "register", "create"}):
        return parseRegisterIntent(input)
    case containsAny(input, []string{"升级", "提升", "upgrade", "promote"}):
        return parseUpgradeIntent(input)
    case containsAny(input, []string{"查询", "查看", "状态", "info", "status"}):
        return parseQueryIntent(input)
    case containsAny(input, []string{"批量", "多个", "batch"}):
        return parseBatchIntent(input)
    case containsAny(input, []string{"本地", "不上链", "local", "offline"}):
        return parseConfigIntent(input)
    default:
        return nil, fmt.Errorf("unable to parse intent")
    }
}

func parseRegisterIntent(input string) (*ParsedIntent, error) {
    intent := &ParsedIntent{Action: "agentid_register", Parameters: map[string]string{}}

    // 提取等级关键词
    if strings.Contains(input, "测试") || strings.Contains(input, "test") {
        intent.Parameters["level"] = "1"
    } else if strings.Contains(input, "普通") || strings.Contains(input, "normal") {
        intent.Parameters["level"] = "2"
    }

    // 提取名称（假设在"注册"和"Agent"之间）
    re := regexp.MustCompile(`(?:注册|新建|创建)\s+(?:一个|个)?\s*(\S+?)\s*(?:Agent|agent|代理)`)
    matches := re.FindStringSubmatch(input)
    if len(matches) > 1 {
        intent.Parameters["name"] = matches[1]
    }

    intent.Confidence = 0.85
    return intent, nil
}
```

#### 4.4.5 使用方式

```bash
# 交互式 Prompt 模式
agentid prompt
> 帮我注册一个名为 data-collector 的普通 Agent
解析意图: register (confidence: 0.92)
参数: name=data-collector, level=2
确认执行? [Y/n]: Y
执行结果: UUID=018f..., 凭证已生成

# 单条命令模式
agentid prompt "把 018f... 升级到高级 Agent，用于批量数据处理"
```

---

## 5. 配置系统

### 5.1 全局配置目录结构

```
config/
├── backend.yaml          # 存储后端配置（链上/链下/混合）
├── agent_level.yaml      # Agent 等级模板配置
├── captcha.yaml          # MoltCaptcha 与 AAP 协议配置
├── a2a.yaml             # A2A 协议参数配置
├── mcp.yaml             # MCP Server 配置
├── cli.yaml             # CLI 客户端默认配置
└── app.yaml             # 服务端口、限流、缓存配置
```

### 5.2 后端配置详解（backend.yaml）

```yaml
backend:
  type: local              # onchain | local | hybrid

  onchain:
    driver: fisco-bcos
    rpc_url: "http://127.0.0.1:8545"
    contract_address: "0x..."
    operator_private_key: "${PRIVATE_KEY}"
    gas_limit: 300000
    confirmation_blocks: 1  # 确认块数

  local:
    db_driver: sqlite
    dsn: "file:agentid.db?cache=shared&_fk=1"
    encrypt_at_rest: true
    encryption_key: "${ENCRYPTION_KEY}"
    redis_addr: "localhost:6379"
    redis_db: 0

  hybrid:
    identity_onchain: true
    permission_local: true
    sync_interval: "5m"
    sync_batch_size: 100
```

### 5.3 A2A 配置详解（a2a.yaml）

```yaml
a2a:
  enabled: true
  token_ttl: "1h"           # 互信令牌有效期
  max_negotiations_per_hour: 100  # 单 Agent 每小时最大协商次数
  allowed_permission_override: false  # 是否允许协商时临时提升权限
  audit_all_negotiations: true      # 所有协商记录审计
```

### 5.4 MCP 配置详解（mcp.yaml）

```yaml
mcp:
  enabled: true
  server_name: "agentid-chain"
  version: "2.0.0"
  tools:
    - agentid_register
    - agentid_get_info
    - agentid_upgrade
    - agentid_check_permission
    - agentid_audit_logs
    - agentid_batch_register
    - agentid_ban
  auth:
    required: true
    api_key_header: "X-API-Key"
```

---

## 6. API 参考

### 6.1 公共 API（所有接入方式共用）

#### 注册 Agent
```http
POST /api/v2/agents/register
Content-Type: application/json
X-API-Key: sk-...

{
  "name": "data-pipeline-01",
  "owner_did": "did:agentid:0x1234...",
  "init_level": 2,
  "capabilities": ["data.read", "batch.submit"],
  "admission_proof": "aap_proof_..."  // AAP 协议完成证明
}

Response:
{
  "uuid": "018f3a2b-4c5d-7e8f-9a0b-1c2d3e4f5a6b",
  "level": 2,
  "status": 0,
  "credential": "eyJhbGciOiJIUzI1NiIs...",
  "backend_type": "local",
  "expires_at": null
}
```

#### 查询 Agent
```http
GET /api/v2/agents/{uuid}
X-API-Key: sk-...

Response:
{
  "uuid": "018f3a2b-...",
  "name": "data-pipeline-01",
  "level": 2,
  "level_name": "普通 Agent",
  "permissions": {
    "apis": ["public.*", "data.read"],
    "qps": 10,
    "daily_quota": 10000,
    "mask": "0x05"
  },
  "status": 0,
  "owner_did": "did:agentid:0x1234...",
  "created_at": "2026-06-08T15:30:00Z",
  "backend_type": "local"
}
```

#### 升级 Agent
```http
POST /api/v2/agents/{uuid}/upgrade
X-API-Key: sk-...

{
  "target_level": 3,
  "reason": "Q3 业务扩容需求",
  "requested_permissions": ["data.write"]
}

Response:
{
  "uuid": "018f3a2b-...",
  "old_level": 2,
  "new_level": 3,
  "status": "pending_approval",  // 或 "approved" / "rejected"
  "approval_id": "app_...",
  "tx_hash": null  // 链上模式时返回交易哈希
}
```

#### 校验权限
```http
POST /api/v2/agents/{uuid}/check
X-API-Key: sk-...

{
  "required_permission": "data.read",
  "requested_qps": 5
}

Response:
{
  "allowed": true,
  "current_qps": 3,
  "remaining_quota": 9997,
  "cache_hit": true
}
```

### 6.2 MCP 工具接口

MCP Server 将上述 HTTP API 封装为标准 MCP Tools，参数与 HTTP API 保持一致，通过 JSON-RPC 2.0 协议通信。

### 6.3 A2A 协议接口

| 端点 | 方法 | 描述 |
|---|---|---|
| `/a2a/negotiate` | POST | 发起 Agent 间互信协商 |
| `/a2a/verify` | POST | 验证 A2A 令牌有效性 |
| `/a2a/revoke` | POST | 撤销已建立的互信关系 |
| `/a2a/list` | GET | 查询 Agent 的所有互信关系 |

---

## 7. 快速开始

### 7.1 场景一：本地零链部署（CLI 模式）

适用于企业内部 Agent 管理，零区块链依赖，5 分钟启动。

```bash
# 1. 安装 CLI
go install github.com/agentid-chain/cli/cmd/agentid@latest

# 2. 初始化本地后端
agentid local init --db-path ./agentid.db

# 3. 编辑配置
mkdir -p ~/.agentid
cat > ~/.agentid/config.yaml <<EOF
backend:
  type: local
  local:
    db_driver: sqlite
    dsn: "file:./agentid.db?cache=shared"
    encrypt_at_rest: false
EOF

# 4. 注册测试 Agent
agentid register --name "test-agent-01" --level 1

# 5. 查询
agentid info --uuid "<返回的UUID>"
```

### 7.2 场景二：MCP 接入 Cursor

适用于开发者在 IDE 中直接管理 Agent 身份。

```bash
# 1. 安装 MCP Server
npm install -g @agentid-chain/mcp-server

# 2. 配置 Cursor
# 编辑 ~/.cursor/mcp.json，填入第 4.2 节配置

# 3. 重启 Cursor，在 Agent 对话中直接调用
# "帮我注册一个普通 Agent"
```

### 7.3 场景三：A2A 互认部署

适用于多 Agent 协作场景，同一体系内自动互信。

```bash
# 1. 部署网关（本地或云端）
docker run -d   -p 8080:8080   -v ./config:/app/config   agentid-chain/gateway:latest

# 2. 配置 A2A 协议
# 编辑 config/a2a.yaml，启用 enabled: true

# 3. Agent A 与 Agent B 协商互信
# 使用 SDK 或 CLI 调用 /a2a/negotiate
```

### 7.4 场景四：混合模式（企业级）

适用于"既要审计追溯，又要高并发"的场景。

```bash
# 1. 部署 FISCO BCOS 节点（或对接现有联盟链）
# 2. 部署智能合约，获取合约地址
# 3. 配置混合模式

cat > config/backend.yaml <<EOF
backend:
  type: hybrid
  onchain:
    driver: fisco-bcos
    rpc_url: "http://fisco-node:8545"
    contract_address: "0x..."
    operator_private_key: "${PRIVATE_KEY}"
  local:
    db_driver: postgres
    dsn: "postgres://agentid:pass@localhost/agentid?sslmode=disable"
    redis_addr: "localhost:6379"
  hybrid:
    identity_onchain: true
    permission_local: true
    sync_interval: "5m"
EOF

# 4. 启动网关
agentid server --config ./config

# 5. 注册 Agent（自动上链存证）
agentid register --name "prod-agent-01" --level 2
```

---

## 8. 安全与风控

### 8.1 准入层安全

- **UA 黑名单**：拦截常见浏览器 UA、爬虫框架标识（Scrapy、curl、wget 等）
- **TLS 指纹校验**：拒绝非标准 TLS 指纹（人类浏览器指纹库 vs Agent SDK 指纹）
- **AAP 协议强制**：无 AAP 完成证明的注册请求直接拒绝
- **MoltCaptcha 兜底**：通过 AAP 后仍需完成反向验证，防止 AAP 协议被逆向

### 8.2 权限安全

- **绝对禁止自升级**：Agent 无法自行修改自身等级或权限
- **多签保护**：高危操作（顶级权限、批量封禁）需 2~3 管理员多签
- **临时权限自动回收**：临时提权到期后合约自动回退，无需人工介入
- **权限缓存失效**：任何等级变更立即触发 Redis 缓存清空，确保实时生效

### 8.3 数据安全

- **本地模式加密**：`encrypt_at_rest: true` 时，SQLite 数据库使用 AES-256-GCM 加密
- **链上数据不可篡改**：所有变更日志永久存证，满足审计要求
- **凭证安全**：凭证使用 HMAC-SHA256（本地）或 ECDSA（链上）签名，防止伪造
- **环境变量隔离**：所有密钥通过 `${ENV_VAR}` 注入，禁止硬编码

### 8.4 审计与风控

- **全量操作日志**：注册/升级/封禁/注销 全量记录，支持按 UUID/时间/操作人检索
- **异常行为检测**：QPS 突增、权限频繁申请、异地登录等触发风控告警
- **批量操作限制**：单管理员每小时批量操作上限，防止误操作或内部作恶

---

## 9. 部署模式总结

| 部署模式 | 存储后端 | 适用场景 | 区块链依赖 | 接入方式 |
|---|---|---|---|---|
| **本地测试** | LocalBackend (SQLite) | 开发调试、单机构测试 | 无 | CLI / MCP / Prompt |
| **企业内网** | LocalBackend (PostgreSQL) | 企业内部 Agent 集群 | 无 | CLI / MCP / A2A / Prompt |
| **混合模式** | Hybrid (链上+本地) | 既要审计又要性能 | 联盟链/公链 | 全方式 |
| **开放生态** | OnchainBackend | 跨平台 Agent 互认 | 公链 (Polygon/BSC) | A2A / MCP |
| **联盟生态** | OnchainBackend | 多机构互信 | 联盟链 (FISCO/Hyperledger) | A2A / CLI |

---

## 10. 贡献指南

### 10.1 新增链适配器

1. 实现 `chain_adapter.BaseChainAdapter` 接口
2. 在 `config/chain.yaml` 中添加驱动配置
3. 提供单元测试与集成测试
4. 提交 PR 至 `adapters/` 目录

### 10.2 新增接入方式

1. 实现对应协议的服务端（如新增 gRPC 接入）
2. 在 `core/backend/` 中复用 IdentityBackend 接口，无需改动存储逻辑
3. 在 `config/` 中添加对应配置文件
4. 更新本文档第 4 节与第 6 节

### 10.3 新增 Agent 等级

1. 编辑 `config/agent_level.yaml` 添加新等级定义
2. 重启服务自动加载，无需重新部署合约（本地模式）或重新部署合约（链上模式需管理员操作）

---

## 11. 附录

### 11.1 环境变量清单

| 变量名 | 说明 | 必填 |
|---|---|---|
| `AGENTID_PRIVATE_KEY` | 链上操作私钥 | 链上/混合模式 |
| `AGENTID_ENCRYPTION_KEY` | 本地数据库加密密钥 | 本地加密模式 |
| `AGENTID_API_KEY` | API 调用密钥 | 是 |
| `AGENTID_REDIS_ADDR` | Redis 地址 | 是 |
| `AGENTID_DB_DSN` | 数据库连接串 | 本地/混合模式 |

### 11.2 错误码表

| 错误码 | 说明 | 处理建议 |
|---|---|---|
| `4001` | AAP 协议校验失败 | 检查 Agent SDK 版本与声明格式 |
| `4002` | MoltCaptcha 验证失败 | 重新完成反向验证 |
| `4003` | 人类/爬虫特征拦截 | 确保使用 Agent SDK 发起请求 |
| `4031` | 权限不足 | 申请升级或检查位掩码权限 |
| `4032` | Agent 已封禁 | 联系管理员申诉 |
| `4091` | UUID 已存在 | 使用新 UUID 或查询已有身份 |
| `5001` | 链上交易失败 | 检查 Gas、Nonce、合约状态 |
| `5002` | 本地数据库错误 | 检查磁盘空间与数据库连接 |

### 11.3 版本兼容性

| 版本 | 协议兼容 | 存储兼容 | 升级路径 |
|---|---|---|---|
| v1.0 → v2.0 | 不兼容（新增 AAP 协议） | 兼容（数据可迁移） | 导出 v1.0 数据 → v2.0 批量导入 |
| v2.0.x → v2.0.y | 兼容 | 兼容 | 直接替换二进制 |

---

> **文档版本**：v2.0  
> **最后更新**：2026-06-08  
> **维护者**：AgentID-Chain 开源社区  
> **许可证**：Apache 2.0
