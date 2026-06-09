# Runbook: 高错误率

## 严重度
**P0** — 5min 内 ack

## 触发告警
- `HighErrorRate` — 5xx 比例 > 1% (5min)
- `VeryHighP99Latency` — P99 > 500ms (2min)

## 症状
- 用户请求失败率上升
- 监控告警触发
- 业务方反馈报错

## 立即行动（5min 内）

1. **确认范围**
   ```bash
   # 看是单个服务还是全栈
   curl -s http://localhost:9090/metrics | grep "^http_requests_total{" | head
   ```

2. **拉日志**
   ```bash
   docker logs dev-api-gateway --tail=500 | grep -i "error\|panic"
   ```

3. **触发回滚（如最近发布过）**
   ```bash
   # Helm
   helm rollback agentid-chain

   # Docker
   docker-compose -f docker-compose.prod.yml down
   git checkout <last-good-tag>
   docker-compose -f docker-compose.prod.yml up -d
   ```

## 诊断

### 1. 看仪表板
- 错误率曲线：哪个 route 异常？
- 错误率突增时间点：与发布时间匹配？
- P99 延迟：是否同时上升？

### 2. 看错误日志（按 status_code）

```bash
# 5xx
docker logs dev-api-gateway --tail=1000 | grep '"status":5'

# 按 trace_id 关联
TRACE_ID=abc123
docker logs dev-api-gateway --tail=1000 | grep $TRACE_ID
```

### 3. 常见 root cause

| 模式 | 原因 | 修复 |
|------|------|------|
| 所有 route 错误 | 依赖服务故障 | 检查 PG/Redis/Chain |
| 单 route 错误 | 代码 bug | 回滚 / 修补 |
| 错误率与发布同步 | 发布引入 | 立即回滚 |
| 内存持续上涨 | 内存泄漏 | 见 [leak-detection.md](../perf/leak-detection.md) |
| 连接池满 | DB 故障或慢查询 | 见 [db-connection-pool.md](db-connection-pool.md) |
| 链上超时 | RPC 节点故障 | 见 [chain-rpc-failure.md](chain-rpc-failure.md) |

### 4. 看 pprof

```bash
# CPU（耗时 30s）
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Heap
go tool pprof http://localhost:6060/debug/pprof/heap
```

## 缓解（短期）

1. **回滚到上一个稳定版本**
2. **关闭非关键路由**（如果有 feature flag）
3. **提高限流阈值**（减少下游压力）
4. **切换到备用依赖**（如备用 PG / 备用 RPC）

## 修复（根本）

1. 找到 root cause（参考诊断）
2. 修补代码 / 配置
3. 添加测试覆盖
4. 灰度发布（10% → 50% → 100%）
5. 观察 30min 确认稳定

## 验证

- [ ] 错误率 < 0.1%
- [ ] P99 延迟 < 100ms
- [ ] 告警已恢复（`HighErrorRate` 消失）
- [ ] 业务方确认恢复

## 沟通

- **首响**：Slack `#incident` 群
- **15min 升级**：Tech Lead
- **30min 升级**：VP Engineering
- **影响用户 > 1000**：发公告

## 事后

- 24h 内出 Post-Mortem
- 加入 [postmortems/](../postmortems/) 目录
- Action items 跟踪到完成
