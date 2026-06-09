# 故障排查

> 常见问题的诊断与解决

## 🔍 通用流程

```
症状（错误率↑ / 延迟↑ / 服务下线）
   ↓
1. 看 Grafana 仪表板
   ↓
2. 定位异常指标（哪条曲线突变）
   ↓
3. 看 Prometheus 告警（最近 1h 触发列表）
   ↓
4. 拉取日志（按 trace_id 关联）
   ↓
5. 抓取 pprof（CPU/heap/goroutine）
   ↓
6. 应用 Runbook 或修补
```

## 🆘 常见问题

### Q: 服务无法启动 "address already in use"

**症状**：
```
bind: address already in use
```

**诊断**：
```bash
# 查找占用
lsof -i :8080
lsof -i :9090
lsof -i :6060

# 杀掉
lsof -ti :8080 | xargs kill -9
```

**根本解决**：
- 修改 `configs/app.yaml` 中端口
- 或停掉冲突进程

### Q: 数据库连接失败

**症状**：
```
dial tcp 127.0.0.1:5432: connect: connection refused
```

**诊断**：
```bash
# 1. 检查容器
docker ps | grep postgres

# 2. 检查端口
nc -zv localhost 5432

# 3. 测试连接
psql "$POSTGRES_DSN" -c "SELECT 1"
```

**修复**：
```bash
# 启动容器
docker-compose -f docker-compose.dev.yml up -d postgres

# 或重启
docker-compose -f docker-compose.dev.yml restart postgres
```

### Q: 错误率突增

**诊断**：
```bash
# 1. 看仪表板
open "http://grafana/d/agentid-chain-overview"

# 2. 按 status_code 分组
curl -s http://localhost:9090/metrics | grep http_requests_total | head -20

# 3. 拉错误日志
docker logs dev-api-gateway --tail=200 | grep -i error
```

**可能原因**：
- 依赖服务（PG/Redis/Chain）故障
- 配置错误
- 资源耗尽（CPU/内存/连接池）
- 代码 bug（最近发布引入）

详见 [Runbook: 高错误率](../runbooks/high-error-rate.md)

### Q: 延迟突增

**诊断**：
```bash
# 1. 看 P99
curl -s http://localhost:9090/metrics | grep http_request_duration_seconds_bucket | tail -20

# 2. 慢查询
psql "$POSTGRES_DSN" -c "SELECT pid, query, state, now() - query_start as duration FROM pg_stat_activity WHERE state != 'idle' ORDER BY duration DESC LIMIT 10"

# 3. pprof CPU
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

**可能原因**：
- 慢查询（缺索引 / 大表扫描）
- 连接池耗尽
- GC 暂停
- 链上 RPC 阻塞

### Q: AAP 验证失败激增

**诊断**：
```bash
# 1. 看 reason 分布
curl -s http://localhost:9090/metrics | grep aap_verify_total

# 2. 看是否重放
curl -s http://localhost:9090/metrics | grep aap_nonce_replays_total
```

**可能原因**：
- 客户端时钟漂移
- challenge 过期
- 私钥错误
- **重放攻击**（critical）

详见 [Runbook: AAP 重放攻击](../runbooks/aap-replay-attack.md)

### Q: 缓存命中率低

**诊断**：
```bash
# 命中率
curl -s http://localhost:9090/metrics | grep cache_hit_ratio

# 按 backend 分组
curl -s http://localhost:9090/metrics | grep cache_operations_total
```

**可能原因**：
- TTL 太短
- 缓存 key 不合理（变化太频繁）
- Redis 内存压力（eviction）
- 业务模式变化

### Q: 内存泄漏

**诊断**：
```bash
# 1. heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# 2. 在 pprof 中查看
(pprof) top  # 占用最大的对象
(pprof) list <function>  # 看具体代码

# 3. alloc_objects
go tool pprof http://localhost:6060/debug/pprof/heap?sample_index=alloc_objects
```

详见 [leak-detection.md](../perf/leak-detection.md)

### Q: Goroutine 泄漏

**诊断**：
```bash
# 1. goroutine profile
curl http://localhost:6060/debug/pprof/goroutine?debug=2 > goroutine.txt
grep -c "goroutine " goroutine.txt
# 数量 > 10000 异常

# 2. 看堆栈
head -200 goroutine.txt

# 3. 找阻塞点
grep "chan receive\|select\|sync.Mutex" goroutine.txt | head
```

## 🛠️ 工具速查

| 工具 | 用途 |
|------|------|
| `curl /healthz` | 服务健康 |
| `curl /metrics` | Prometheus 指标 |
| `curl /debug/pprof/` | pprof 入口 |
| `docker logs <svc>` | 容器日志 |
| `docker stats` | 资源占用 |
| `psql` | PG 客户端 |
| `redis-cli` | Redis 客户端 |
| `pg_stat_activity` | PG 活动查询 |
| `redis-cli INFO` | Redis 状态 |
| `tcpdump -i any port 8080` | 抓包 |

## 📊 关键指标速查

```bash
# QPS
curl -s http://localhost:9090/metrics | grep -E "^http_requests_total" | head

# P99 延迟（需计算）
curl -s http://localhost:9090/metrics | grep http_request_duration_seconds_bucket | head

# 错误数
curl -s http://localhost:9090/metrics | grep "status_code=\"5" | head

# 活跃 goroutine
curl -s http://localhost:9090/metrics | grep "^go_goroutines "

# 堆内存 (MB)
curl -s http://localhost:9090/metrics | grep "go_memstats_heap_inuse_bytes " | awk '{print $2/1024/1024}'
```

## 🔗 相关 Runbook

- [高错误率](../runbooks/high-error-rate.md)
- [数据库连接池耗尽](../runbooks/db-connection-pool.md)
- [链上 RPC 失败](../runbooks/chain-rpc-failure.md)
- [AAP 重放攻击](../runbooks/aap-replay-attack.md)
- [磁盘压力](../runbooks/disk-pressure.md)
