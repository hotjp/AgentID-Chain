# task_001

## ⚠️ 重要提示（Agent 必读）

**当前位置**: `.long-run-agent/tasks/task_001.md`（任务描述文件）

**工作目录**: 项目根目录（`.long-run-agent` 的同级目录）

**产出物**: 请在项目根目录或适当子目录创建交付物

**这是配置文件**，不是最终产出！

## 描述

AgentID-Chain v2.0.1 全流程实施


## 需求 (requirements)

基于 docs/AgentID-Chain-技术文档-v2.0.1.md 与 docs/architecture.md，从零搭建到生产可用的 AI Agent 分布式身份与权限网关。覆盖大厂软件研发全流程：项目初始化→基础设施→5层架构实现→4种接入范式→测试70%+→CI/CD→安全合规→性能→可观测性→文档→Skill/Prompt→发布→运维。



## 验收标准 (acceptance)


- 所有子任务通过 LRA Constitution 验证（quality_first MANDATORY + deliverables_exist NON_NEGOTIABLE）

- 测试覆盖率 ≥ 70%（go test -cover）

- Docker 镜像发布到 Docker Hub 公开仓库

- 可观测性：trace/metrics/log 三件套接通

- Skills & Prompts 可被其他 Agent 直接调用




## 交付物 (deliverables)


- cmd/agentid/main.go

- internal/{gateway,authz,service,domain,storage}/

- core/backend/{interface,local,onchain}.go

- internal/captcha/{aap,moltcaptcha}/

- internal/a2a/handler.go

- mcp/server.go

- prompt/parser.go

- config/*.yaml

- docker/{Dockerfile.*,compose/*.yml}

- .github/workflows/*.yml

- skills/agentid/**/SKILL.md




## 设计方案 (design)

采用 5 层架构（L5-Gateway → L3-Authz → L4-Service → L2-Domain → L1-Storage）+ 混合存储后端。技术栈：connect-go + ent + pgx/v5 + go-redis + koanf + slog + OpenTelemetry。验收基于 LRA Ralph Loop 7 阶段。


## 验证证据（完成前必填）

<!-- 标记完成前，请提供以下证据： -->

- [ ] **实现证明**: 简要说明如何实现
- [ ] **测试验证**: 如何验证功能正常（测试步骤/截图/命令输出）
- [ ] **影响范围**: 是否影响其他功能

### 测试步骤
1. 
2. 
3. 

### 验证结果
<!-- 粘贴验证截图、命令输出或测试结果 -->