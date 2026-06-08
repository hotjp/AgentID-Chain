# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Project**: AgentID-Chain
**Description**: AI Agent 分布式身份与权限网关（v2.0.1：PostgreSQL + 混合存储 + 四种接入范式 CLI/MCP/A2A/Prompt）

## Architecture

5 层分层架构（依赖铁律 `L5-Gateway → L3-Authz → L4-Service → L2-Domain → L1-Storage`，与 `docs/architecture.md` 对齐）：

- **L5 Gateway**：connect-go 入口，UA 拦截 / AAP 协议握手 / 路由
- **L3 Authz**：AAP 校验 / MoltCaptcha / A2A Token / Rate Limit，前置关卡 Fail Fast
- **L4 Service**：Register/Upgrade/Batch 工作流，事务边界 + 插件接口
- **L2 Domain**：Agent 实体 / 状态机 / 领域事件，零第三方依赖
- **L1 Storage**：PostgreSQL（ent + pgx/v5）+ Redis（缓存/nonce/撤销/限流）+ Outbox

存储后端可配置：`local`（PG）/ `onchain`（FISCO/Polygon/BSC/mock）/ `hybrid`（混合）。完整需求基线见 `docs/AgentID-Chain-技术文档-v2.0.1.md`。

## Task Management

This project uses **LRA** for task tracking.
See [lra.md](lra.md) for command reference.

## Quick Start

```bash
lra ready              # Find available work
lra show <id>          # View task details
```

<!-- BEGIN LRA CLAUDE SECTION -->

## LRA Task Management

This project uses **LRA** profile: **full**

- Detailed guide: [lra.md](lra.md)
- Use `lra` for all task management
- Run `lra ready` before starting work
- ❌ Do not use markdown TODO lists

<!-- END LRA CLAUDE SECTION -->
