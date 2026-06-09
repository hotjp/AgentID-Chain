# 配置参考

> 所有可配置项的完整说明

## 📁 配置文件层级

| 层 | 来源 | 优先级 | 用途 |
|----|------|--------|------|
| 1 | 命令行 flag | 最高 | 单次运行覆盖 |
| 2 | 环境变量 `AGENTID_*` | 高 | 容器/CI 注入 |
| 3 | 配置文件 `configs/app.yaml` | 中 | 标准配置 |
| 4 | 默认值 | 低 | 兜底 |

## 📄 完整配置 (`configs/app.yaml`)

```yaml
# =========================================================================
# 基础
# =========================================================================
env: prod  # dev | staging | prod
instance_id: ${HOSTNAME}  # 默认使用 hostname
shutdown_grace_period: 30s

# =========================================================================
# 日志
# =========================================================================
log:
  level: info  # debug | info | warn | error
  format: json  # json | text
  output: stdout  # stdout | stderr | file
  file_path: ""  # output=file 时使用
  add_source: false  # 生产建议 false
  sensitive:
    enabled: true
    extra_keys: []  # 额外脱敏字段
    preserve_length: false  # true: 保留长度

# =========================================================================
# 服务发现
# =========================================================================
services:
  api_gateway:
    addr: :8080
    metrics_addr: :9090
    pprof_addr: :6060
  auth_center:
    addr: :8081
    metrics_addr: :9091
    pprof_addr: :6061
  tag_sense:
    addr: :8082
    metrics_addr: :9092
    pprof_addr: :6062

# =========================================================================
# 存储
# =========================================================================
storage:
  backend: hybrid  # local | onchain | hybrid
  local:
    driver: postgres
    dsn: ${POSTGRES_DSN}
    max_open: 25
    max_idle: 10
    max_lifetime: 5m
    max_idle_time: 10m
  chain:
    type: polygon  # fisco | polygon | bsc | mock
    rpc: ${CHAIN_RPC}
    contract: 0x1234...
    private_key: ${CHAIN_PRIVATE_KEY}
    gas_limit: 200000
  hybrid:
    read_from: local
    write_to: local
    mirror_to: chain
    worker_interval: 5s
    worker_batch_size: 100
    max_retry: 3

# =========================================================================
# 缓存 / Redis
# =========================================================================
cache:
  redis:
    addr: ${REDIS_ADDR}
    password: ${REDIS_PASSWORD:-}
    db: 0
    pool_size: 20
    min_idle_conns: 10
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
  ttl:
    agent: 5m
    nonce: 60s
    aap_jwt: 1h
    a2a_token: 1h

# =========================================================================
# 鉴权
# =========================================================================
authz:
  jwt:
    algorithm: HS256  # HS256 | RS256
    signing_key: ${JWT_SIGNING_KEY}  # 至少 32B
    issuer: agentid-chain
    audience: agentid
  aap:
    challenge_ttl: 60s
    nonce_redis_key_prefix: "aap:nonce:"
    max_pending_nonces_per_pubkey: 5
  a2a:
    token_ttl: 1h
    revocation_redis_key: "a2a:revoked"
  rbac:
    default_level: test
    level_definitions:
      - name: test
        permissions: 0x000F  # 4 个低位
      - name: prod
        permissions: 0x00FF  # 8 个低位
      - name: internal
        permissions: 0xFFFF  # 全部

# =========================================================================
# MoltCaptcha
# =========================================================================
captcha:
  difficulty: 5
  ttl: 5m
  per_ip_per_minute: 10

# =========================================================================
# 限流
# =========================================================================
ratelimit:
  per_ip:
    limit: 60
    window: 1m
  per_agent:
    limit: 120
    window: 1m
  endpoints:
    - path: /v1/agents
      method: POST
      limit: 5
      window: 1m
  global:
    limit: 5000
    window: 1s

# =========================================================================
# TLS
# =========================================================================
tls:
  enabled: true
  cert_file: /etc/tls/tls.crt
  key_file: /etc/tls/tls.key
  hsts:
    max_age: 31536000
    include_subdomains: true
    preload: true
  trust_proxy: false  # 是否信任 X-Forwarded-Proto

# =========================================================================
# 安全响应头
# =========================================================================
security_headers:
  csp: "default-src 'self'; script-src 'self'; style-src 'self'"
  x_frame_options: DENY
  x_content_type_options: nosniff
  referrer_policy: no-referrer
  permissions_policy: "geolocation=(), microphone=()"
  coop: same-origin
  coep: require-corp
  corp: same-origin

# =========================================================================
# 观测
# =========================================================================
observability:
  otel:
    enabled: true
    endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT}
    service_name: agentid-gateway
    sample_ratio: 0.1
  metrics:
    enabled: true
    namespace: agentid
  logging:
    correlation: true
    inject_trace_id: true

# =========================================================================
# AAP
# =========================================================================
aap:
  algorithm: EdDSA  # EdDSA (Ed25519)
  challenge_size: 32  # bytes
  challenge_ttl: 60s
  token_ttl: 1h

# =========================================================================
# 链上
# =========================================================================
chain:
  type: polygon
  rpc: ${CHAIN_RPC}
  contract: ${CHAIN_CONTRACT}
  wallet:
    private_key: ${CHAIN_PRIVATE_KEY}
  gas:
    limit: 200000
    price_gwei: 30
  confirmations: 12

# =========================================================================
# 业务开关
# =========================================================================
feature_flags:
  enable_moltcaptcha: true
  enable_a2a: true
  enable_mcp: true
  enable_prompt: false  # 试验性
```

## 🔧 环境变量参考

| 变量 | 默认 | 必填 | 说明 |
|------|------|------|------|
| `AGENTID_ENV` | dev | | 运行环境 |
| `AGENTID_CONFIG` | configs/app.yaml | | 配置文件 |
| `AGENTID_LOG_LEVEL` | info | | 日志级别 |
| `AGENTID_LOG_FORMAT` | json | | json/text |
| `POSTGRES_DSN` | - | ✅ | PG 连接串 |
| `REDIS_ADDR` | - | ✅ | Redis 地址 |
| `REDIS_PASSWORD` | - | | Redis 密码 |
| `CHAIN_RPC` | - | (hybrid) | 链 RPC |
| `CHAIN_PRIVATE_KEY` | - | (hybrid) | 链钱包 |
| `CHAIN_CONTRACT` | - | (hybrid) | 合约地址 |
| `JWT_SIGNING_KEY` | - | ✅ | JWT 签名密钥 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | | OTel Collector |

## 🛡️ 敏感配置

以下配置**绝不能**提交到 Git：

- `JWT_SIGNING_KEY`
- `CHAIN_PRIVATE_KEY`
- `POSTGRES_DSN`（含密码）
- `REDIS_PASSWORD`

使用：
- 本地：`.env` 文件（`.gitignore`）
- 容器：K8s Secret / Docker Secret
- 编排：HashiCorp Vault / AWS Secrets Manager

## ✅ 配置验证

```bash
# 启动时自动验证
go run ./cmd/agentid serve --config configs/app.yaml
# → "config validated" 出现在日志

# 手动验证
go run ./cmd/agentid config validate
```
