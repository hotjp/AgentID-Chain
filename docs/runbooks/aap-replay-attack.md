# Runbook: AAP 重放攻击

## 严重度
**P0** — 5min 内 ack（安全事件）

## 触发告警
- `AAPNonceReplayDetected` — `aap_nonce_replays_total` 增量 > 0 (5min)
- `AAPVerifyFailureSpike` — 失败率 > 5/s (5min)

## 症状
- 监控告警 `aap_nonce_replays_total` 触发
- 短时间内大量 `verify` 失败，reason=replay
- 来自同一 IP 或同一 `public_key` 的高频请求

## 安全背景

AAP 使用一次性 challenge 防重放：
1. 客户端请求 challenge
2. 服务端存入 Redis：`SET aap:nonce:<pubkey> <challenge> NX EX 60`
3. 客户端签名 + verify
4. 服务端 `DEL` nonce，**保证一次性**

如果同一 challenge 被第二次使用，**必然是攻击**。

## 立即行动（5min 内）

1. **确认是攻击还是误用**
   ```bash
   # 哪类客户端在触发
   curl -s http://localhost:9090/metrics | grep "aap_nonce_replays_total"
   # 或
   psql "$POSTGRES_DSN" -c "SELECT client_ip, public_key, count(*), max(at) FROM aap_replay_log GROUP BY 1,2 ORDER BY 3 DESC LIMIT 20"
   ```

2. **拉取详细日志**
   ```bash
   docker logs dev-api-gateway --tail=1000 | grep "replay detected"
   ```

3. **评估是否需要封禁**
   - 单客户端少量重试：观察
   - 大量 IP 协同攻击：启用 IP 封禁
   - 同一 pubkey 大量重试：**撤销该 pubkey**

## 诊断

### 1. 攻击规模

```sql
-- 过去 1h
SELECT
  date_trunc('minute', at) AS minute,
  count(*) AS replay_count,
  count(DISTINCT client_ip) AS unique_ips,
  count(DISTINCT public_key) AS unique_keys
FROM aap_replay_log
WHERE at > now() - interval '1 hour'
GROUP BY 1
ORDER BY 1;
```

### 2. 攻击来源

```sql
-- Top IP
SELECT client_ip, count(*)
FROM aap_replay_log
WHERE at > now() - interval '1 hour'
GROUP BY 1
ORDER BY 2 DESC
LIMIT 20;
```

```sql
-- Top pubkey
SELECT public_key, count(*)
FROM aap_replay_log
WHERE at > now() - interval '1 hour'
GROUP BY 1
ORDER BY 2 DESC
LIMIT 20;
```

### 3. 攻击类型判断

| 模式 | 含义 | 应对 |
|------|------|------|
| 少量 IP + 少量 pubkey | 客户端 bug / 重试逻辑错误 | 通知客户端修复 |
| 大量 IP + 少量 pubkey | 针对特定 key 的攻击 | 撤销 pubkey |
| 少量 IP + 大量 pubkey | 探测攻击 | 封禁 IP |
| 大量 IP + 大量 pubkey | DDoS / 协调攻击 | 启用 WAF / 限流 |

## 缓解（短期）

### 1. IP 封禁

```bash
# iptables（Linux）
iptables -A INPUT -s <attacker_ip> -j DROP

# Cloudflare WAF（生产推荐）
# 在控制台 → Security → WAF → Tools → IP Access Rules
```

### 2. pubkey 撤销

```sql
-- 加入黑名单
INSERT INTO aap_revoked_keys (public_key, reason, revoked_at)
VALUES ('<pubkey>', 'replay attack', now())
ON CONFLICT DO NOTHING;
```

### 3. 临时限流

```yaml
ratelimit:
  per_ip:
    limit: 10      # 临时降低
    window: 1m
  per_agent:
    limit: 20      # 临时降低
    window: 1m
```

## 修复（根本）

### 1. 客户端重试逻辑检查

常见 bug：
```python
# ❌ 错误：用同一个 challenge 多次重试
def register_with_retry(owner, level, max_retries=3):
    challenge = get_challenge()
    for i in range(max_retries):
        try:
            return verify_and_register(challenge, ...)
        except NetworkError:
            continue  # ← 此时 challenge 已消耗！
```

正确：
```python
# ✅ 正确：每次重试重新获取 challenge
def register_with_retry(owner, level, max_retries=3):
    for i in range(max_retries):
        challenge = get_challenge()
        try:
            return verify_and_register(challenge, ...)
        except (NetworkError, AAPError) as e:
            if not is_retryable(e): raise
            time.sleep(2 ** i)
```

### 2. 服务端加固

- **challenge 绑定公钥**：`aap:nonce:<pubkey>:<challenge>` 而非 `aap:nonce:<challenge>`，防止跨 key 串用
- **verify 失败计数**：单 pubkey 失败 N 次后临时锁定
- **速率限制**：单 IP / 单 pubkey 的 verify 频率

### 3. 监控增强

```promql
# 告警阈值降低
sum(rate(aap_nonce_replays_total[1m])) > 0  # 任何一次都告警
```

```promql
# 趋势告警（持续）
sum(increase(aap_nonce_replays_total[10m])) > 10
```

## 验证

- [ ] 告警恢复
- [ ] 攻击来源 IP 已封禁
- [ ] 受害 pubkey 已撤销
- [ ] 客户端修复并部署
- [ ] 加强监控和告警

## 沟通

- 安全事件：立即通知 Security Team
- 影响用户：发公告
- **72h 内出安全 Post-Mortem**

## 📚 相关

- [api/aap.md](../api/aap.md)
- [SECURITY_AUDIT.md](../SECURITY_AUDIT.md)
- [operations/troubleshooting.md](../operations/troubleshooting.md)
