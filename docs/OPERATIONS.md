# Operations Guide

> AgentID-Chain 运维总入口

## 🚀 部署

### 本地开发

```bash
make bootstrap
docker-compose -f docker-compose.dev.yml up -d
go run ./cmd/agentid
```

详见 [local-dev.md](operations/local-dev.md)

### 生产部署

Helm chart：

```bash
helm install agentid-chain deploy/helm/agentid-chain \
  --namespace agentid --create-namespace \
  --values values-production.yaml
```

详见 [deployment.md](operations/deployment.md)

### GitOps

ArgoCD：

```bash
kubectl apply -f deploy/gitops/application.yaml
```

## 🔧 配置

详见 [configuration.md](operations/configuration.md)

## 📊 监控

- **Prometheus**：[metrics.md](operations/metrics.md)
- **Grafana dashboard**：[observability/grafana-dashboard.json](observability/grafana-dashboard.json)
- **告警规则**：[observability/prometheus-alerts.yaml](observability/prometheus-alerts.yaml)

## 🛠️ 故障响应

### Runbook

- [高错误率](runbooks/high-error-rate.md)
- [DB 连接池耗尽](runbooks/db-connection-pool.md)
- [链 RPC 失败](runbooks/chain-rpc-failure.md)
- [AAP 重放攻击](runbooks/aap-replay-attack.md)
- [磁盘压力](runbooks/disk-pressure.md)

### On-call

详见 [oncall.md](operations/oncall.md)

## 🔄 升级 / 回滚

```bash
# 升级
helm upgrade agentid-chain deploy/helm/agentid-chain

# 回滚
helm rollback agentid-chain
```

详见 [migration.md](operations/migration.md) + [release-process.md](operations/release-process.md)

## 📈 SLO 报表

每月初生成上月的 SLO 报告：

```bash
make slo-report MONTH=2026-05
```

详见 [slo-reporting.md](operations/slo-reporting.md)

## 🔐 安全运维

- 密钥轮换：[security-rotation.md](operations/security-rotation.md)
- 漏洞响应：[../docs/SECURITY.md](../docs/SECURITY.md)
- 审计日志：[audit-log.md](operations/audit-log.md)

## 🛠️ 常用命令

```bash
# 查看运行状态
kubectl get pods -n agentid
agentid status

# 查看日志
kubectl logs -n agentid -l app=agentid-chain --tail=100 -f

# 执行健康检查
curl http://agentid.local/healthz

# 查看 metrics
curl http://agentid.local:9090/metrics

# DB 迁移
agentid migrate up
agentid migrate down
agentid migrate status

# Chain 健康
agentid chain status
agentid chain sync
```

## 📚 延伸阅读

- [部署](operations/deployment.md)
- [本地开发](operations/local-dev.md)
- [配置](operations/configuration.md)
- [指标](operations/metrics.md)
- [故障排查](../docs/TROUBLESHOOTING.md)
- [Runbook 列表](runbooks/)
- [发布流程](operations/release-process.md)
- [变更管理](operations/change-management.md)
