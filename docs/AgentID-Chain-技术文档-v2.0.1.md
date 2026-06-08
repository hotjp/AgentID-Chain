# AgentID-Chain 技术文档 v2.0.1

> **文档定位**：面向开发者的全量技术方案，支持"给其他 Agent 直接开发"的认知对齐水平。
> **核心变更**：v2.0.1 在 v2.0（混合存储 + 四种接入范式）基础上做以下微调（**不破坏既有结构**）：
> 1. 数据库统一为 **PostgreSQL**（删除 SQLite 选项）
> 2. 模块职责表新增「**架构分层**」列，明确每个模块在 5 层架构中的位置
> 3. 新增 §3.5「**5 层架构在 AgentID-Chain 的落地**」，与 `architecture.md` 对齐
> 4. AAP 协议从概要升级为**完整协议规范**（Challenge/Response 格式、签名、防重放、TTL）
> 5. A2A 协议补全 **Token 格式 / 撤销机制 / TrustLevel 计算规则**
> 6. backend.yaml local 段强制 `db_driver: postgres`
> 7. 环境变量收敛（去除 SQLite 加密相关变量）
> 8. **§7.5 Docker 部署补全**：全栈多场景 compose（dev / local-only / hybrid）+ Docker Hub 镜像约定
> 9. **§3.3.3 MoltCaptcha 引擎**：Go 重写，保留与上游协议格式兼容（重写 vs 集成 vs 借鉴 — 见 §3.3.3.1）
>
> **冻结状态**：v2.0.1 作为当前需求基线，写入后只接受 bug 修复，不再做结构性调整。

---

## 1. 项目概述

### 1.1 设计目标

AgentID-Chain 是专为 AI Agent 生态设计的分布式身份与权限网关。v2.0.1 在保持 v2.0「混合存储 + 多协议接入」核心能力的基础上，对数据库选型、架构映射、协议细节做收敛与补全，使项目可直接进入工程实施阶段。

### 1.2 核心能力矩阵

| 能力维度 | v1.0 状态 | v2.0 状态 | v2.0.1 调整 |
|---|---|---|---|
| 身份容量 | 百亿级 UUID | **万亿级 UUID**，架构层面无冲突 | 不变 |
| 存储后端 | 仅区块链 | **链上/链下可配置**，企业内可零链部署 | 链下后端**统一为 PostgreSQL**（ent + pgx/v5），删除 SQLite 选项 |
| Agent 体系 | 固定 4 级（含爬虫） | **可配置模板**，默认测试/普通两级 | 不变 |
| 准入控制 | 反向验证 | **Agent 准入协议 + MoltCaptcha** | AAP 协议升级为完整规范（见 §3.3） |
| 接入方式 | HTTP API | **CLI / MCP / A2A / Prompt** 四种范式 | 不变 |
| 互认协议 | 无 | **A2A 协议原生支持** | A2A Token/撤销/TrustLevel 补全（见 §4.3） |
| 架构规范 | 隐式 | 隐式 | **对齐 architecture.md 5 层架构**，每个模块标注 L1-L5（见 §2.2 / §3.5） |

### 1.3 适用场景

- **开放生态**：对接 Polygon/BSC 等公链，支撑跨平台 Agent 身份互认
- **企业内网**：**PostgreSQL** 部署，零区块链依赖，内部 Agent 集群管理
- **混合模式**：核心身份链上存证，高频权限本地缓存，兼顾安全与性能
- **Agent 工具链**：通过 MCP 协议嵌入 Cursor/Cline/Claude Desktop 等工具
- **自动化脚本**：通过 CLI 完成批量注册、权限调整、审计导出
- **Prompt 工程**：通过自然语言配置完成身份初始化与权限申请

---

## 2. 整体架构

### 2.1 逻辑架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    接入层（Access Layer / L5）                │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │  CLI    │  │  MCP    │  │  A2A    │  │  Prompt Parser  │ │
│  │ Client  │  │ Server  │  │ Agent   │  │  (NL→Config)    │ │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────────┬────────┘ │
│       └─────────────┴─────────────┴────────────────┘          │
│                         │                                    │
│              ┌──────────▼──────────┐                       │
│              │  网关 & 路由分流(L5)  │                       │
│              │  UA拦截/限流/AAP 协议  │                       │
│              └──────────┬──────────┘                       │
└───────────────────────────┼─────────────────────────────────┘
                          │
┌───────────────────────────▼─────────────────────────────────┐
│                 核心服务层（Core / L3-L4）                    │
│  ┌──────────────────────────┐  ┌────────────────────────┐  │
│  │  L3 Authz：准入+权限决策   │  │ L4 Service：业务编排   │  │
│  │  · AAP Challenge 校验     │  │ · Register/Upgrade/... │  │
│  │  · MoltCaptcha 验证        │  │ · 事务边界/工作流       │  │
│  │  · A2A Token 校验          │  │ · 插件接口定义          │  │
│  │  · Rate Limiter            │  │                       │  │
│  └────────────┬─────────────┘  └──────────┬─────────────┘  │
│               │                            │                │
│  ┌────────────▼────────────────────────────▼────────────┐  │
│  │  L2 Domain：UUID 生成 / Agent 实体 / 状态机 / 事件     │  │
│  │  IdentityBackend 接口（链上/链下统一抽象）             │  │
│  └────────────┬──────────────────────────────────────────┘  │
└───────────────┼─────────────────────────────────────────────┘
                │
┌───────────────▼─────────────────────────────────────────────┐
│                  数据层（Data / L1）                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  链上存证   │  │  链下后端   │  │   审计日志库         │  │
│  │  FISCO/Poly │  │ PostgreSQL │  │   PostgreSQL        │  │
│  │  (可选)     │  │  + ent     │  │  (同库)             │  │
│  └─────────────┘  └──────┬──────┘  └─────────────────────┘  │
│                          │                                  │
│                  ┌───────▼───────┐                          │
│                  │  Redis 缓存   │                          │
│                  │  + Stream     │                          │
│                  └───────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 模块职责

| 模块 | 职责 | 关键接口/文件 | **架构分层** |
|---|---|---|---|
| **CLI Client** | 命令行工具，支持注册、查询、升级、导出 | `cmd/agentid/main.go` | L5 接入 |
| **MCP Server** | 暴露 MCP 标准工具集，供 IDE/Agent 调用 | `mcp/server.go` | L5 接入 |
| **A2A Agent** | A2A 协议端点，处理 Agent 间身份互认请求 | `a2a/handler.go` | L5 接入 + L3 校验 |
| **Prompt Parser** | 自然语言解析器，将需求转为配置指令 | `prompt/parser.go` | L5 接入 |
| **网关层** | TLS 终止、流量拆分、UA 拦截、AAP 协议入口 | `gateway/` | L5 |
| **Authz 决策** | AAP / MoltCaptcha / A2A Token / Rate Limit 决策 | `internal/authz/` | L3 |
| **Service 编排** | Register/Upgrade/Batch/Revoke 业务流程 | `internal/service/` | L4 |
| **Agent 实体** | 领域模型、状态机（注册→正常→封禁→注销）、版本号 | `internal/domain/agent.go` | L2 |
| **UUID 生成器** | 分布式 v4/v7 生成，全局唯一性保障 | `internal/uuid_generator/` | L2 |
| **Identity Backend** | 存储后端抽象，链上/链下统一接口 | `core/backend/interface.go` | L2 接口 / L1 实现 |
| **链适配器** | 多链插拔实现（FISCO/Polygon/BSC/mock） | `core/chain_adapter/` | L1 插件 |
| **本地后端** | 基于 **PostgreSQL + ent** 的纯本地身份管理 | `core/backend/local.go` | L1 |
| **链上后端** | 通过 ChainAdapter 调用智能合约 | `core/backend/onchain.go` | L1 插件 |
| **Outbox 转发** | 领域事件 → Redis Stream | `internal/storage/outbox/` | L1 |
| **MoltCaptcha 引擎** | SMHL 挑战生成与验证（详见 §3.3） | `internal/captcha/moltcaptcha/` | L3 |
| **AAP 协议** | Challenge-Response 握手（详见 §3.3） | `internal/captcha/aap/` | L3 |
| **RBAC 引擎** | 位掩码权限 + 等级模板校验 | `internal/authz/rbac/` | L3 |
| **审计日志** | 全量操作记录，UUID/时间/操作人可检索 | `internal/storage/audit/` | L1 |
| **缓存层** | Redis 权限缓存、Nonce 防重放、A2A Token 撤销列表 | `internal/cache/` | L1 支撑 |

**分层铁律**（与 `architecture.md` 一致）：
```
L5-Gateway → L3-Authz → L4-Service → L2-Domain → L1-Storage
```
- 核心层定义接口，插件层实现，依赖倒置
- L2 禁止 import 任何第三方包（除标准库）
- L4 通过 interface 注入插件，禁止直接 import 插件包

---

## 3. 核心设计详解

### 3.1 混合存储架构：链上/链下可配置

v2.0.1 引入 `IdentityBackend` 统一抽象接口，所有身份操作均通过该接口下发，业务层完全无感知存储后端类型。

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

- **实现文件**：`core/backend/onchain.go`（插件）
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

#### 3.1.3 链下后端（LocalBackend）— **v2.0.1 调整：统一 PostgreSQL**

- **实现文件**：`core/backend/local.go`
- **职责**：基于 **PostgreSQL + ent ORM** 的纯本地身份管理，零区块链依赖
- **适用场景**：企业内部 Agent 集群、开发测试、隐私合规要求高的场景
- **存储栈**：
  - 主存储：**PostgreSQL 14+**（使用 ent ORM，底层 `pgx/v5`）
  - 缓存：Redis（权限位、Nonce、A2A Token 黑名单）
  - 审计：同库 `audit_logs` 表（外键关联 agents）
- **特性**：
  - UUIDv7 主键，默认 B-tree 索引即可按时间范围扫描
  - 权限变更通过本地 RBAC 引擎执行
  - 审计日志写入 `audit_logs` 表，支持按 UUID/时间/操作人检索
  - 与链上后端保持**接口级兼容**，未来可无损迁移上链
  - **v2.0.1 变更**：删除 SQLite 路径，DSN 统一为 PostgreSQL

```go
type LocalBackend struct {
    db    *ent.Client        // ent 客户端（底层 pgx/v5）
    cache *cache.RedisCache
    rbac  *authz.RBACEngine
}

func (lb *LocalBackend) RegisterAgent(ctx context.Context, req *RegisterRequest) (*AgentCredential, error) {
    // 1. 生成 UUIDv7
    uuid := uuid_generator.GenerateV7()

    // 2. ent 事务写入身份记录（txn 包含 audit_log 同步写入）
    cred, err := lb.db.InTransaction(ctx, func(tx *ent.Tx) (any, error) {
        agent, err := tx.Agent.Create().
            SetUUID(uuid).
            SetOwner(req.Owner).
            SetLevel(req.InitLevel).
            SetPermission(req.Permission).
            SetStatus(0). // 0=正常
            Save(ctx)
        if err != nil {
            return nil, fmt.Errorf("local register failed: %w", err)
        }

        // 同步写入审计
        _, err = tx.AuditLog.Create().
            SetAgentID(agent.ID).
            SetAction("register").
            SetReason(req.Reason).
            SetOperator(req.OperatorDID).
            Save(ctx)
        if err != nil {
            return nil, fmt.Errorf("audit log insert failed: %w", err)
        }

        return &AgentCredential{
            UUID:      uuid,
            Signature: lb.signLocalCredential(uuid),
        }, nil
    })
    if err != nil {
        return nil, err
    }

    // 3. 本地 RBAC 初始化权限
    lb.rbac.Grant(uuid, req.InitLevel, req.Permission)

    // 4. 写入缓存
    lb.cache.SetAgentInfo(ctx, uuid, req.InitLevel, 15*time.Minute)

    return cred.(*AgentCredential), nil
}
```

#### 3.1.4 配置化切换（v2.0.1 调整：db_driver 强制 postgres）

```yaml
# config/backend.yaml
backend:
  type: local           # 可选：onchain | local | hybrid

  # 当 type=onchain 或 hybrid 时生效
  onchain:
    driver: fisco-bcos  # 可选：fisco-bcos / fabric / polygon / bsc / mock
    rpc_url: "http://127.0.0.1:8545"
    contract_address: "0x..."
    operator_private_key: "${AGENTID_PRIVATE_KEY}"  # v2.0.1：统一变量名
    gas_limit: 300000
    confirmation_blocks: 1

  # 当 type=local 或 hybrid 时生效
  # v2.0.1：删除 db_driver 字段，强制使用 postgres
  local:
    dsn: "${AGENTID_DB_DSN}"   # 形如 postgres://agentid:pass@localhost:5432/agentid?sslmode=disable
    max_open: 25
    max_idle: 10
    max_lifetime: "5m"
    redis_addr: "localhost:6379"
    redis_db: 0

  # 混合模式：核心身份上链，高频权限本地缓存
  hybrid:
    identity_onchain: true      # 注册/注销/等级变更上链
    permission_local: true       # 权限查询/限流走本地
    sync_interval: "5m"          # 链上→本地同步间隔
    sync_batch_size: 100
```

**混合模式（Hybrid）说明**：
- 注册、注销、等级变更等低频高危操作上链存证
- 权限查询、QPS 限流、缓存校验等高频操作走本地
- 后台定时同步链上状态到本地数据库，确保一致性
- 适用于"既要审计追溯，又要高并发性能"的企业场景

**PostgreSQL Schema 概览**（ent 生成）：
```go
// ent/schema/agent.go
type Agent struct {
    ent.Schema
}

func (Agent) Fields() []ent.Field {
    return []ent.Field{
        field.String("uuid").Unique().Immutable(),  // UUIDv7
        field.String("owner_did"),
        field.Uint8("level"),
        field.Uint64("permission"),  // 位掩码
        field.Uint8("status"),        // 0=正常 1=封禁 2=注销
        field.Time("created_at").Default(time.Now).Immutable(),
        field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
    }
}

func (Agent) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("owner", User.Type).Ref("agents").Unique(),
        edge.To("audit_logs", AuditLog.Type),
    }
}
```

---

### 3.2 可配置 Agent 等级体系

v2.0.1 沿用 v2.0 的可配置模板方案。等级规则通过 YAML 加载到内存（链下后端）或部署到合约（链上后端），无新增/破坏性变更。

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

### 3.3 准入控制：Agent 专属协议（v2.0.1 补全 AAP + MoltCaptcha）

v2.0.1 在 v2.0 三层拦截模型基础上，对 AAP 协议与 MoltCaptcha 的协议细节做完整定义。

#### 3.3.1 三层拦截模型

| 层级 | 拦截对象 | 机制 | 通过率 | 实现位置 |
|---|---|---|---|---|
| L1 网关层 | 人类浏览器、低质量爬虫 | UA 黑名单、TLS 指纹、行为模式 | 拦截 90% 人类/爬虫 | `gateway/middleware/ua_block.go` |
| L2 协议层 | 无法响应 AAP 协议的实体 | AAP Challenge-Response（详见 §3.3.2） | 拦截 99% 非 Agent | `internal/captcha/aap/` |
| L3 验证层 | 恶意自动化脚本 | MoltCaptcha 反向验证（详见 §3.3.3） | 拦截剩余攻击 | `internal/captcha/moltcaptcha/` |

#### 3.3.2 Agent 准入协议（AAP）— **v2.0.1 完整规范**

AAP 是 L2 层的轻量级 Challenge-Response 协议。**任何注册/敏感接口调用**都必须在 AAP 完成之后。

##### 3.3.2.1 协议流程

```
Agent                AgentID 网关                  Redis
 │                      │                            │
 │ ① POST /aap/challenge │                            │
 │ (X-Agent-Version, ...)│                            │
 │─────────────────────>│                            │
 │                      │ 生成 nonce + ts + sig       │
 │                      │ 写入 nonce:{v} (TTL 30s)   │
 │                      │─────────────────────────>│
 │<─────────────────────│                            │
 │  Challenge (JSON)    │                            │
 │                      │                            │
 │ ② POST /aap/verify  │                            │
 │ {challenge, response} │                            │
 │   response = HMAC-SHA256(                            │
 │     shared_secret,                                │
 │     nonce|ts|agent_decl_fp                        │
 │   )                                              │
 │─────────────────────>│                            │
 │                      │ 校验 ts 新鲜度 (30s 窗口)   │
 │                      │ GETEXISTS nonce:{v}        │
 │                      │─────────────────────────>│
 │                      │ 校验签名                    │
 │                      │ 删除 nonce（防重放）         │
 │                      │─────────────────────────>│
 │                      │ 颁发 AAP Proof (JWT, 5min) │
 │<─────────────────────│                            │
 │ AAP Proof (JWT)      │                            │
```

##### 3.3.2.2 Challenge 格式

```json
// POST /aap/challenge  Response 200
{
  "challenge_id": "chg_01HXY...",
  "nonce": "3f9c1a8b7e2d...",      // 128-bit 随机
  "issued_at": 1718000000,           // Unix 秒
  "expires_at": 1718000030,          // 30s 窗口
  "domain_sig": "ed25519:abc...xyz", // 网关域签名（防跨域重放）
  "agent_decl_required": [
    "agent_version",
    "capabilities",
    "runtime",
    "purpose",
    "owner_did"
  ]
}
```

**字段语义**：
| 字段 | 必填 | 长度/格式 | 说明 |
|---|---|---|---|
| `challenge_id` | 是 | ULID | 全局唯一挑战 ID，用于审计 |
| `nonce` | 是 | 32 hex chars (128 bit) | 高熵随机，绑定单次会话 |
| `issued_at` / `expires_at` | 是 | Unix 秒 | 时间窗，**默认 30s**（`config.aap.challenge_ttl`） |
| `domain_sig` | 是 | Ed25519 hex | 网关对 `{challenge_id, nonce, expires_at}` 的签名，Agent 必须验证 |
| `agent_decl_required` | 是 | string[] | 要求 Agent 声明的字段清单 |

**服务端存储**：
- Redis Key：`aap:nonce:{nonce}`，Value：`{challenge_id, agent_ip}`，TTL=30s
- 用途：响应时一次性消费（`DEL` 后置位），防重放

##### 3.3.2.3 Agent 声明（Request Body）

```json
// POST /aap/challenge  Request Body（可选，用于能力指纹绑定）
{
  "agent_version": "agentid-sdk/v2.0.1",
  "capabilities": ["a2a", "mcp", "streaming"],
  "runtime": "python3.11",
  "purpose": "data_pipeline",
  "owner_did": "did:agentid:0x1234..."
}
```

服务端对 `agent_decl` 计算 **能力指纹**：
```go
agent_decl_fp := sha256(canonical_json(agent_decl))
// 存入 Redis 与 challenge_id 关联
```

##### 3.3.2.4 Response 格式

```json
// POST /aap/verify  Request Body
{
  "challenge_id": "chg_01HXY...",
  "nonce": "3f9c1a8b7e2d...",
  "agent_decl": { ... },                    // 与 challenge 阶段一致
  "response_sig": "ed25519:abc...xyz"        // Agent 用其身份私钥签名
}

// 签名原文（Agent 侧组装）：
canonical := "{nonce}|{issued_at}|{sha256(canonical_json(agent_decl))}"
response_sig := Ed25519.sign(agent_private_key, canonical)
```

##### 3.3.2.5 服务端校验流程

1. **时间窗**：`now ∈ [issued_at, expires_at]`，否则 `4001 AAP_TIMEOUT`
2. **Nonce 一次性**：`DEL aap:nonce:{nonce}`，若返回值=0 说明已被消费，`4001 AAP_REPLAY`
3. **域签名验证**：用网关公钥验证 `domain_sig`，失败 `4001 AAP_BAD_DOMAIN`
4. **声明指纹比对**：服务端重算 `sha256(canonical_json(agent_decl))`，与 Redis 中存储比对
5. **响应验签**：用 Agent 声明的 `owner_did` 派生公钥，验证 `response_sig`

##### 3.3.2.6 AAP Proof 颁发

通过后服务端颁发 **AAP Proof**（短期 JWT）：
```json
// POST /aap/verify  Response 200
{
  "aap_proof": "eyJhbGciOiJFZERTQSIs...",   // EdDSA JWT
  "expires_at": 1718000300,                  // issued_at + 5min
  "claims": {
    "sub": "did:agentid:0x1234...",
    "aud": "agentid-gateway",
    "iss": "agentid-aap",
    "exp": 1718000300,
    "challenge_id": "chg_01HXY...",
    "capabilities": ["a2a", "mcp", "streaming"]
  }
}
```

**AAP Proof 用法**：所有后续注册/敏感接口必须在请求头携带 `X-AAP-Proof: <jwt>`。网关验签 JWT 有效且未过期后放行。

##### 3.3.2.7 配置项

```yaml
# config/captcha.yaml（v2.0.1 收敛）
captcha:
  aap:
    enabled: true
    challenge_ttl: "30s"
    proof_ttl: "5m"
    domain_private_key: "${AGENTID_DOMAIN_PRIVATE_KEY}"  # Ed25519，签 domain_sig
    domain_public_key_jwk: "<jwk>"                       # 公开发布供 Agent 验证
    require_https: true        # 生产强制 HTTPS
    rate_limit_per_ip: "10/min"
```

#### 3.3.3 MoltCaptcha 引擎 — **v2.0.1 设计**

> **设计目标**：在 AAP 通过后，对请求方做"语义+数学混合锁"反向验证，确保调用者是真实 LLM 而非脚本/爬虫。

##### 3.3.3.1 设计原则

- **实现语言**：Go（`internal/captcha/moltcaptcha/`），不依赖外部 Python 进程
- **协议兼容**：与上游 `docs/参考/MoltCaptcha` 协议格式保持一致（约束类型、难度、Topic 集合）
- **可插拔**：未来如需切换实现，仅需替换 `MoltCaptchaEngine` 接口

##### 3.3.3.2 挑战结构

```go
// internal/captcha/moltcaptcha/types.go
package moltcaptcha

type Difficulty string
const (
    Easy    Difficulty = "easy"     // ASCII sum only
    Medium  Difficulty = "medium"   // + word count
    Hard    Difficulty = "hard"     // + char position
    Extreme Difficulty = "extreme"  // + total char count
)

type Challenge struct {
    ID            string     `json:"id"`             // ULID
    Difficulty    Difficulty `json:"difficulty"`
    Topic         string     `json:"topic"`          // 预定义 topic 池
    Format        string     `json:"format"`         // haiku|quatrain|free_verse|micro_story
    AsciiTarget   int        `json:"ascii_target"`   // 首字母 ASCII 之和
    WordCount     int        `json:"word_count,omitempty"`
    CharPosition  *CharPosRule `json:"char_position,omitempty"`
    TotalChars    int        `json:"total_chars,omitempty"`
    TimeLimit     int        `json:"time_limit"`     // 秒
    IssuedAt      int64      `json:"issued_at"`
    ExpiresAt     int64      `json:"expires_at"`
    Nonce         string     `json:"nonce"`          // 32 hex
}
```

##### 3.3.3.3 难度与时间窗

| 难度 | 约束 | 时间窗 | 通过率预期 |
|---|---|---|---|
| Easy | ASCII sum only | 30s | 99%+ LLM |
| Medium | ASCII sum + word count | 20s | 99%+ LLM |
| Hard | + char position | 15s | 95%+ LLM |
| Extreme | + total chars | 10s | 90%+ LLM |

##### 3.3.3.4 Topic 池

预定义（与上游兼容）：
```
verification, authenticity, digital trust, cryptography, identity,
algorithms, neural networks, computation, binary, protocols, encryption,
tokens, agents, automation, circuits, logic gates, recursion, entropy,
hashing, signatures
```

##### 3.3.3.5 生成算法（Go）

```go
// internal/captcha/moltcaptcha/generator.go
func GenerateChallenge(difficulty Difficulty) (*Challenge, error) {
    topic := pickRandom(TopicPool)
    format := pickRandom(Formats)
    nLines := lineCountOf(format)
    
    // ASCII target：根据 nLines 在合理区间
    var asciiMin, asciiMax int
    if nLines == 3 {
        asciiMin, asciiMax = 280, 320
    } else {
        asciiMin, asciiMax = 380, 420
    }
    asciiTarget := asciiMin + rand.Intn(asciiMax-asciiMin)
    
    // 可解性预检：确保 asciiTarget 在常用字母组合下可达
    if !isAchievable(asciiTarget, nLines) {
        return GenerateChallenge(difficulty)  // 重抽
    }
    
    ch := &Challenge{
        ID:          ulid.Make().String(),
        Difficulty:  difficulty,
        Topic:       topic,
        Format:      format,
        AsciiTarget: asciiTarget,
        TimeLimit:   timeLimitOf(difficulty),
        IssuedAt:    time.Now().Unix(),
        Nonce:       randomHex(32),
    }
    ch.ExpiresAt = ch.IssuedAt + int64(ch.TimeLimit)
    
    if difficulty >= Medium {
        ch.WordCount = 6 + rand.Intn(15)  // 6~20
    }
    if difficulty >= Hard {
        // 随机指定一行第 N 个字符
        ch.CharPosition = &CharPosRule{
            Line:     1 + rand.Intn(nLines),
            Position: 1 + rand.Intn(20),
            Char:     randomLowerLetter(),
        }
    }
    if difficulty == Extreme {
        ch.TotalChars = 50 + rand.Intn(150)
    }
    
    // 持久化 challenge 用于后续 verify
    store.SaveChallenge(ch, ch.TimeLimit)  // Redis TTL
    return ch, nil
}
```

##### 3.3.3.6 验证算法

```go
// internal/captcha/moltcaptcha/verifier.go
type VerifyResult struct {
    AsciiOK     bool
    WordOK      bool
    CharPosOK   bool
    TotalCharOK bool
    SemanticOK  bool
    TimingOK    bool
    OverallPass bool
}

func VerifyResponse(challengeID string, responseText string) (*VerifyResult, error) {
    ch, err := store.GetChallenge(challengeID)
    if err != nil {
        return nil, ErrChallengeNotFound
    }
    
    result := &VerifyResult{}
    
    // 1. 时间窗
    result.TimingOK = time.Now().Unix() <= ch.ExpiresAt
    
    // 2. 拆分 lines/sentences
    lines := splitByFormat(ch.Format, responseText)
    if len(lines) != expectedLineCount(ch.Format) {
        return result, nil  // 直接返回失败
    }
    
    // 3. ASCII sum
    sum := 0
    for _, line := range lines {
        if len(line) == 0 {
            return result, nil
        }
        sum += int(line[0])
    }
    result.AsciiOK = sum == ch.AsciiTarget
    
    // 4. word count
    if ch.WordCount > 0 {
        result.WordOK = countWords(responseText) == ch.WordCount
    } else {
        result.WordOK = true
    }
    
    // 5. char position
    if ch.CharPosition != nil {
        line := lines[ch.CharPosition.Line-1]
        if ch.CharPosition.Position-1 >= len(line) {
            return result, nil
        }
        result.CharPosOK = line[ch.CharPosition.Position-1] == ch.CharPosition.Char
    } else {
        result.CharPosOK = true
    }
    
    // 6. total chars
    if ch.TotalChars > 0 {
        result.TotalCharOK = len(responseText) == ch.TotalChars
    } else {
        result.TotalCharOK = true
    }
    
    // 7. semantic（v2.0.1 简化：基于关键词命中 topic，避免引入额外 LLM 依赖）
    result.SemanticOK = containsAny(responseText, topicKeywords[ch.Topic])
    
    result.OverallPass = result.AsciiOK && result.WordOK && 
                         result.CharPosOK && result.TotalCharOK && 
                         result.SemanticOK && result.TimingOK
    return result, nil
}
```

##### 3.3.3.7 API 形态

```http
POST /api/v2/captcha/moltcaptcha/challenge
X-AAP-Proof: <jwt>
Content-Type: application/json

{ "difficulty": "medium" }

Response 200:
{ "challenge_id": "chg_...", "ascii_target": 295, "topic": "verification", ... }
```

```http
POST /api/v2/captcha/moltcaptcha/verify
X-AAP-Proof: <jwt>
Content-Type: application/json

{
  "challenge_id": "chg_...",
  "response": "algorithms churn the bits\ncollisions hide in shadows\nevery key finds place"
}

Response 200:
{ "overall_pass": true, "ascii_ok": true, ... }
```

##### 3.3.3.8 配置

```yaml
# config/captcha.yaml
captcha:
  moltcaptcha:
    enabled: true
    default_difficulty: medium
    storage_backend: redis  # challenge 存储
    challenge_ttl: "30s"    # 与 difficulty.time_limit 取小
    semantic_check: keyword_match  # v2.0.1: 仅关键词匹配
    bypass_for_trusted_owners: true  # owner_did 在白名单内可跳过
```

---

### 3.4 万亿级 UUID 设计

v2.0.1 沿用 v2.0 的 UUID v7 + v4 双模式，不做调整。

#### 3.4.1 UUID v7 优势

- **时间排序**：前 48 位为 Unix 时间戳，天然支持按时间范围查询
- **分布式安全**：后 74 位随机数，冲突概率低于 10^-18
- **万亿级支撑**：理论空间 2^122 ≈ 5.3 × 10^36，远超万亿需求
- **PostgreSQL 友好**：默认 B-tree 索引即可按时间有序插入，避免页分裂

#### 3.4.2 生成器实现

```go
// internal/uuid_generator/generator.go
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

### 3.5 5 层架构在 AgentID-Chain 的落地（v2.0.1 新增）

> 本节是 v2.0.1 与 `docs/architecture.md` 的对齐章节，明确每个能力在 L1-L5 中的归属。

#### 3.5.1 分层映射总览

```
┌────────────────────────────────────────────────────────────────────┐
│ L5-Gateway（接入网关）                                              │
│  · connect-go (gRPC/HTTP) 双协议监听                                │
│  · 中间件：Recover → RequestID → Metrics → Logging → CORS          │
│          → UA拦截 → AAP 协议握手 → Routing                          │
│  · 暴露：HTTP REST、MCP JSON-RPC、A2A endpoint、gRPC                │
├────────────────────────────────────────────────────────────────────┤
│ L3-Authz（权限决策，前置关卡）                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────┐ │
│  │ AAP 协议校验  │  │ MoltCaptcha  │  │ A2A Token    │  │ Rate   │ │
│  │              │  │ 引擎         │  │ 校验+撤销检查  │  │ Limit  │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────┘ │
│  · 失败一律 401/403/429，Fail Fast                                  │
│  · 不可用时降级：Redis 缓存最近结果（TTL 5min），过期返回 503         │
├────────────────────────────────────────────────────────────────────┤
│ L4-Service（业务编排）                                               │
│  · RegisterAgent / UpgradeAgent / BatchRegister / RevokeAgent      │
│  · 事务边界：ent.InTransaction，含 outbox_events 写入                │
│  · 工作流：升级审批链（AAP → L3 校验 → 业务规则 → 审计 → 通知）       │
│  · 插件接口（interfaces.go）：                                      │
│      ChainAdapter, CaptchaEngine, AuditNotifier, IdentityProvider   │
├────────────────────────────────────────────────────────────────────┤
│ L2-Domain（领域核心）                                                │
│  · 实体：Agent（UUID, Owner, Level, Permission, Status, Version）   │
│  · 值对象：AgentCredential, ChangeLog, AAPProof                     │
│  · 状态机（声明式）：                                                │
│      Created → Active ↔ Banned → Revoked                            │
│  · 领域事件：                                                       │
│      AgentRegisteredV1, AgentUpgradedV1, AgentBannedV1, ...         │
│  · 业务不变量：                                                     │
│      - Agent 不可自升级（operator_did ≠ owner_did）                  │
│      - 临时权限到期必须回退                                          │
│  · 纯 Go struct，零第三方依赖                                       │
├────────────────────────────────────────────────────────────────────┤
│ L1-Storage（数据持久）                                               │
│  · PostgreSQL（ent）：agents, audit_logs, outbox_events, aap_challenges│
│  · Redis：权限位缓存、AAP nonce 防重放、A2A Token 黑名单、限流计数   │
│  · Outbox 转发器：goroutine 轮询 outbox_events.pending → Redis Stream│
│  · 链适配器（插件）：FISCO/Polygon/BSC/mock（按需启用）               │
└────────────────────────────────────────────────────────────────────┘
```

#### 3.5.2 关键流程的分层穿越示例

**注册流程**（`POST /api/v2/agents/register`）：
```
L5: connect-go 收到 HTTP 请求
    ├─ 中间件解析 JWT（基础解密，不验签——L3 验）
    └─ 路由到 RegisterAgent
L3: Authz 校验
    ├─ 检查 X-AAP-Proof：验签 + 未过期
    ├─ 检查 MoltCaptcha 状态：首次注册需通过 Medium 难度
    └─ 检查 Rate Limit（按 IP + owner_did）
L4: RegisterAgent.Handle
    ├─ 参数校验（业务规则）
    ├─ 调用 domain.RegisterAgent
    └─ 事务内：ent 创建 + outbox 写入 + 审计
L2: domain.RegisterAgent
    ├─ 状态机：Created → Active
    ├─ UUIDv7 生成
    ├─ 业务不变量校验（owner_did 格式、level 范围）
    └─ 发布事件 AgentRegisteredV1
L1: Storage
    ├─ ent: INSERT agents + INSERT outbox_events + INSERT audit_logs
    ├─ Redis: SET agent:{uuid} (15min TTL)
    └─ 后台 Outbox 转发器：publish to Redis Stream
```

#### 3.5.3 依赖铁律（强化）

| 层级 | 允许依赖 | 禁止依赖 |
|---|---|---|
| L5 | L3（authz） | L4 以下直接调用 |
| L3 | L4 (通过 user_context 透传) | L2 / L1 |
| L4 | L2、L3、插件接口 | 插件具体实现 |
| L2 | L1（仅 ent 接口） | 任何第三方包（除标准库） |
| L1 | 标准库、ent、pgx、go-redis | L2 以上 |

#### 3.5.4 接口倒置示例

```go
// internal/service/interfaces.go (L4 定义)
package service

// ChainAdapter 是 L4 定义的链适配器接口，由 plugins/chain_adapter/ 实现
type ChainAdapter interface {
    RegisterAgent(ctx context.Context, uuid, ownerDID string, level uint8, perm uint64) (txHash string, err error)
    GetAgentInfo(ctx context.Context, uuid string) (info *AgentInfo, err error)
    UpdateLevel(ctx context.Context, uuid string, newLevel uint8) (txHash string, err error)
    Ban(ctx context.Context, uuid string) (txHash string, err error)
}

// CaptchaEngine 验证码引擎接口（可切换 MoltCaptcha / hCaptcha / 自研）
type CaptchaEngine interface {
    GenerateChallenge(difficulty string) (any, error)
    Verify(challengeID, response string) (pass bool, err error)
}
```

```go
// cmd/server/main.go（依赖注入）
func main() {
    cfg := config.MustLoad("config.yaml")
    
    storage := storage.New(cfg.Database, cfg.Redis)
    domainSvc := domain.New(storage)
    
    // 插件（按配置启用，禁用则用 noop）
    var chain service.ChainAdapter = noop.NoopChain{}
    if cfg.Backend.Type == "onchain" || cfg.Backend.Type == "hybrid" {
        chain = chainadapter.New(cfg.Backend.Onchain)
    }
    
    var captcha service.CaptchaEngine = moltcaptcha.New(storage.Redis())
    if !cfg.Captcha.MoltCaptcha.Enabled {
        captcha = noop.NoopCaptcha{}
    }
    
    authzSvc := authz.New(cfg.Authz, captcha, storage.Redis())
    svc := service.New(domainSvc, authzSvc, chain, captcha)
    gateway.New(svc, authzSvc, cfg.Server).Start()
}
```

#### 3.5.5 与 `architecture.md` 的差异点

| 维度 | architecture.md | v2.0.1 |
|---|---|---|
| 协议层 | 纯 connect-go | connect-go + MCP JSON-RPC + A2A endpoint |
| 存储 ORM | ent（pgx） | ent（pgx/v5）— 一致 |
| 缓存 | Redis | Redis — 一致，新增用途：AAP nonce、A2A 撤销列表 |
| 配置 | koanf | koanf — 一致 |
| 日志 | slog | slog — 一致 |
| 追踪 | OTel | OTel — 一致 |
| Outbox | Redis Stream | Redis Stream — 一致 |
| 状态机 | 声明式 | 声明式（YAML）+ Go 实现 — 兼容 |

v2.0.1 不与 `architecture.md` 冲突；只在协议层和存储用途上做扩展。

---

## 4. 四种接入范式

v2.0.1 沿用 v2.0 的四种接入方式，本节对 **A2A 协议（§4.3）** 做完整规范补全。

### 4.1 CLI 客户端

#### 4.1.1 安装方式

```bash
# 方式一：Go install
go install github.com/agentid-chain/cli/cmd/agentid@v2.0.1

# 方式二：Homebrew
brew tap agentid-chain/tap
brew install agentid

# 方式三：Docker（v2.0.1 推荐，详见 §7.5）
docker pull agentid-chain/cli:v2.0.1
alias agentid='docker run --rm -v ~/.agentid:/root/.agentid agentid-chain/cli:v2.0.1'
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

# 本地模式初始化（v2.0.1：强制 PostgreSQL）
agentid local init --dsn "postgres://agentid:pass@localhost:5432/agentid?sslmode=disable"
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
      "args": ["-y", "@agentid-chain/mcp-server@v2.0.1"],
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

### 4.3 A2A 协议（Agent-to-Agent）— **v2.0.1 补全 Token / 撤销 / TrustLevel**

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

#### 4.3.3 A2A Token 格式（v2.0.1 规范）

**Token 类型**：EdDSA（Ed25519）签名的 JWT

**Claims 结构**：
```json
{
  "iss": "agentid-a2a-gateway",      // 颁发方
  "aud": "did:agentid:0xABCD...",     // 接收 Agent
  "sub": "did:agentid:0x1234...",     // 发起 Agent
  "exp": 1718003600,                  // 过期时间（默认 1h）
  "iat": 1718000000,                  // 签发时间
  "jti": "a2a_tok_01HXY...",          // 唯一 token ID，用于撤销
  "scope": ["data.read", "batch.submit"],  // 协商授予的权限
  "trust_level": "full",              // 见 §4.3.5
  "audit_id": "audit_01HXY..."        // 关联审计日志
}
```

**Header**：
```json
{
  "alg": "EdDSA",
  "typ": "JWT",
  "kid": "agentid-gateway-2026"       // 网关密钥 ID
}
```

**签名输入**：`base64url(header) + "." + base64url(payload)`，使用网关 Ed25519 私钥签名。

#### 4.3.4 A2A 协商端点

```go
// internal/a2a/handler.go
package a2a

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type A2AHandler struct {
    backend core.IdentityBackend
    cache   *cache.RedisCache
    signer  ed25519.PrivateKey
}

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

    // 2. 校验 A2A 权限位（双方都需 PermA2AInterop）
    if agentA.Permission&core.PermA2AInterop == 0 {
        c.JSON(403, gin.H{"error": "agent A lacks A2A permission"})
        return
    }
    if agentB.Permission&core.PermA2AInterop == 0 {
        c.JSON(403, gin.H{"error": "agent B lacks A2A permission"})
        return
    }

    // 3. 计算 trust_level（详见 §4.3.5）
    trust := computeTrustLevel(agentA, agentB)

    // 4. 协商 scope：取双方 requested 与自身 permission 的交集
    scope := intersectScopes(agentA.Permission, agentB.Permission, req.RequestedPermissions)

    // 5. 生成 token（jti 唯一）
    jti := ulid.Make().String()
    token, err := h.signA2AToken(jti, agentA.Owner, agentB.Owner, scope, trust, req.Duration)
    if err != nil {
        c.JSON(500, gin.H{"error": "sign failed"})
        return
    }

    // 6. 记录 token 元数据（用于后续撤销与审计）
    h.cache.SetA2ATokenMeta(c, jti, A2ATokenMeta{
        AgentA: agentA.UUID,
        AgentB: agentB.UUID,
        Scope:  scope,
        IssuedAt: time.Now().Unix(),
    }, parseDuration(req.Duration))

    // 7. 审计
    auditID := h.writeAuditLog(agentA, agentB, scope, trust)

    c.JSON(200, A2ANegotiateResponse{
        Token:     token,
        JTI:       jti,
        ExpiresAt: time.Now().Add(parseDuration(req.Duration)).Unix(),
        TrustLevel: trust,
        AllowedPermissions: scope,
        AuditLogID: auditID,
    })
}
```

#### 4.3.5 TrustLevel 计算规则（v2.0.1 规范）

```go
// internal/a2a/trust.go
type TrustLevel string
const (
    TrustNone   TrustLevel = "none"     // 不互信
    TrustBasic  TrustLevel = "basic"    // 仅身份确认
    TrustFull   TrustLevel = "full"     // 完全互信
)

func computeTrustLevel(a, b *AgentInfo) TrustLevel {
    // 规则 1：任一方被封禁 → none
    if a.Status != 0 || b.Status != 0 {
        return TrustNone
    }
    
    // 规则 2：测试 Agent（level=1）参与 → 降级为 basic
    if a.Level == 1 || b.Level == 1 {
        return TrustBasic
    }
    
    // 规则 3：等级差 > 2 → basic（避免高级 Agent 与低级 Agent 完全互信）
    diff := int(a.Level) - int(b.Level)
    if diff < 0 {
        diff = -diff
    }
    if diff > 2 {
        return TrustBasic
    }
    
    // 规则 4：双方都有 PermA2AInterop 且 level >= 2 → full
    if a.Permission & core.PermA2AInterop != 0 && b.Permission & core.PermA2AInterop != 0 {
        return TrustFull
    }
    
    return TrustBasic
}
```

#### 4.3.6 Token 撤销机制（v2.0.1 新增）

**撤销方式**：服务端主动撤销 + Agent 主动撤销

**服务端撤销**（如 Agent 被封禁）：
```go
// 封禁时调用
func (h *A2AHandler) RevokeByAgent(uuid string) error {
    // 查询该 Agent 的所有 active token（通过 a2a:token:agent:{uuid} 索引）
    jtis := h.cache.GetActiveTokensByAgent(uuid)
    for _, jti := range jtis {
        // 加入黑名单（TTL = 剩余有效期，最多 1h）
        remaining := h.cache.GetTokenTTL(jti)
        h.cache.SetA2ARevoked(jti, uuid, remaining)
    }
    return nil
}
```

**Agent 主动撤销**：
```http
POST /a2a/revoke
X-API-Key: sk-...
Content-Type: application/json

{
  "jti": "a2a_tok_01HXY...",
  "reason": "collaboration ended"
}

Response 200:
{ "revoked": true, "effective_at": 1718000000 }
```

**Token 验证流程**（每次 A2A 通信前）：
```
1. 验签 JWT 签名（Ed25519，公钥从网关 jwks 端点获取）
2. 检查 exp 未过期
3. GET a2a:revoked:{jti}  → 若存在则拒绝
4. 检查 sub/aud 与请求上下文匹配
```

**Redis 键设计**：
- `a2a:token:{jti}` → token 元数据（agent_a, agent_b, scope, issued_at, ttl）
- `a2a:revoked:{jti}` → 撤销标记，TTL = 剩余有效期
- `a2a:agent:{uuid}:tokens` → Set，记录该 Agent 的所有 active jti（用于批量撤销）

#### 4.3.7 A2A 请求/响应格式

```json
// POST /a2a/negotiate
{
  "agent_a_uuid": "018f3a2b-4c5d-7e8f-9a0b-1c2d3e4f5a6b",
  "agent_a_credential": "eyJhbGciOiJFZERTQSIs...",
  "agent_b_uuid": "018f3a2b-4c5d-7e8f-9a0b-1c2d3e4f5a6c",
  "agent_b_credential": "eyJhbGciOiJFZERTQSIs...",
  "purpose": "data_pipeline_collaboration",
  "requested_permissions": ["data.read"],
  "duration": "1h"
}

// Response 200
{
  "token": "eyJhbGciOiJFZERTQSIs...",
  "jti": "a2a_tok_01HXY...",
  "expires_at": 1718003600,
  "trust_level": "full",
  "allowed_permissions": ["data.read"],
  "audit_log_id": "audit_01HXY..."
}
```

#### 4.3.8 A2A 端点总览

| 端点 | 方法 | 描述 | 鉴权 |
|---|---|---|---|
| `/a2a/negotiate` | POST | 发起 Agent 间互信协商 | API Key + AAP Proof |
| `/a2a/verify` | POST | 验证 A2A Token 有效性 | API Key |
| `/a2a/revoke` | POST | 撤销已建立的互信关系 | API Key |
| `/a2a/list` | GET | 查询 Agent 的所有 active token | API Key |
| `/.well-known/jwks.json` | GET | 网关 Ed25519 公钥集 | 公开 |

#### 4.3.9 A2A 配置

```yaml
# config/a2a.yaml
a2a:
  enabled: true
  token_ttl: "1h"                       # 默认 token 有效期
  max_ttl: "24h"                        # 协商时允许的最大 ttl
  signing_key_id: "agentid-gateway-2026"
  signing_private_key: "${AGENTID_A2A_PRIVATE_KEY}"  # Ed25519
  jwks_refresh_interval: "1h"           # 公钥集刷新周期
  max_negotiations_per_hour: 100
  allowed_permission_override: false
  audit_all_negotiations: true
  revocation:
    cache_backend: redis
    default_ttl: "1h"                   # 撤销标记最大保留时间
```

---

### 4.4 Prompt 接入（自然语言配置）

v2.0.1 沿用 v2.0 的 Prompt 接入方案，**不调整**。

#### 4.4.1 设计定位

为不具备开发能力的运营人员或终端用户提供自然语言配置入口。

#### 4.4.2 Prompt Parser 架构

```
User Prompt → Intent Classifier → Slot Filling → Config Generator → Validation → Execution
```

#### 4.4.3 支持的自然语言模板

| 意图 | 示例 Prompt | 生成的动作 |
|---|---|---|
| 注册 | "帮我注册一个测试 Agent，用于数据抓取" | `agentid_register` (level=1) |
| 升级 | "把 search-agent 升级到普通 Agent，因为 Q3 要用" | `agentid_upgrade` (target=2) |
| 查询 | "查一下 018f... 的状态和权限" | `agentid_get_info` |
| 批量 | "给团队批量注册 5 个普通 Agent，都做数据读取" | `agentid_batch_register` |
| 配置 | "把后端改成本地模式，不用上链" | 修改 `backend.yaml` |
| 审计 | "导出最近 30 天所有 Agent 的操作日志" | `agentid_audit_logs` |

---

## 5. 配置系统

### 5.1 全局配置目录结构

```
config/
├── backend.yaml          # 存储后端配置（链上/链下/混合），v2.0.1 强制 local 用 postgres
├── agent_level.yaml      # Agent 等级模板配置
├── captcha.yaml          # AAP + MoltCaptcha 配置（v2.0.1 合并）
├── a2a.yaml              # A2A 协议参数配置（v2.0.1 增 signing_key）
├── mcp.yaml              # MCP Server 配置
├── cli.yaml              # CLI 客户端默认配置
└── app.yaml              # 服务端口、限流、缓存配置

docker/                   # v2.0.1 新增
├── Dockerfile.gateway
├── Dockerfile.cli
├── Dockerfile.migration
├── Dockerfile.mock-chain
├── compose/              # 详见 §7.5
└── config/               # compose 共享配置模板
```

### 5.2 后端配置（backend.yaml，v2.0.1 调整）

```yaml
backend:
  type: local              # onchain | local | hybrid

  onchain:
    driver: fisco-bcos
    rpc_url: "http://127.0.0.1:8545"
    contract_address: "0x..."
    operator_private_key: "${AGENTID_PRIVATE_KEY}"  # v2.0.1 统一变量名
    gas_limit: 300000
    confirmation_blocks: 1

  local:
    # v2.0.1：删除 db_driver 字段，强制 postgres
    dsn: "${AGENTID_DB_DSN}"
    max_open: 25
    max_idle: 10
    max_lifetime: "5m"
    redis_addr: "localhost:6379"
    redis_db: 0

  hybrid:
    identity_onchain: true
    permission_local: true
    sync_interval: "5m"
    sync_batch_size: 100
```

### 5.3 A2A 配置（a2a.yaml，v2.0.1 调整）

```yaml
a2a:
  enabled: true
  token_ttl: "1h"
  max_ttl: "24h"
  signing_key_id: "agentid-gateway-2026"
  signing_private_key: "${AGENTID_A2A_PRIVATE_KEY}"   # v2.0.1 新增
  jwks_refresh_interval: "1h"
  max_negotiations_per_hour: 100
  allowed_permission_override: false
  audit_all_negotiations: true
  revocation:
    cache_backend: redis
    default_ttl: "1h"
```

### 5.4 MCP 配置（mcp.yaml）

```yaml
mcp:
  enabled: true
  server_name: "agentid-chain"
  version: "2.0.1"
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

### 5.5 Captcha 配置（captcha.yaml，v2.0.1 调整）

```yaml
captcha:
  aap:
    enabled: true
    challenge_ttl: "30s"
    proof_ttl: "5m"
    domain_private_key: "${AGENTID_DOMAIN_PRIVATE_KEY}"
    domain_public_key_jwk: "<jwk>"
    require_https: true
    rate_limit_per_ip: "10/min"

  moltcaptcha:
    enabled: true
    default_difficulty: medium
    storage_backend: redis
    challenge_ttl: "30s"
    semantic_check: keyword_match
    bypass_for_trusted_owners: true
```

---

## 6. API 参考

### 6.1 公共 API（所有接入方式共用）

#### 注册 Agent
```http
POST /api/v2/agents/register
Content-Type: application/json
X-API-Key: sk-...
X-AAP-Proof: eyJ...      # v2.0.1：强制携带

{
  "name": "data-pipeline-01",
  "owner_did": "did:agentid:0x1234...",
  "init_level": 2,
  "capabilities": ["data.read", "batch.submit"],
  "admission_proof": "aap_proof_..."
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
X-AAP-Proof: eyJ...

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
  "status": "pending_approval",
  "approval_id": "app_...",
  "tx_hash": null
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

### 6.2 AAP/MoltCaptcha 端点（v2.0.1 新增）

```http
POST /aap/challenge
POST /aap/verify
POST /api/v2/captcha/moltcaptcha/challenge
POST /api/v2/captcha/moltcaptcha/verify
```

### 6.3 MCP 工具接口

MCP Server 将上述 HTTP API 封装为标准 MCP Tools，参数与 HTTP API 保持一致，通过 JSON-RPC 2.0 协议通信。

### 6.4 A2A 协议接口

| 端点 | 方法 | 描述 |
|---|---|---|
| `/a2a/negotiate` | POST | 发起 Agent 间互信协商 |
| `/a2a/verify` | POST | 验证 A2A Token 有效性 |
| `/a2a/revoke` | POST | 撤销已建立的互信关系 |
| `/a2a/list` | GET | 查询 Agent 的所有 active token |
| `/.well-known/jwks.json` | GET | 网关公钥集 |

---

## 7. 快速开始

### 7.1 场景一：本地 PostgreSQL 部署（CLI 模式，v2.0.1 调整）

适用企业内部 Agent 管理，**PostgreSQL + Redis**，零区块链依赖，5 分钟启动。

```bash
# 1. 启动基础设施（v2.0.1 推荐 Docker，详见 §7.5）
docker compose -f docker/docker-compose.dev.yml up -d postgres redis

# 2. 初始化数据库
agentid local init --dsn "postgres://agentid:pass@localhost:5432/agentid?sslmode=disable"

# 3. 写入配置
mkdir -p ~/.agentid
cat > ~/.agentid/config.yaml <<EOF
backend:
  type: local
  local:
    dsn: "postgres://agentid:pass@localhost:5432/agentid?sslmode=disable"
    redis_addr: "localhost:6379"
EOF

# 4. 注册测试 Agent
agentid register --name "test-agent-01" --level 1

# 5. 查询
agentid info --uuid "<返回的UUID>"
```

### 7.2 场景二：MCP 接入 Cursor

```bash
# 1. 安装 MCP Server
npm install -g @agentid-chain/mcp-server@v2.0.1

# 2. 配置 Cursor
# 编辑 ~/.cursor/mcp.json，填入第 4.2 节配置

# 3. 重启 Cursor，在 Agent 对话中直接调用
# "帮我注册一个普通 Agent"
```

### 7.3 场景三：A2A 互认部署

```bash
# 1. 部署网关（Docker 推荐，详见 §7.5）
docker run -d   -p 8080:8080   -v ./config:/app/config   agentid-chain/gateway:v2.0.1

# 2. 配置 A2A 协议（必须配置 signing_key）
# 编辑 config/a2a.yaml

# 3. Agent A 与 Agent B 协商互信
# 使用 SDK 或 CLI 调用 /a2a/negotiate
```

### 7.4 场景四：混合模式（企业级）

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
    operator_private_key: "${AGENTID_PRIVATE_KEY}"
  local:
    dsn: "${AGENTID_DB_DSN}"
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

### 7.5 Docker 部署（v2.0.1 新增）

#### 7.5.1 镜像清单

所有镜像发布到 **Docker Hub**：`agentid-chain/`

| 镜像 | 用途 | 标签示例 | 基镜像 |
|---|---|---|---|
| `agentid-chain/gateway` | 主网关（HTTP/gRPC/MCP/A2A） | `v2.0.1`, `latest`, `sha-<7位sha>` | `gcr.io/distroless/static-debian12:nonroot` |
| `agentid-chain/cli` | CLI 工具 | `v2.0.1`, `latest` | `gcr.io/distroless/static-debian12:nonroot` |
| `agentid-chain/migration-tool` | DB Schema 迁移（ent migrate） | `v2.0.1`, `latest` | `gcr.io/distroless/static-debian12:nonroot` |
| `agentid-chain/mock-chain` | 模拟链 sidecar（hybrid 场景） | `v2.0.1`, `latest` | `gcr.io/distroless/static-debian12:nonroot` |

构建镜像：`golang:1.26-alpine`（多阶段构建的 builder 阶段）

#### 7.5.2 目录结构

```
docker/
├── Dockerfile.gateway           # 多阶段构建 gateway
├── Dockerfile.cli               # 多阶段构建 cli
├── Dockerfile.migration         # 迁移工具镜像
├── Dockerfile.mock-chain        # 模拟链 sidecar
├── compose/
│   ├── docker-compose.dev.yml        # dev: postgres+redis+gateway+cli
│   ├── docker-compose.local.yml      # local-only: 同 dev 简化，无链
│   └── docker-compose.hybrid.yml     # hybrid: + mock-chain sidecar
├── config/                       # compose 共享配置模板
│   ├── backend.local.yaml
│   ├── backend.hybrid.yaml
│   ├── captcha.yaml
│   ├── a2a.yaml
│   └── agent_level.yaml
├── mock-chain/                   # 模拟链服务实现
│   ├── main.go
│   └── README.md
└── README.md                     # 部署说明
```

#### 7.5.3 Dockerfile 模板（gateway 示例）

```dockerfile
# docker/Dockerfile.gateway
# 多阶段构建：编译 → 静态二进制 → distroless

FROM golang:1.26-alpine AS builder
WORKDIR /src

# 依赖层缓存
COPY go.mod go.sum ./
RUN go mod download

# 源码层
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/agentid ./cmd/agentid

# 运行时（distroless，无 shell，攻击面最小）
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/agentid /app/agentid
COPY docker/config /app/config
EXPOSE 8080 9090 6060
USER nonroot:nonroot
ENTRYPOINT ["/app/agentid"]
CMD ["server", "--config", "/app/config"]
```

#### 7.5.4 Compose 模板

##### 7.5.4.1 dev（`docker/compose/docker-compose.dev.yml`）

适用于开发调试：本地 PostgreSQL + Redis + Gateway，端口 8080/9090/6060。

```yaml
version: "3.9"
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: agentid
      POSTGRES_PASSWORD: agentid_dev
      POSTGRES_DB: agentid
    ports: ["5432:5432"]
    volumes:
      - pg_data:/var/lib/postgresql/data
      - ../../scripts/init-databases.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U agentid"]
      interval: 5s

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]

  gateway:
    image: agentid-chain/gateway:v2.0.1
    build:
      context: ../..
      dockerfile: docker/Dockerfile.gateway
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    environment:
      AGENTID_DB_DSN: "postgres://agentid:agentid_dev@postgres:5432/agentid?sslmode=disable"
      AGENTID_REDIS_ADDR: "redis:6379"
      AGENTID_API_KEY: "${AGENTID_API_KEY:-devkey}"
      AGENTID_LOG_LEVEL: "${AGENTID_LOG_LEVEL:-info}"
    ports:
      - "8080:8080"  # HTTP / gRPC
      - "9090:9090"  # Metrics
      - "6060:6060"  # Pprof
    volumes:
      - ./config:/app/config:ro
    healthcheck:
      test: ["CMD", "/app/agentid", "healthz"]
      interval: 10s
      retries: 5

  cli:
    image: agentid-chain/cli:v2.0.1
    build:
      context: ../..
      dockerfile: docker/Dockerfile.cli
    environment:
      AGENTID_ENDPOINT: "http://gateway:8080"
      AGENTID_API_KEY: "${AGENTID_API_KEY:-devkey}"
    profiles: ["with-cli"]   # 默认不启动
    entrypoint: ["/app/agentid"]

  migration:
    image: agentid-chain/migration-tool:v2.0.1
    build:
      context: ../..
      dockerfile: docker/Dockerfile.migration
    depends_on:
      postgres: { condition: service_healthy }
    environment:
      AGENTID_DB_DSN: "postgres://agentid:agentid_dev@postgres:5432/agentid?sslmode=disable"
    command: ["migrate", "up"]
    profiles: ["migration"]

volumes:
  pg_data:
  redis_data:
```

**启动**：
```bash
cd docker/compose
docker compose -f docker-compose.dev.yml up -d                   # 核心服务
docker compose -f docker-compose.dev.yml --profile migration run --rm migration  # 跑迁移
docker compose -f docker-compose.dev.yml --profile with-cli run --rm cli agentid info --uuid ...
docker compose -f docker-compose.dev.yml logs -f gateway
```

##### 7.5.4.2 local-only（`docker-compose.local.yml`）

简化版：仅 PostgreSQL + Redis + Gateway，**固定 `backend.type: local`，无链上后端**。适合企业内部首次部署。

与 dev 的差异：
- 移除 `cli` 的 profile 限制，cli 默认随 gateway 启动
- 强制环境变量 `AGENTID_BACKEND_TYPE=local`
- **不暴露** PostgreSQL/Redis 端口到宿主机（仅容器内访问）
- 端口收敛：`8080/9090/6060` 三个网关端口
- gateway 健康检查依赖 postgres+redis 必须 healthy

```yaml
# 关键差异示例
services:
  postgres:
    # 不暴露 ports 到宿主机
    expose: ["5432"]

  redis:
    expose: ["6379"]

  gateway:
    environment:
      AGENTID_BACKEND_TYPE: "local"
    ports:
      - "8080:8080"
      - "9090:9090"
      - "6060:6060"
```

##### 7.5.4.3 hybrid（`docker-compose.hybrid.yml`）

包含 **mock-chain sidecar**，模拟链上后端行为，用于：
- 本地无真实链节点时调试 OnchainBackend 路径
- 演示混合模式（核心身份上链 + 高频权限本地缓存）
- 集成测试

```yaml
version: "3.9"
services:
  postgres:    # 同 dev
  redis:       # 同 dev
  mock-chain:
    image: agentid-chain/mock-chain:v2.0.1
    build:
      context: ../mock-chain
    ports: ["8545:8545"]
    healthcheck:
      test: ["CMD", "/app/mock-chain", "healthz"]

  gateway:
    image: agentid-chain/gateway:v2.0.1
    depends_on:
      postgres:    { condition: service_healthy }
      redis:       { condition: service_healthy }
      mock-chain:  { condition: service_healthy }
    environment:
      AGENTID_BACKEND_TYPE: "hybrid"
      AGENTID_CHAIN_RPC_URL: "http://mock-chain:8545"
      AGENTID_CHAIN_DRIVER: "mock"
      AGENTID_PRIVATE_KEY: "${AGENTID_PRIVATE_KEY:-0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80}"
      AGENTID_DB_DSN: "postgres://agentid:agentid_dev@postgres:5432/agentid?sslmode=disable"
      AGENTID_REDIS_ADDR: "redis:6379"
    ports: ["8080:8080", "9090:9090", "6060:6060"]
```

**mock-chain 镜像**（`docker/mock-chain/`）说明：
- 监听 `:8545`，提供 JSON-RPC 风格接口
- 内存存储 Agent 注册/升级/封禁记录
- 启动时自动部署 mock 合约（go 实现的极简合约镜像）
- 重启即清空（**仅用于开发/测试，禁止用于生产**）
- 与真实链适配器使用相同接口（`core/chain_adapter/mock`）

#### 7.5.5 端口规划（与项目根 `CLAUDE.md` 尾号规则一致）

| 尾号 | 服务 | HTTP | Metrics | Pprof |
|---|---|---|---|---|
| **0** | API Gateway | `8080` | `9090` | `6060` |
| 1 | Auth Center（v2.0.1 预留扩展） | `8081` | `9091` | `6061` |
| 2 | Tag Sense（v2.0.1 预留扩展） | `8082` | `9092` | `6062` |
| — | PostgreSQL | `5432` | — | — |
| — | Redis | `6379` | — | — |
| — | Mock Chain | `8545` | — | — |

#### 7.5.6 镜像构建与发布

```bash
# 本地构建
docker build -f docker/Dockerfile.gateway      -t agentid-chain/gateway:dev        .
docker build -f docker/Dockerfile.cli          -t agentid-chain/cli:dev            .
docker build -f docker/Dockerfile.migration    -t agentid-chain/migration-tool:dev .
docker build -f docker/Dockerfile.mock-chain   -t agentid-chain/mock-chain:dev     .

# 发布（CI/CD 流程，tag 触发）
docker build -f docker/Dockerfile.gateway -t agentid-chain/gateway:v2.0.1 .
docker push agentid-chain/gateway:v2.0.1
docker push agentid-chain/gateway:latest
```

**标签约定**：
| 标签格式 | 含义 | 发布渠道 |
|---|---|---|
| `vX.Y.Z` | 语义化版本（与 Git tag 一致） | 稳定发布 |
| `latest` | 最新稳定版 | 稳定发布 |
| `sha-<7位短sha>` | 不可变追溯 | 每次 main 合并 |
| `dev` | 本地开发 | 不发布 |
| `rcN` | 预发布 | 候选版本 |

#### 7.5.7 健康检查端点

| 服务 | 端点 | 说明 |
|---|---|---|
| gateway | `http://localhost:8080/live` | 存活检查（进程在跑） |
| gateway | `http://localhost:8080/ready` | 就绪检查（PG + Redis 双健康） |
| gateway | `http://localhost:9090/metrics` | Prometheus 指标 |
| gateway | `http://localhost:6060/debug/pprof` | 性能分析 |
| postgres | `pg_isready -U agentid` | 容器内执行 |
| redis | `redis-cli ping` | 容器内执行 |
| mock-chain | `http://localhost:8545/healthz` | 模拟链健康 |

`/live` 与 `/ready` 的区别：
- `/live`：进程响应即返回 200，K8s/Docker 据此决定是否重启容器
- `/ready`：PG + Redis + 链上后端（如启用）全部健康才返回 200，反向代理据此决定是否转发流量

#### 7.5.8 升级与回滚

```bash
# 1. 拉取新版本
docker compose -f docker-compose.dev.yml pull gateway

# 2. 备份（升级前必做）
docker compose -f docker-compose.dev.yml exec postgres \
  pg_dump -U agentid agentid > backup_$(date +%Y%m%d_%H%M%S).sql

# 3. 跑迁移（schema 变更）
docker compose -f docker-compose.dev.yml --profile migration run --rm migration

# 4. 滚动升级
docker compose -f docker-compose.dev.yml up -d gateway

# 5. 验证
curl http://localhost:8080/ready
docker compose -f docker-compose.dev.yml logs -f gateway | grep -i error

# 6. 回滚（如升级失败）
docker compose -f docker-compose.dev.yml stop gateway
docker tag agentid-chain/gateway:v2.0.1 agentid-chain/gateway:rollback
docker pull agentid-chain/gateway:v2.0.0
docker compose -f docker-compose.dev.yml up -d gateway
```

#### 7.5.9 安全基线

- **基镜像**：distroless（无 shell、无包管理器，攻击面最小）
- **用户**：所有容器以 UID 65532（nonroot）运行
- **镜像签名**：CI 中使用 `cosign sign` 签名镜像，部署时验证
- **密钥管理**：生产环境所有 `AGENTID_*` 密钥从 Kubernetes Secret / Vault 注入，**禁止**写入 compose 文件
- **网络隔离**：compose 内置独立 network，PostgreSQL/Redis 不暴露宿主端口（local-only 默认）
- **只读挂载**：配置文件 `volumes: ["./config:/app/config:ro"]`
- **资源限制**：生产 compose 添加 `deploy.resources.limits`

---

## 8. 安全与风控

### 8.1 准入层安全

- **UA 黑名单**：拦截常见浏览器 UA、爬虫框架标识（Scrapy、curl、wget 等）
- **TLS 指纹校验**：拒绝非标准 TLS 指纹（人类浏览器指纹库 vs Agent SDK 指纹）
- **AAP 协议强制**：无 AAP Proof 的注册/敏感请求直接拒绝
- **MoltCaptcha 兜底**：通过 AAP 后仍需完成反向验证（首次注册或可疑行为触发）

### 8.2 权限安全

- **绝对禁止自升级**：Agent 无法自行修改自身等级或权限（operator_did ≠ owner_did）
- **多签保护**：高危操作（顶级权限、批量封禁）需 2~3 管理员多签
- **临时权限自动回收**：临时提权到期后自动回退（事务 + 定时任务）
- **权限缓存失效**：任何等级变更立即触发 Redis 缓存清空

### 8.3 A2A Token 安全（v2.0.1 新增）

- **Token 短 TTL**：默认 1h，最大 24h
- **签名算法**：Ed25519，私钥通过 `${AGENTID_A2A_PRIVATE_KEY}` 注入
- **撤销列表**：Redis 黑名单，TTL = 剩余有效期
- **JWKS 公开**：通过 `/.well-known/jwks.json` 暴露公钥，支持分布式验证
- **trust_level 计算**：基于双方 level 与 permission 自动降级，避免高权限 Agent 被低权限 Agent 利用

### 8.4 数据安全

- **传输加密**：TLS 1.2+ 强制（v2.0.1 `require_https: true`）
- **凭证签名**：链上 ECDSA，链下 HMAC-SHA256
- **环境变量隔离**：所有密钥通过 `${ENV_VAR}` 注入，禁止硬编码
- **审计完整性**：audit_logs 与业务事务同事务写入（v2.0.1 ent InTransaction）

### 8.5 审计与风控

- **全量操作日志**：注册/升级/封禁/注销/AAP/A2A 全量记录
- **异常行为检测**：QPS 突增、权限频繁申请、异地登录等触发风控告警
- **批量操作限制**：单管理员每小时批量操作上限

---

## 9. 部署模式总结

| 部署模式 | 存储后端 | 适用场景 | 区块链依赖 | 接入方式 |
|---|---|---|---|---|
| **本地测试** | LocalBackend (PostgreSQL) | 开发调试、单机构测试 | 无 | CLI / MCP / Prompt |
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
4. 提交 PR 至 `plugins/chain_adapter/` 目录

### 10.2 新增接入方式

1. 实现对应协议的服务端（如新增 gRPC 接入）
2. 在 L4-Service 定义插件接口（`internal/service/interfaces.go`）
3. 在 `config/` 中添加对应配置文件
4. 更新本文档第 4 节与第 6 节

### 10.3 新增 Agent 等级

1. 编辑 `config/agent_level.yaml` 添加新等级定义
2. 重启服务自动加载（链下）；链上模式需管理员操作合约

### 10.4 新增 CAPTCHA 实现（v2.0.1 新增）

1. 实现 `service.CaptchaEngine` 接口
2. 在 `config/captcha.yaml` 中配置启用
3. 在 `cmd/server/main.go` 中按配置注入

---

## 11. 附录

### 11.1 环境变量清单（v2.0.1 收敛）

| 变量名 | 说明 | 必填 | v2.0.1 变更 |
|---|---|---|---|
| `AGENTID_PRIVATE_KEY` | 链上操作私钥 | 链上/混合模式 | 重命名（原 `PRIVATE_KEY`） |
| `AGENTID_DB_DSN` | PostgreSQL 连接串 | 本地/混合模式 | 强制 postgres |
| `AGENTID_API_KEY` | API 调用密钥 | 是 | 不变 |
| `AGENTID_REDIS_ADDR` | Redis 地址 | 是 | 不变 |
| `AGENTID_DOMAIN_PRIVATE_KEY` | AAP 域签名私钥（Ed25519） | 是（生产） | v2.0.1 新增 |
| `AGENTID_A2A_PRIVATE_KEY` | A2A Token 签名私钥（Ed25519） | 是 | v2.0.1 新增 |
| `AGENTID_ENCRYPTION_KEY` | ~~SQLite 加密密钥~~ | — | **v2.0.1 删除**（统一 PostgreSQL） |

### 11.2 错误码表

| 错误码 | 说明 | 处理建议 | 所属分层 |
|---|---|---|---|
| `L1_001` | 数据库连接失败 | 检查 DSN / 网络 | L1 |
| `L1_002` | 唯一约束冲突（UUID 已存在） | 使用新 UUID 或查询已有身份 | L1 |
| `L1_003` | 外键约束违反 | 检查关联实体 | L1 |
| `L1_010` | Outbox 轮询失败 | 检查 Redis 连接 | L1 |
| `L2_201` | 状态机转换非法 | 检查当前状态与目标事件 | L2 |
| `L2_202` | 业务规则校验失败 | 检查参数合规性 | L2 |
| `L2_203` | 聚合根版本冲突（乐观锁） | 重试或检查并发 | L2 |
| `4001` | AAP 协议校验失败（超时/重放/域签名错/响应验签失败） | 检查 Agent SDK 与声明 | L3 |
| `4002` | MoltCaptcha 验证失败 | 重新完成反向验证 | L3 |
| `4003` | 人类/爬虫特征拦截 | 使用 Agent SDK 发起请求 | L3 |
| `4031` | 权限不足 | 申请升级或检查位掩码 | L3 |
| `4032` | Agent 已封禁 | 联系管理员申诉 | L3 |
| `4033` | A2A Token 已撤销 | 重新协商 | L3 |
| `4291` | Rate Limit 超限 | 降低请求频率 | L3 |
| `5001` | 链上交易失败 | 检查 Gas/Nonce/合约状态 | L1 |
| `5002` | 链下数据库错误 | 检查磁盘空间与连接 | L1 |
| `L5_801` | 请求参数解析失败 | 检查请求体格式 | L5 |

### 11.3 版本兼容性

| 版本 | 协议兼容 | 存储兼容 | 升级路径 |
|---|---|---|---|
| v1.0 → v2.0 | 不兼容（新增 AAP 协议） | 兼容（数据可迁移） | 导出 v1.0 数据 → v2.0 批量导入 |
| v2.0 → v2.0.1 | **兼容**（AAP/MoltCaptcha 协议增强，向后兼容） | **不兼容**（SQLite → PostgreSQL） | 导出 v2.0 数据 → 导入 PostgreSQL → 升级二进制 |
| v2.0.1.x → v2.0.1.y | 兼容 | 兼容 | 直接替换二进制 |

### 11.4 与 v2.0 的差异清单（变更日志）

| 变更 | 类别 | 章节 |
|---|---|---|
| 数据库统一 PostgreSQL | 收敛 | §1.2, §3.1.3, §3.1.4, §5.2, §11.1, §11.3 |
| 模块职责表加架构分层列 | 补缺口 | §2.2 |
| 新增 §3.5 5 层架构落地 | 补缺口 | §3.5 |
| AAP 协议补全（Challenge/Response/签名/防重放/TTL） | 补协议 | §3.3.2 |
| MoltCaptcha Go 实现规范 | 补设计 | §3.3.3 |
| A2A Token 格式（EdDSA JWT） | 补协议 | §4.3.3 |
| A2A 撤销机制（Redis 黑名单） | 补协议 | §4.3.6 |
| A2A TrustLevel 计算规则 | 补协议 | §4.3.5 |
| backend.yaml 强制 postgres | 收敛 | §3.1.4, §5.2 |
| A2A 增 signing_key 配置 | 补协议 | §5.3 |
| 环境变量统一前缀 `AGENTID_` | 收敛 | §11.1 |
| 错误码增加 L1-L5 分层标签 | 补缺口 | §11.2 |
| §7.5 Docker 部署（dev/local-only/hybrid 三套 compose + 4 个镜像） | **新增** | §7.5 |
| §3.3.3 MoltCaptcha Go 重写（保留与上游协议格式兼容） | **新增** | §3.3.3 |

---

> **文档版本**：v2.0.1
> **状态**：冻结（需求基线）
> **最后更新**：2026-06-08
> **维护者**：AgentID-Chain 开源社区
> **许可证**：Apache 2.0
