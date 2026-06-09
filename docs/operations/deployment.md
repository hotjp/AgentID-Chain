# 部署

> AgentID-Chain 的生产部署方案

## 🏗️ 部署模式

| 模式 | 适用 | 复杂度 |
|------|------|--------|
| **本地开发** | 个人 / 团队开发 | 1 |
| **Docker Compose** | 小规模 / 内部 | 2 |
| **Kubernetes** | 大规模 / 多区域 | 3 |

## 🐳 Docker Compose（推荐生产）

### 文件
`docker-compose.prod.yml`

### 启动

```bash
# 启动所有服务
docker-compose -f docker-compose.prod.yml up -d

# 查看状态
docker-compose -f docker-compose.prod.yml ps

# 查看日志
docker-compose -f docker-compose.prod.yml logs -f --tail=100 api-gateway
```

### 服务清单

| 服务 | 端口 | 副本数 | 资源 |
|------|------|--------|------|
| api-gateway | 8080/9090/6060 | 3 | 500m CPU, 512Mi RAM |
| auth-center | 8081/9091/6061 | 3 | 500m CPU, 512Mi RAM |
| tag-sense | 8082/9092/6062 | 2 | 300m CPU, 256Mi RAM |
| postgres | 5432 | 1 (主从) | 1000m CPU, 2Gi RAM |
| redis | 6379 | 1 (cluster) | 500m CPU, 1Gi RAM |

## ☸️ Kubernetes（大规模）

### Helm Chart

`deploy/helm/agentid-chain/`

```bash
helm install agentid-chain deploy/helm/agentid-chain/ \
  --namespace agentid \
  --create-namespace \
  --values values.prod.yaml
```

### values.prod.yaml 关键配置

```yaml
apiGateway:
  replicas: 3
  resources:
    requests: { cpu: 500m, memory: 512Mi }
    limits:   { cpu: 2000m, memory: 2Gi }
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
    targetCPUUtilization: 70

postgres:
  enabled: false  # 使用外部 RDS
  external:
    host: pg.cluster.local
    port: 5432
    database: agentid
    secretName: agentid-pg-credentials

redis:
  enabled: false  # 使用外部 ElastiCache
  external:
    host: redis.cluster.local
    port: 6379
    secretName: agentid-redis-credentials

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: api.agentid-chain.example.com
      paths:
        - path: /
          pathType: Prefix

monitoring:
  prometheus:
    enabled: true
    serviceMonitor: true
  grafana:
    enabled: true
    dashboardsConfigMap: agentid-dashboards
```

## 🔐 TLS / mTLS

### 单向 TLS（推荐对外）

```yaml
gateway:
  tls:
    enabled: true
    cert_file: /etc/tls/tls.crt
    key_file: /etc/tls/tls.key
    hsts:
      max_age: 31536000
      include_subdomains: true
      preload: true
```

### 双向 mTLS（服务网格内）

```yaml
gateway:
  tls:
    enabled: true
    client_ca_file: /etc/tls/ca.crt
    require_client_cert: true
```

## 🔧 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `AGENTID_ENV` | ✅ | `dev` / `staging` / `prod` |
| `AGENTID_CONFIG` | | 配置文件路径（默认 `configs/app.yaml`） |
| `AGENTID_LOG_LEVEL` | | `debug` / `info` / `warn` / `error` |
| `AGENTID_LOG_FORMAT` | | `json` / `text` |
| `POSTGRES_DSN` | ✅ | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `REDIS_ADDR` | ✅ | `host:6379` |
| `CHAIN_RPC` | (hybrid) | 链上 RPC URL |
| `CHAIN_PRIVATE_KEY` | (hybrid) | 链上钱包私钥（建议用 KMS） |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | | OTel Collector 地址 |
| `JWT_SIGNING_KEY` | ✅ | AAP JWT 签名密钥（建议 32B 随机） |

## 📊 部署后验证

```bash
# 健康检查
curl https://api.agentid-chain.example.com/live
# → 200 OK

# 注册测试 Agent
curl -X POST https://api.agentid-chain.example.com/v1/agents \
  -H "Authorization: Bearer <aap-jwt>" \
  -H "Content-Type: application/json" \
  -d '{"owner":"smoketest","level":"test"}'
# → 201 Created

# 查看指标
curl https://api.agentid-chain.example.com:9090/metrics | head
```

## 🔄 升级 / 回滚

### Docker Compose

```bash
# 升级
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d

# 回滚
docker-compose -f docker-compose.prod.yml down
git checkout v2.0.0
docker-compose -f docker-compose.prod.yml up -d
```

### Kubernetes

```bash
# 升级
helm upgrade agentid-chain deploy/helm/agentid-chain/ \
  --values values.prod.yaml \
  --version 2.0.2

# 回滚
helm rollback agentid-chain
```

## 🗃️ 数据库迁移

```bash
# 应用迁移
go run ./cmd/migration-tool up

# 回滚
go run ./cmd/migration-tool down --steps 1

# 状态
go run ./cmd/migration-tool status
```

## 🆘 故障

详见 [troubleshooting.md](troubleshooting.md) 和 [runbooks/](../runbooks/)

## 📚 相关

- [本地开发](local-dev.md)
- [配置参考](configuration.md)
- [故障排查](troubleshooting.md)
- [数据迁移](migration.md)
